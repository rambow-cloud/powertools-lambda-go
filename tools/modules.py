"""Maintain and verify independently consumable Go modules without publishing."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = json.loads((ROOT / "tools/modules.json").read_text(encoding="utf-8"))
BASE = MANIFEST["base"]
MODULES = MANIFEST["modules"]


def module_path(module):
    return BASE if module["directory"] == "." else BASE + "/" + module["directory"]


def run(*args, cwd=ROOT, env=None):
    result = subprocess.run(args, cwd=cwd, env=env, text=True, encoding="utf-8", capture_output=True)
    if result.returncode:
        raise RuntimeError(f"{cwd}: {' '.join(args)}\n{result.stdout}\n{result.stderr}")
    return result.stdout


def write_report(path, report):
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    temporary.replace(path)


def environment():
    env = os.environ.copy()
    env.update(CGO_ENABLED="0", GOWORK="off", GOMAXPROCS="2", GOFLAGS="-p=1", GOMEMLIMIT="128MiB", GOGC="20")
    env.pop("GOOS", None)
    env.pop("GOARCH", None)
    cache = ROOT / "dist/integration/go-cache"
    temporary = ROOT / "dist/local/go-tmp"
    cache.mkdir(parents=True, exist_ok=True)
    temporary.mkdir(parents=True, exist_ok=True)
    # Go ignores nested modules, but not .gitignore. Keep generated module caches
    # outside the root module's package walk, including pre-module dependencies.
    boundary = ROOT / "dist/go.mod"
    if not boundary.exists():
        boundary.write_text("module example.com/powertools-build-artifacts\n\ngo 1.26\n", encoding="utf-8")
    env.update(GOCACHE=str(cache), GOTMPDIR=str(temporary))
    return env


def publish_fixture(module, destination):
    """Write a local Go proxy artifact; this does not publish a repository or tag."""
    directory = ROOT / module["directory"]
    for filename in ("LICENSE", "NOTICE"):
        if not (directory / filename).is_file():
            raise RuntimeError(f"Missing {filename} in {directory}; run tools/licenses.py")
    name, version = module_path(module), module["version"]
    output = destination / name / "@v"
    output.mkdir(parents=True, exist_ok=True)
    output.joinpath(version + ".mod").write_bytes((directory / "go.mod").read_bytes())
    output.joinpath(version + ".info").write_text(json.dumps({"Version": version, "Time": "2026-09-14T00:00:00Z"}), encoding="utf-8")
    output.joinpath("list").write_text(version + "\n", encoding="utf-8")
    excluded = {".git", ".cache", ".upstream", ".uv-cache", "__pycache__", "dist", "node_modules", ".private", ".tmp", ".venv", ".codex", ".agents"}
    files_to_pack = []
    for parent, directories, files in os.walk(directory):
        directories[:] = sorted(d for d in directories if d not in excluded and not (Path(parent) / d / "go.mod").exists())
        for filename in sorted(files):
            if filename in {"go.work", "go.work.sum"} or filename.endswith(".exe"):
                continue
            files_to_pack.append(Path(parent) / filename)
    # Apply the repository ignore policy even to accidentally staged private files.
    checked = subprocess.run(
        ["git", "-c", "core.excludesFile=NUL", "check-ignore", "--no-index", "-z", "--stdin"],
        input="".join(file.relative_to(ROOT).as_posix() + "\0" for file in files_to_pack),
        cwd=ROOT, text=True, encoding="utf-8", capture_output=True,
    )
    if checked.returncode not in (0, 1):
        raise RuntimeError("Cannot evaluate publication ignore rules: " + checked.stderr)
    ignored = set(checked.stdout.split("\0"))
    archive_path = output / (version + ".zip")
    with zipfile.ZipFile(archive_path, "w", zipfile.ZIP_DEFLATED) as archive:
        for file in files_to_pack:
            if file.relative_to(ROOT).as_posix() in ignored:
                continue
            relative = file.relative_to(directory).as_posix()
            entry = zipfile.ZipInfo(name + "@" + version + "/" + relative, (2026, 9, 14, 0, 0, 0))
            entry.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(entry, file.read_bytes())
        license_file = ROOT / "LICENSE"
        if directory != ROOT and license_file.exists() and not (directory / "LICENSE").exists():
            archive.write(license_file, name + "@" + version + "/LICENSE")
    return hashlib.sha256(archive_path.read_bytes()).hexdigest()


def ordered_modules(env):
    """Sort current local versions; older requirements resolve through the proxy."""
    known = {module_path(m): m for m in MODULES}
    pending = {}
    for name, module in known.items():
        mod = json.loads(run("go", "mod", "edit", "-json", cwd=ROOT / module["directory"], env=env))
        if mod["Module"]["Path"] != name or mod.get("Replace"):
            raise RuntimeError(f"Unexpected module path or local replace in {name}")
        dependencies = set()
        for requirement in mod.get("Require") or []:
            dependency = requirement["Path"]
            if dependency == BASE or dependency.startswith(BASE + "/"):
                if dependency not in known:
                    raise RuntimeError(f"Unknown internal dependency: {requirement}")
                if module["public"] and not known[dependency]["public"]:
                    raise RuntimeError(f"Public module depends on development module: {dependency}")
                if requirement["Version"] == known[dependency]["version"]:
                    dependencies.add(dependency)
        pending[name] = dependencies
    ordered = []
    linux = {name for name, module in known.items() if module.get("test_os") == "linux"}
    while pending:
        ready = sorted(name for name, dependencies in pending.items() if not dependencies)
        if not ready:
            raise RuntimeError(f"Module dependency cycle: {pending}")
        for name in ready:
            if name in linux:
                known[name]["test_os"] = "linux"
            ordered.append(known[name])
            del pending[name]
        for name, dependencies in pending.items():
            if dependencies & linux:
                linux.add(name)
            dependencies.difference_update(ready)
    return ordered


def external_sums(path):
    if not path.exists():
        return ""
    return "".join(line + "\n" for line in path.read_text(encoding="utf-8").splitlines() if line.split()[0] != BASE and not line.startswith(BASE + "/"))


def check_isolation(module, dependencies):
    directory = module["directory"]
    if directory in {".", "eventhandler/http", "eventhandler/appsyncevents", "eventhandler/appsyncgraphql", "eventhandler/bedrock", "kafka", "datamasking"} and dependencies:
        raise RuntimeError("The core must remain free of external module dependencies")
    forbidden = []
    if directory in {".", "commons/metadata", "metrics", "logger", "eventhandler/http/metrics"}:
        forbidden += ["github.com/aws/aws-sdk-go", "github.com/aws/aws-xray-sdk-go", "go.opentelemetry.io/otel/sdk"]
    if directory in {"tracer", "integration", "eventhandler/http/tracer"}:
        forbidden += ["github.com/aws/aws-xray-sdk-go"]
    if directory in {"parameters", "signer", "idempotency", "idempotency/cache"}:
        forbidden += ["github.com/aws/aws-xray-sdk-go", "go.opentelemetry.io"]
    if directory in {"kafka/avro", "kafka/protobuf"}:
        forbidden += ["github.com/aws/", "go.opentelemetry.io", "github.com/redis/"]
    if directory == "kafka/avro":
        forbidden += ["google.golang.org/protobuf"]
    if directory == "kafka/protobuf":
        forbidden += ["github.com/hamba/avro", "github.com/linkedin/goavro"]
    if directory == "datamasking/kms":
        forbidden += ["github.com/aws/aws-xray-sdk-go", "go.opentelemetry.io", "github.com/redis/"]
    if directory == "commons/regex":
        forbidden += ["github.com/aws/", "go.opentelemetry.io", "github.com/redis/", "github.com/santhosh-tekuri/"]
    if directory == "jmespath":
        forbidden += ["github.com/aws/", "go.opentelemetry.io"]
    if directory == "parser":
        forbidden += ["github.com/aws/", "go.opentelemetry.io", "github.com/redis/", "github.com/jmespath-community/"]
    if directory == "logger":
        forbidden += ["github.com/jmespath-community/"]
    if directory == "batch":
        forbidden += ["github.com/aws/aws-sdk-go", "github.com/aws/aws-xray-sdk-go", "go.opentelemetry.io", "github.com/jmespath-community/"]
    if any(name.startswith(prefix) for name in dependencies for prefix in forbidden):
        raise RuntimeError(f"Optional dependency leaked into {directory}: {dependencies}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("list", "tidy", "check"))
    parser.add_argument("--only", action="append", default=[], metavar="DIRECTORY", help="Check selected module directories; save a separate scoped report")
    args = parser.parse_args()
    selected = set(args.only)
    if selected and (args.command != "check" or not selected <= {m["directory"] for m in MODULES}):
        parser.error("--only requires check and known module directories")
    if args.command == "list":
        for module in MODULES:
            prefix = "" if module["directory"] == "." else module["directory"] + "/"
            print(f"{module_path(module)} {prefix + module['version'] if module['public'] else '(development only)'} {module.get('status', '')}".rstrip())
        return
    if shutil.disk_usage(ROOT).free < 2 * 1024**3:
        raise RuntimeError("Module verification requires at least 2 GiB of free workspace disk space before starting. Review historical dist/modules caches before retrying; no files were automatically removed.")
    env = environment()
    modules = ordered_modules(env)
    workspace = json.loads(run("go", "work", "edit", "-json", str(ROOT / "go.work"), env=env))
    if {(ROOT / entry["DiskPath"]).resolve() for entry in workspace["Use"]} != {(ROOT / module["directory"]).resolve() for module in modules}:
        raise RuntimeError("go.work and tools/modules.json disagree")
    output = ROOT / "dist/modules"
    output.mkdir(parents=True, exist_ok=True)
    session = Path(tempfile.mkdtemp(prefix=args.command + "-", dir=output))
    original_cache = Path(run("go", "env", "GOMODCACHE", env=env).strip()) / "cache/download"
    proxy = session / "proxy"
    proxy.mkdir()
    # A fresh module cache prevents synthetic local versions from polluting the user's cache.
    # Reuse downloaded archives from earlier isolated runs before using the
    # network. The new fixture proxy remains first; checksum verification of
    # external modules remains enabled and the writable module cache is fresh.
    prior_caches = sorted((path for path in output.glob("*/cache/cache/download") if session not in path.parents), key=lambda path: path.stat().st_mtime, reverse=True)
    proxies = [proxy.as_uri(), original_cache.as_uri(), *(path.as_uri() for path in prior_caches), "https://proxy.golang.org"]
    env.update(GOMODCACHE=str(session / "cache"), GOPROXY=",".join(proxies), GONOSUMDB=",".join(filter(None, [env.get("GONOSUMDB"), BASE, BASE + "/*"])))
    report = {"cgo_enabled": False, "workspace": "off", "source": "local module proxy fixtures; not published versions", "completed": False, "modules": []}
    if selected:
        report["scope"] = "Selected modules only; this report does not replace full-workspace acceptance"
        report["selected_directories"] = sorted(selected)
    # Reuse pinned external hashes while still verifying downloaded module content.
    # Synthetic internal hashes are always resolved by each isolated consumer.
    pinned_sums = "\n".join(sorted({line for module in modules for line in external_sums(ROOT / module["directory"] / "go.sum").splitlines()})) + "\n"
    for module in modules:
        module_env = env.copy()
        if module.get("test_os") == "linux":
            module_env.update(GOOS="linux", GOARCH="amd64")
        test_executor = []
        if module.get("test_os") == "linux" and sys.platform != "linux":
            executor = f'"{sys.executable}" "{ROOT / "tools/run-linux-test.py"}"'
            test_executor = ["-exec", executor]
        name = module_path(module)
        directory = ROOT / module["directory"]
        if args.command == "tidy":
            print(f"{args.command}: {name}", flush=True)
            sums = directory / "go.sum"
            if sums.exists():
                sums.write_text(external_sums(sums), encoding="utf-8")
            run("go", "mod", "tidy", cwd=directory, env=env)
            if sums.exists():
                sums.write_text(external_sums(sums), encoding="utf-8")
            publish_fixture(module, proxy)
            continue
        digest = publish_fixture(module, proxy)
        if selected and module["directory"] not in selected:
            continue
        print(f"{args.command}: {name}", flush=True)
        # Test the extracted release-shaped archive, including its own test fixtures.
        consumer = session / "consumers" / module["directory"]
        consumer.mkdir(parents=True, exist_ok=True)
        consumer.joinpath("go.mod").write_text(f"module example.com/consumer\n\ngo 1.26\n\nrequire {name} {module['version']}\n", encoding="utf-8")
        consumer.joinpath("go.sum").write_text(pinned_sums, encoding="utf-8")
        downloaded = json.loads(run("go", "mod", "download", "-json", name, cwd=consumer, env=env))
        extracted = session / "sources" / module["directory"]
        # Keep the module cache immutable and run tests on an independently extracted copy.
        shutil.copytree(downloaded["Dir"], extracted, dirs_exist_ok=True)
        for parent, directories, files in os.walk(extracted):
            Path(parent).chmod(0o755)
            for filename in files:
                (Path(parent) / filename).chmod(0o644)
        before_mod = (extracted / "go.mod").read_bytes()
        before_sums = external_sums(extracted / "go.sum")
        run("go", "mod", "tidy", cwd=extracted, env=env)
        if (extracted / "go.mod").read_bytes() != before_mod or external_sums(extracted / "go.sum") != before_sums:
            raise RuntimeError(f"Dependency files need tidy: {name}")
        run("go", "test", "-mod=readonly", *test_executor, "./...", cwd=extracted, env=module_env)
        run("go", "vet", "-mod=readonly", "./...", cwd=extracted, env=module_env)
        item = {"path": name, "directory": module["directory"], "public": module["public"], "version": module["version"], "archive_sha256": digest, "tests": True, "vet": True, "tidy": True}
        if module.get("test_os") == "linux":
            item.update(binary_os="linux", binary_architecture="amd64", tests_runtime="docker-linux-amd64" if test_executor else "native-linux")
        if module["public"]:
            imported = name + ("/" + module["package"] if module.get("package") else "")
            consumer.joinpath("main.go").write_text(f'package main\n\nimport _ "{imported}"\n\nfunc main() {{}}\n', encoding="utf-8")
            run("go", "mod", "tidy", cwd=consumer, env=env)
            dependencies = run("go", "list", "-m", "-f", "{{.Path}}", "all", cwd=consumer, env=env).splitlines()[1:]
            external = [dependency for dependency in dependencies if dependency != BASE and not dependency.startswith(BASE + "/")]
            check_isolation(module, external)
            binary = consumer / ("consumer.exe" if os.name == "nt" and module.get("test_os") != "linux" else "consumer")
            run("go", "build", "-mod=readonly", "-o", str(binary), ".", cwd=consumer, env=module_env)
            item.update(consumer_build=True, external_modules=external, binary_bytes=binary.stat().st_size)
        else:
            dependencies = run("go", "list", "-m", "-f", "{{.Path}}", "all", cwd=extracted, env=env).splitlines()[1:]
            external = [dependency for dependency in dependencies if dependency != BASE and not dependency.startswith(BASE + "/")]
            check_isolation(module, external)
            item["external_modules"] = external
        report["modules"].append(item)
        write_report(session / "progress.json", report)
    if args.command == "check":
        report["completed"] = True
        report_path = ROOT / "docs/MODULE_ACCEPTANCE.json"
        if selected:
            report_path = ROOT / "docs/MODULE_ACCEPTANCE_SCOPED.json"
        write_report(session / "progress.json", report)
        write_report(report_path, report)
        verified = report["modules"]
        print(f"Verified {len(verified)} packaged modules and {sum(m['public'] for m in verified)} standalone consumers.")


if __name__ == "__main__":
    main()
