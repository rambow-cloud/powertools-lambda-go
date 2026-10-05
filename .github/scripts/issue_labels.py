"""Synchronize the label catalog and classify explicitly declared issue modules."""

import argparse
import json
import os
from pathlib import Path
import re
import sys
from urllib.parse import quote

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools"))
from release import GitHub, REPOSITORY

PROJECT = REPOSITORY.split("/")[-1]
CATEGORIES = {"bug": "bug", "feature": "enhancement", "docs": "documentation",
              "ci/cd": "cicd", "maintenance": "maintenance", "question": "question", "release": "release"}
LEGACY_FIELDS = ("Affected module or tool", "Affected module or area", "Affected page, module, or tool")


def module_label(directory):
    return "module:" + (PROJECT if directory == "." else directory)


def field(body, heading):
    body = re.sub(r"<!--.*?(?:-->|\Z)", "", body or "", flags=re.S)
    match = re.search(rf"(?im)^#{{2,3}} {re.escape(heading)}[ \t]*\r?$", body)
    if match is None:
        return None
    return re.split(r"(?m)^#{1,6} ", body[match.end():], maxsplit=1)[0].strip()


def desired_labels(issue, directories):
    """Return labels to add and whether an explicit selector owns module labels."""
    if "pull_request" in issue:
        return set(), False
    title = re.match(r"^\[([^]]+)\]:", issue.get("title", ""))
    category = CATEGORIES.get(title[1].casefold()) if title else None
    desired = {category} if category else set()
    if category == "release":
        return desired, False
    selection = field(issue.get("body"), "Affected modules")
    if selection is not None:
        aliases = {directory.casefold(): directory for directory in directories}
        aliases[PROJECT.casefold()] = "."
        for token in selection.split(","):
            name = token.strip().strip("`").casefold()
            if name in {"", "repository only", "_no response_"}:
                continue
            if name not in aliases:
                raise ValueError(f"Unknown affected module: {token.strip()}")
            directory = aliases.get(name)
            if directory is not None:
                desired.add(module_label(directory))
        return desired, True
    # Older forms have free text. Read only their declared area, never the narrative.
    for heading in LEGACY_FIELDS:
        text = field(issue.get("body"), heading)
        if text is None:
            continue
        for directory in sorted(directories, key=len, reverse=True):
            name = PROJECT if directory == "." else directory
            pattern = rf"(?<![\w/-]){re.escape(name)}(?![\w/-])"
            if re.search(pattern, text, flags=re.I) or (directory == "." and text.strip("` \r\n") == "."):
                desired.add(module_label(directory))
    return desired, False


def synchronize(api, definitions):
    existing = {label["name"].casefold(): label for label in api.pages("labels")}
    for label in definitions:
        current = existing.get(label["name"].casefold())
        if current is None:
            api.repo("labels", method="POST", data=label)
        elif any(current.get(key) != label[key] for key in ("name", "color", "description")):
            api.repo("labels/" + quote(current["name"], safe=""), method="PATCH", data=label)


def classify(api, issue, directories):
    if "pull_request" in issue:
        return
    desired, explicit = desired_labels(issue, directories)
    current = {label["name"] for label in issue.get("labels", [])}
    # POST is additive and preserves labels applied concurrently by maintainers.
    missing = desired - current
    if missing:
        api.repo(f"issues/{issue['number']}/labels", method="POST", data={"labels": sorted(missing)})
    managed = {module_label(directory) for directory in directories}
    if explicit:
        for stale in sorted((current & managed) - desired):
            api.repo(f"issues/{issue['number']}/labels/" + quote(stale, safe=""), method="DELETE")
    print(f"Issue #{issue['number']}: " + (", ".join(sorted(desired)) or "no explicit classification"), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    scope = parser.add_mutually_exclusive_group(required=True)
    scope.add_argument("--issue", type=int)
    scope.add_argument("--backfill", action="store_true")
    args = parser.parse_args()
    if args.issue is not None and args.issue < 1:
        parser.error("Use a positive issue number.")
    if os.environ.get("GITHUB_REPOSITORY", REPOSITORY) != REPOSITORY:
        raise ValueError("Unexpected repository.")
    definitions = json.loads((ROOT / ".github/labels.json").read_text(encoding="utf-8"))
    directories = [module["directory"] for module in json.loads((ROOT / "tools/modules.json").read_text(encoding="utf-8"))["modules"]]
    api = GitHub()
    synchronize(api, definitions)
    issues = api.pages("issues?state=all") if args.backfill else [api.repo(f"issues/{args.issue}")]
    for issue in issues:
        classify(api, issue, directories)


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, ValueError, KeyError, OSError, TypeError) as error:
        raise SystemExit(str(error)) from None
