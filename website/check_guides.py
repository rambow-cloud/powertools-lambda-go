"""Require usable examples, observable results and pinned-source feature maps."""

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[1]
GUIDES = (
    "LOGGER", "TRACER", "METRICS", "PARAMETERS", "BATCH", "IDEMPOTENCY",
    "JMESPATH", "PARSER", "JSON_SCHEMA_VALIDATION", "HTTP", "APPSYNC_EVENTS",
    "APPSYNC_GRAPHQL", "BEDROCK", "KAFKA", "DATAMASKING", "SIGNER", "METADATA",
    "COMMONS",
)
BASELINE = "7bcc27b1574493f9452688673658f52b80c53847"
SOURCE_PREFIX = "https://github.com/rambow-cloud/powertools-lambda-go/blob/main/"


def main() -> None:
    failures = []
    for name in GUIDES:
        path = ROOT / "docs" / f"{name}.md"
        content = path.read_text(encoding="utf-8")
        headings = set(re.findall(r"^## (.+)$", content, re.M))
        required = {"Objects and lifecycle", "TypeScript feature coverage"}
        required.add("Write your first log" if name == "LOGGER" else "Complete example")
        required.add("Messages, fields, and errors" if name == "LOGGER" else "Input and output")
        for heading in sorted(required - headings):
            failures.append(f"{path.name}: missing section {heading!r}")
        if BASELINE not in content:
            failures.append(f"{path.name}: missing pinned TypeScript source")
        if "| TypeScript feature | Go API or approach | Compatibility scope |" not in content:
            failures.append(f"{path.name}: missing feature comparison table")
        if not re.search(r"^~~~go\n(?:package main\b|--8<--)", content, re.M):
            failures.append(f"{path.name}: missing complete or maintained Go example")
        for target in re.findall(re.escape(SOURCE_PREFIX) + r"([^\s)#]+)", content):
            if not (ROOT / target).is_file():
                failures.append(f"{path.name}: missing referenced source {target}")
    if failures:
        raise SystemExit("\n".join(failures))
    print(f"Verified {len(GUIDES)} utility guides: examples, results, lifecycle and pinned feature/source coverage.")


if __name__ == "__main__":
    main()
