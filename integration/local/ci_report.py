"""Validate fresh runtime evidence and write the GitHub Actions job summary."""

import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def summarize(root, identity):
    acceptance = root / "dist/runtime-acceptance"
    suites = {
        "Lambda RIE and Valkey": (root / "dist/local/report.json", "passed"),
        "Streaming": (root / "dist/local/streaming/report.json", "passed"),
        "Batch logs and spans": (acceptance / "BATCH_ACCEPTANCE.json", "passed"),
        "Go to TypeScript KMS": (acceptance / "DATAMASKING_KMS_INTEROP.json", "completed"),
    }
    summary = {"execution": identity, "runtime_architecture": "amd64",
               "binary_architectures": ["amd64", "arm64"], "cgo_enabled": False,
               "module_checks": "reused from prerequisite Modules and Lambda artifacts job",
               "suites": {}, "binary_sha256": {}, "errors": []}
    errors = summary["errors"]
    for key in ("source_sha", "run_id", "run_attempt"):
        if not identity.get(key):
            errors.append(f"Missing execution identity: {key}")
    for name, (path, flag) in suites.items():
        try:
            report = json.loads(path.read_text(encoding="utf-8"))
            checks = report.get("checks", [])
            passed = (report.get(flag) is True and bool(checks)
                      and all(check.get("passed") is True for check in checks))
            if not passed:
                errors.append(f"{name}: missing or failed assertions")
            if report.get("execution") != identity:
                errors.append(f"{name}: evidence belongs to another commit or run attempt")
                passed = False
            if report.get("cleanup_errors"):
                errors.append(f"{name}: resource cleanup failed")
                passed = False
            if name in ("Lambda RIE and Valkey", "Streaming"):
                if (report.get("runtime_architecture") != "amd64"
                        or report.get("binary_architectures") != ["amd64", "arm64"]
                        or report.get("cgo_enabled") is not False):
                    errors.append(f"{name}: unexpected runtime or build configuration")
                    passed = False
            if name == "Lambda RIE and Valkey":
                if report.get("builds_executed") is not True or report.get("go_validation_executed") is not False:
                    errors.append(f"{name}: expected fresh builds and reused module checks")
                    passed = False
            if name == "Batch logs and spans":
                for filename in ("report.json", "otlp.json", "lambda.log"):
                    if report.get("sha256", {}).get(filename) != digest(root / "dist/local" / filename):
                        errors.append(f"{name}: source artifact changed: {filename}")
                        passed = False
            summary["suites"][name] = {"passed": passed, "assertions": len(checks)}
        except (OSError, ValueError, TypeError, AttributeError) as error:
            errors.append(f"{name}: {error}")
            summary["suites"][name] = {"passed": False, "assertions": 0}
    binaries = [f"{arch}/{name}" for arch in ("amd64", "arm64")
                for name in ("bootstrap", "stream-bootstrap")]
    binaries.extend(("amd64/capture", "amd64/kmsinterop"))
    for filename in binaries:
        try:
            summary["binary_sha256"][filename] = digest(root / "dist/local" / filename)
        except OSError as error:
            errors.append(f"Missing built binary {filename}: {error}")
    summary["passed"] = not errors
    return summary


def render(summary):
    identity = summary["execution"]
    lines = ["## Runtime simulation", "",
             f"Commit: `{identity.get('source_sha', 'missing')}`; run: "
             f"`{identity.get('run_id', 'missing')}`; attempt: `{identity.get('run_attempt', 'missing')}`.", "",
             "| Suite | Assertions | Result |", "| --- | ---: | --- |"]
    for name, suite in summary["suites"].items():
        lines.append(f"| {name} | {suite['assertions']} | {'Passed' if suite['passed'] else 'Failed'} |")
    lines.extend(("", "Module checks ran in the prerequisite job. Both architectures were rebuilt; "
                  "Docker executed amd64. CGO was disabled. Local synthetic endpoints only; no AWS resources.", ""))
    lines.extend(f"- {error}" for error in summary["errors"])
    return "\n".join(lines) + "\n"


def main():
    identity = {key: os.environ.get(name, "") for key, name in (
        ("source_sha", "GITHUB_SHA"), ("run_id", "GITHUB_RUN_ID"), ("run_attempt", "GITHUB_RUN_ATTEMPT"))}
    summary = summarize(ROOT, identity)
    head = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT, check=True,
                          capture_output=True, text=True).stdout.strip()
    if head != identity["source_sha"]:
        summary["errors"].append("Checked-out commit does not match GITHUB_SHA")
        summary["passed"] = False
    directory = ROOT / "dist/runtime-acceptance"
    directory.mkdir(parents=True, exist_ok=True)
    (directory / "summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    markdown = render(summary)
    (directory / "summary.md").write_text(markdown, encoding="utf-8")
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with Path(os.environ["GITHUB_STEP_SUMMARY"]).open("a", encoding="utf-8") as output:
            output.write(markdown)
    print(markdown)
    if not summary["passed"]:
        raise SystemExit("Runtime acceptance evidence is incomplete or failed")


if __name__ == "__main__":
    main()
