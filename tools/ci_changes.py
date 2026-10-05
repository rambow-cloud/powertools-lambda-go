"""Select CI suites from the full Git diff; unknown inputs receive every suite."""

import json
import os
from pathlib import Path
import re
import subprocess


SUITES = ("go", "docs", "automation", "release")
SHA = re.compile(r"[0-9a-f]{40}")


def classify(paths, force=False):
    selected = dict.fromkeys(SUITES, False)
    if force or not paths:
        return dict.fromkeys(SUITES, True)
    for path in paths:
        if path in {".github/workflows/ci.yml", "tools/ci_changes.py", "tools/test_ci_changes.py"}:
            return dict.fromkeys(SUITES, True)
        if path.startswith("releases/") or path == "tools/modules.json":
            return dict.fromkeys(SUITES, True)
        if path.startswith(".github/actions/") or Path(path).name in {"go.mod", "go.sum", "go.work", "go.work.sum"}:
            return dict.fromkeys(SUITES, True)
        if path.endswith(".go"):
            selected["go"] = selected["docs"] = True
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
        elif path.startswith(("integration/", "tools/reference/")) or path in {"tools/modules.py", "tools/licenses.py", "tools/test_runtime_ci.py"}:
            selected["go"] = True
        else:
            # Includes fixtures, licenses, build configuration and new file types.
            return dict.fromkeys(SUITES, True)
    return selected


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
    selected = classify(paths, force)
    with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
        for name, enabled in selected.items():
            output.write(f"{name}={str(enabled).lower()}\n")
    summary = "## Selected CI suites\n\n| Suite | Run |\n|---|---|\n"
    summary += "".join(f"| {name} | {str(enabled).lower()} |\n" for name, enabled in selected.items())
    summary += f"\nCompared {len(paths)} changed paths. Manual runs and unknown paths select all suites.\n"
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as output:
        output.write(summary)
    print(json.dumps({"selected": selected, "changed_paths": paths}, ensure_ascii=True))


if __name__ == "__main__":
    main()
