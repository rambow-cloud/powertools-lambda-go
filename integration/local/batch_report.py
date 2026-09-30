"""Verify Batch composition from existing Docker evidence without rerunning Lambda."""

import base64
import hashlib
import json
from pathlib import Path

root = Path(__file__).resolve().parents[2]
directory = root / "dist/local"
report = json.loads((directory / "report.json").read_text(encoding="utf-8"))
if not report["passed"] or "batch" not in report.get("utilities", []):
    raise SystemExit("Passing Batch runtime evidence is required")
snapshot = json.loads((directory / "otlp.json").read_text(encoding="utf-8"))
spans = [span for batch in snapshot["batches"] for resource in batch.get("resourceSpans", []) for scope in resource.get("scopeSpans", []) for span in scope.get("spans", [])]
logs = []
for line in (directory / "lambda.log").read_text(encoding="utf-8").splitlines():
    try:
        value = json.loads(line)
        if isinstance(value, dict):
            logs.append(value)
    except json.JSONDecodeError:
        pass
checks = []


def check(name, value):
    checks.append({"name": name, "passed": bool(value)})


for invocation in report["invocations"]:
    event, response = invocation["event"], invocation["response"]
    if "batch" not in response:
        continue
    identifier = event["id"]
    trace_id = response["trace_id"]
    records = [record for record in logs if record.get("test_id") == identifier and record.get("message") == "batch record"]
    check(f"{identifier}: eight invoked record logs", len(records) == 8)
    check(f"{identifier}: scoped Logger identity", len(records) == 8 and all(record.get("trace_id") == trace_id and record.get("correlation_id") == "query:" + identifier and record.get("function_request_id") == response["request_id"] for record in records))
    current = [span for span in spans if base64.b64decode(span["traceId"]).hex() == trace_id]
    children = [span for span in current if span["name"] == "### batch-record"]
    if event["mode"] == "unsampled":
        check(f"{identifier}: unsampled Batch exports no spans", not children)
        continue
    business = [span for span in current if span["name"] == "### business"]
    check(f"{identifier}: eight OTel record spans", len(children) == 8)
    check(f"{identifier}: business parent preserved", len(business) == 1 and len(children) == 8 and all(span.get("parentSpanId") == business[0]["spanId"] for span in children))
    check(f"{identifier}: two isolated record failures", sum(span.get("status", {}).get("code") in (2, "STATUS_CODE_ERROR") for span in children) == 2)

check("three successful invocation probes", sum("batch" in value["response"] for value in report["invocations"]) == 3)
summary = {"source": "existing local Docker artifacts; no additional invocations", "passed": all(item["passed"] for item in checks), "checks": checks, "sha256": {name: hashlib.sha256((directory / name).read_bytes()).hexdigest() for name in ("report.json", "otlp.json", "lambda.log")}}
(root / "docs/BATCH_ACCEPTANCE.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
failed = [item["name"] for item in checks if not item["passed"]]
print(f"Batch composition: {len(checks) - len(failed)}/{len(checks)} checks passed")
if failed:
    raise SystemExit("\n".join(failed))
