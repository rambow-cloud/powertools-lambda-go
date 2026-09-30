"""Preserve compact local acceptance evidence without repeating runtime checks."""

import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path

root = Path(__file__).resolve().parents[2]
directory = root / "dist" / "local"
report = json.loads((directory / "report.json").read_text(encoding="utf-8"))
if not report["passed"] or report["cleanup_errors"]:
    raise SystemExit("Only a completed, passing, cleaned-up run can be summarized")
summary = {
    "date": datetime.fromtimestamp((directory / "report.json").stat().st_mtime, timezone.utc).date().isoformat(),
    "runtime": "AWS Lambda Runtime Interface Emulator",
    "utilities": report.get("utilities", ["logger", "tracer", "metrics", "parameters", "commons", "metadata"]),
    "image": report["image"],
    "runtime_architecture": report["runtime_architecture"],
    "cgo_enabled": report["cgo_enabled"],
    "module_checks_executed_in_runtime_run": report["go_validation_executed"],
    "builds_executed": report.get("builds_executed", report["go_validation_executed"]),
    "invocation_count": len(report["invocations"]),
    "assertion_count": len(report["checks"]),
    "passed": report["passed"],
    "cleanup_errors": report["cleanup_errors"],
    "checks": report["checks"],
    "sha256": {},
}
for relative in ("report.json", "otlp.json", "lambda.log", "amd64/bootstrap", "arm64/bootstrap", "amd64/capture"):
    digest = hashlib.sha256()
    with (directory / relative).open("rb") as source:
        for chunk in iter(lambda: source.read(65536), b""):
            digest.update(chunk)
    summary["sha256"][relative] = digest.hexdigest()
if "cache_image" in report:
    summary["cache_image"] = report["cache_image"]
if "cache_interop" in report:
    summary["cache_interop"] = report["cache_interop"]
(root / "docs" / "LOCAL_ACCEPTANCE.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
print(f"Preserved {summary['assertion_count']} assertions in docs/LOCAL_ACCEPTANCE.json")
