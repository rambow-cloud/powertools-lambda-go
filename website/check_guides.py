"""Keep utility guides focused on code, visible results and application guidance."""

import json
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[1]
GUIDES = (
    "LOGGER", "TRACER", "METRICS", "PARAMETERS", "BATCH", "IDEMPOTENCY",
    "JMESPATH", "PARSER", "JSON_SCHEMA_VALIDATION", "HTTP", "APPSYNC_EVENTS",
    "APPSYNC_GRAPHQL", "BEDROCK", "KAFKA", "DATAMASKING", "SIGNER", "METADATA",
    "COMMONS", "HTTP_MIDDLEWARE", "HTTP_OBSERVABILITY", "HTTP_STREAMING",
    "IDEMPOTENCY_CACHE", "KAFKA_BINARY", "KAFKA_MODES", "DATAMASKING_KMS", "REGEX",
)
FENCE = re.compile(r"^[ \t]*(?P<mark>`{3}|~{3})(?P<language>\w*)[^\n]*\n(?P<code>.*?)\n[ \t]*(?P=mark)[ \t]*$", re.M | re.S)
SOURCE_PREFIX = "https://github.com/rambow-cloud/powertools-lambda-go/blob/main/"
LOGGER_GUIDES = {"KAFKA", "KAFKA_BINARY", "KAFKA_MODES", "DATAMASKING"}


def check_application_logging(code: str) -> list[str]:
    failures = []
    imports = re.findall(r'\bimport\s+(\([^)]*\)|(?:\w+\s+)?"[^"\n]+")', code, re.S)
    if any(re.search(r'"log(?:/slog)?"', block) for block in imports) or re.search(r'\b(?:stdlog|log)\.(?:Print|Fatal|Panic)(?:ln|f)?\(', code):
        failures.append("use Powertools Logger instead of the standard logging packages")
    print_names = {"fmt"}
    for block in imports:
        print_names.update(re.findall(r'\b(\w+)\s+"fmt"', block))
    for name in print_names:
        if re.search(r'\b' + re.escape(name) + r'\.(?:Print|Fprint)(?:ln|f)?\(', code):
            failures.append("use Powertools Logger for application output instead of fmt printing")
            break
    return failures


def check_logging_surface(root: Path = ROOT) -> list[str]:
    failures = []
    pages = list((root / "docs").rglob("*.md")) + [root / "README.md"]
    for path in pages:
        if not path.is_file():
            continue
        for number, block in enumerate(FENCE.finditer(path.read_text(encoding="utf-8")), 1):
            if block["language"] == "go":
                failures.extend(f"{path.relative_to(root)} Go block {number}: {failure}" for failure in check_application_logging(block["code"]))
    for path in (root / "examples").rglob("*.go"):
        if not path.name.endswith("_test.go"):
            failures.extend(f"{path.relative_to(root)}: {failure}" for failure in check_application_logging(path.read_text(encoding="utf-8")))
    return failures


def invalid_constant(value: str) -> None:
    raise ValueError(f"non-JSON number {value}")


def check_guide(content: str, name: str, root: Path = ROOT) -> list[str]:
    failures = []
    blocks = list(FENCE.finditer(content))
    if not any(block["language"] == "go" and block["code"].strip() for block in blocks):
        failures.append("missing Go example")
    if name in LOGGER_GUIDES:
        code = "\n".join(block["code"] for block in blocks if block["language"] == "go")
        if not all(token in code for token in ("logger.New(", "logger.Fields{", ".Info(")):
            failures.append("compose application logging examples with Powertools Logger")
        if re.search(r"\bfmt\.Print(?:ln|f)?\(|\bjson\.Marshal\(", code):
            failures.append("pass structured values to Logger instead of manually marshaling or printing application logs")
    if not re.search(r"^\*\*Input\*\*\s*$", content, re.M):
        failures.append("missing explicit example input")
    label = re.search(r"^\*\*Output\*\*\s*$", content, re.M)
    output = FENCE.match(content[label.end():].lstrip()) if label else None
    if not output or output["language"] == "go" or not output["code"].strip():
        failures.append("missing fenced output; prose alone is not a demonstration")
    for block in blocks:
        if block["language"] == "json":
            try:
                json.loads(block["code"], parse_constant=invalid_constant)
            except ValueError as exc:
                failures.append(f"invalid JSON example: {exc}")
    for heading in ("Things to know", "Next steps"):
        if not re.search(r"^## " + heading + r"$", content, re.M):
            failures.append(f"missing {heading!r} section")
    if re.search(r"TypeScript\s+v?\d+\.\d+\.\d+|@aws-lambda-powertools/[^\s@]+@\d+", content):
        failures.append("move pinned upstream versions to the contributor reference")
    if re.search(r"^## (?:TypeScript feature coverage|Finite .+ audit|Evidence and limits|Verification scope)$", content, re.M):
        failures.append("move implementation audits to reference pages")
    if f"reference/{name}.md" not in content or not (root / "docs/reference" / f"{name}.md").is_file():
        failures.append("missing linked API and behavior reference")
    for target in re.findall(re.escape(SOURCE_PREFIX) + r"([^\s)#]+)", content):
        if not (root / target).is_file():
            failures.append(f"missing referenced source {target}")
    for target in re.findall(r'--8<-- "([^"\n]+)"', content):
        if not (root / target.split(":", 1)[0]).is_file():
            failures.append(f"missing included example {target}")
    return failures


def main() -> None:
    failures = check_logging_surface()
    for name in GUIDES:
        content = (ROOT / "docs" / f"{name}.md").read_text(encoding="utf-8")
        failures.extend(f"{name}.md: {failure}" for failure in check_guide(content, name))
    if failures:
        raise SystemExit("\n".join(failures))
    print(f"Verified {len(GUIDES)} utility guides and application logging across all documentation and runnable examples.")


if __name__ == "__main__":
    main()
