"""Select affected modules from the full Git diff; unknown inputs receive full CI."""

import json
import os
from pathlib import Path
import re
import subprocess


SUITES = ("go", "docs", "automation", "release")
SHA = re.compile(r"[0-9a-f]{40}")
ROOT = Path(__file__).resolve().parents[1]


def module_graph(root):
    """Match the current-version dependency graph used by packaged checks, without Go."""
    manifest = json.loads((root / "tools/modules.json").read_text(encoding="utf-8"))
    modules = {module["directory"]: module for module in manifest["modules"]}
    names = {manifest["base"] + ("/" + directory if directory != "." else ""): directory for directory in modules}
    reverse = {directory: set() for directory in modules}
    for directory in modules:
        block = False
        declared = None
        for raw in (root / directory / "go.mod").read_text(encoding="utf-8").splitlines():
            line = raw.split("//", 1)[0].strip()
            if line.startswith("module "):
                declared = line.removeprefix("module ").strip()
            if line == "require (":
                block = True
                continue
            if block and line == ")":
                block = False
                continue
            requirement = line if block else line.removeprefix("require ") if line.startswith("require ") else ""
            if not requirement:
                continue
            parts = requirement.split()
            if len(parts) != 2:
                raise ValueError(f"Unsupported require syntax in {directory}/go.mod")
            name, version = parts
            if name in names and modules[names[name]]["version"] == version:
                reverse[names[name]].add(directory)
        expected = manifest["base"] + ("/" + directory if directory != "." else "")
        if declared != expected or block:
            raise ValueError(f"Invalid module declaration or require block in {directory}/go.mod")
    return modules, reverse


def owner(path, modules):
    return max((directory for directory in modules if directory == "." or path.startswith(directory + "/")), key=len)


def select(paths, force=False, root=ROOT):
    modules, reverse = module_graph(root)
    selected = dict.fromkeys(SUITES, False)
    changed, propagated = set(), set()
    runtime = dynamodb = False

    def full():
        return {**dict.fromkeys(SUITES, True), "modules": sorted(modules), "runtime": True, "dynamodb": True, "full": True}

    if force or not paths:
        return full()
    for path in paths:
        if path in {".github/workflows/ci.yml", "tools/ci_changes.py", "tools/test_ci_changes.py", "tools/modules.py", "tools/licenses.py", "go.mod", "go.work", "go.work.sum"}:
            return full()
        if path.startswith(("releases/", ".github/actions/", "tools/package/")) or path == "tools/modules.json":
            return full()
        directory = owner(path, modules)
        if Path(path).name in {"go.mod", "go.sum"}:
            if path != ("" if directory == "." else directory + "/") + Path(path).name:
                return full()  # A new/unregistered module must not inherit its parent's scope.
            changed.add(directory)
            propagated.add(directory)
            if directory == "integration":
                dynamodb = True
        elif path.endswith(".go") or ("/testdata/" in path and not path.endswith(".md")):
            changed.add(directory)
            if not path.endswith("_test.go") and "/testdata/" not in path:
                propagated.add(directory)
            selected["docs"] = True
        elif path in {"website/pyproject.toml", "website/uv.lock"}:
            selected["docs"] = selected["automation"] = selected["release"] = True
        elif path.startswith(".github/"):
            selected["automation"] = True
            if path == ".github/workflows/docs.yml":
                selected["docs"] = True
            elif path.startswith(".github/workflows/") and path != ".github/workflows/contribution.yml":
                selected["release"] = True
        elif path.startswith(("tools/release", "tools/test_release")) or path.startswith(".goreleaser"):
            selected["automation"] = selected["release"] = True
        elif path == "tools/test_contribution_workflow.py":
            selected["automation"] = True
        elif path.startswith(("docs/", "website/")) or path.endswith(".md") or path == "mkdocs.yml":
            selected["docs"] = True
        elif path in {"integration/local/dynamodb_run.py", "integration/local/test_dynamodb_run.py"}:
            changed.add("integration")
            dynamodb = True
        elif path.startswith(("integration/", "tools/reference/")) or path == "tools/test_runtime_ci.py":
            changed.add("integration")
            runtime = runtime or not path.startswith("integration/dynamodblocal/")
        else:
            return full()
        if path.startswith("integration/dynamodblocal/") and not path.endswith(".md"):
            dynamodb = True
        elif path.startswith("integration/") and not path.endswith(".md") and path not in {"integration/local/dynamodb_run.py", "integration/local/test_dynamodb_run.py"}:
            runtime = True
    pending = list(propagated)
    visited = set()
    while pending:
        directory = pending.pop()
        if directory in visited:
            continue
        visited.add(directory)
        changed.add(directory)
        pending.extend(reverse[directory] - visited)
    dynamodb = dynamodb or bool(changed & {"parameters", "idempotency", "commons/dynamodb"})
    selected["go"] = bool(changed)
    return {**selected, "modules": sorted(changed), "runtime": runtime, "dynamodb": dynamodb, "full": False}


def classify(paths, force=False):
    selection = select(paths, force)
    return {name: selection[name] for name in SUITES}


def changed_paths(event, event_name):
    if event_name == "workflow_dispatch":
        return [], True
    if event_name == "pull_request":
        base = event["pull_request"]["base"]["sha"]
        head = event["pull_request"]["head"]["sha"]
        separator = "..."
    elif event_name == "push":
        base, head, separator = event["before"], event["after"], ".."
    else:
        raise ValueError(f"Unsupported CI event: {event_name}")
    if not all(SHA.fullmatch(value) for value in (base, head)):
        raise ValueError("Expected full Git commit SHAs")
    if base == "0" * 40:
        return [], True
    # --no-renames includes both old and new paths; NUL delimiters handle any name.
    result = subprocess.run(
        ["git", "diff", "--name-only", "--no-renames", "-z", base + separator + head, "--"],
        check=True, capture_output=True,
    )
    return [os.fsdecode(path) for path in result.stdout.split(b"\0") if path], False


def main():
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text(encoding="utf-8"))
    paths, force = changed_paths(event, os.environ["GITHUB_EVENT_NAME"])
    selected = select(paths, force)
    with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
        for name, enabled in selected.items():
            value = json.dumps(enabled, separators=(",", ":")) if name == "modules" else str(enabled).lower()
            output.write(f"{name}={value}\n")
    summary = "## Selected CI suites\n\n| Suite | Run |\n|---|---|\n"
    summary += "".join(f"| {name} | {str(enabled).lower()} |\n" for name, enabled in selected.items() if name != "modules")
    summary += "\nSelected modules: " + (", ".join(selected["modules"]) or "none") + "\n"
    summary += f"\nCompared {len(paths)} changed paths. Release plans, manual runs and shared/unknown paths select full regression.\n"
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as output:
        output.write(summary)
    print(json.dumps({"selected": selected, "changed_paths": paths}, ensure_ascii=True))


if __name__ == "__main__":
    main()
