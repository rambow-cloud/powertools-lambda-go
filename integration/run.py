"""Reproducible, disposable AWS Lambda acceptance tests using Hong Kong only."""
from __future__ import annotations

import base64
import concurrent.futures
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import time
import urllib.request
import zipfile

# Test fixtures must never inherit a CGO-enabled build from the caller.
os.environ["CGO_ENABLED"] = "0"

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "dist" / "integration"
STATE = OUT / "state.json"
PROFILE = os.environ.get("AWS_PROFILE", "")
REGION = "ap-east-1"
ACCOUNT = os.environ.get("POWERTOOLS_TEST_ACCOUNT", "")
COLLECTOR = "0.23.0"


def save(path, value):
    Path(path).write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def aws(*args, raw=False):
    require_configuration()
    command = ["aws", "--profile", PROFILE, "--region", REGION, "--no-cli-pager", *map(str, args)]
    if not raw:
        command += ["--output", "json"]
    result = subprocess.run(command, capture_output=True, text=True, encoding="utf-8", env={**os.environ, "AWS_PAGER": ""})
    if result.returncode:
        raise RuntimeError(f"AWS {args[0]} {args[1]} failed: {result.stderr.strip()}")
    return result.stdout.strip() if raw else json.loads(result.stdout or "{}")


def identity():
    found = aws("sts", "get-caller-identity")
    if found["Account"] != ACCOUNT:
        raise RuntimeError("Unexpected AWS account; refusing mutation")
    return found


def require_configuration():
    if not PROFILE or len(ACCOUNT) != 12 or not ACCOUNT.isascii() or not ACCOUNT.isdigit():
        raise RuntimeError("Set AWS_PROFILE and POWERTOOLS_TEST_ACCOUNT explicitly before AWS integration operations")


def state():
    require_configuration()
    value = json.loads(STATE.read_text(encoding="utf-8"))
    if value["account"] != ACCOUNT or value["region"] != REGION or not value["name"].startswith("ptgo-it-"):
        raise RuntimeError("Unrecognized integration resource scope")
    return value


def prepare():
    require_configuration()
    OUT.mkdir(parents=True, exist_ok=True)
    if STATE.exists():
        run = state()
        if run.get("stack_created") or run.get("bucket_created"):
            raise RuntimeError("Existing cloud run must be cleaned before preparing another run")
        if run.get("cleaned_at"):
            raise RuntimeError("Archive dist/integration before starting a new run; completed evidence must not be overwritten")
    else:
        name = "ptgo-it-" + dt.datetime.now(dt.UTC).strftime("%Y%m%d%H%M") + "-" + secrets.token_hex(3)
        run = {"name": name, "account": ACCOUNT, "region": REGION, "bucket": name + "-" + ACCOUNT, "hashes": {}, "collector_version": COLLECTOR}
        save(STATE, run)
    for arch in ("amd64", "arm64"):
        target = OUT / arch
        target.mkdir(exist_ok=True)
        env = {**os.environ, "CGO_ENABLED": "0", "GOWORK": str(ROOT / "go.work"), "GOOS": "linux", "GOARCH": arch}
        subprocess.run(["go", "build", "-trimpath", "-tags", "lambda.norpc", "-o", str(target / "bootstrap"), "./integration/lambda"], cwd=ROOT, env=env, check=True)
        binary = (target / "bootstrap").read_bytes()
        expected = 62 if arch == "amd64" else 183
        if binary[:4] != b"\x7fELF" or int.from_bytes(binary[18:20], "little") != expected:
            raise RuntimeError("Cross-compilation produced an incorrect ELF")
        with zipfile.ZipFile(OUT / f"function-{arch}.zip", "w", zipfile.ZIP_DEFLATED) as archive:
            entry = zipfile.ZipInfo("bootstrap")
            entry.create_system = 3
            entry.external_attr = 0o100755 << 16
            entry.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(entry, binary)
            archive.write(ROOT / "integration" / "collector.yaml", "collector.yaml")
        layer = OUT / f"collector-{arch}.zip"
        url = f"https://github.com/open-telemetry/opentelemetry-lambda/releases/download/layer-collector%2F{COLLECTOR}/opentelemetry-collector-layer-{arch}.zip"
        if not layer.exists():
            print(f"Downloading official collector {COLLECTOR} for {arch}", flush=True)
            with urllib.request.urlopen(url, timeout=60) as response:
                layer.write_bytes(response.read())
        with zipfile.ZipFile(layer) as archive:
            if not any(name.startswith("extensions/") for name in archive.namelist()):
                raise RuntimeError("Collector archive does not contain a Lambda extension")
        for path in (target / "bootstrap", layer, OUT / f"function-{arch}.zip"):
            run["hashes"][str(path.relative_to(OUT))] = hashlib.sha256(path.read_bytes()).hexdigest()
        print(f"Prepared Linux {arch}, CGO_ENABLED=0", flush=True)
    save(STATE, run)
    save(OUT / "template.json", template(run))
    print(f"Prepared {run['name']} in {REGION}; cloud resources have not been created", flush=True)


def template(run):
    name, bucket = run["name"], run["bucket"]
    resources = {
        "Table": {"Type": "AWS::DynamoDB::Table", "Properties": {"TableName": name + "-table", "BillingMode": "PAY_PER_REQUEST", "AttributeDefinitions": [{"AttributeName": "id", "AttributeType": "S"}], "KeySchema": [{"AttributeName": "id", "KeyType": "HASH"}]}},
        "Role": {"Type": "AWS::IAM::Role", "Properties": {
            "AssumeRolePolicyDocument": {"Version": "2012-10-17", "Statement": [{"Effect": "Allow", "Principal": {"Service": "lambda.amazonaws.com"}, "Action": "sts:AssumeRole"}]},
            "Policies": [{"PolicyName": "IntegrationOnly", "PolicyDocument": {"Version": "2012-10-17", "Statement": [
                {"Effect": "Allow", "Action": ["logs:CreateLogStream", "logs:PutLogEvents"], "Resource": f"arn:aws:logs:{REGION}:{ACCOUNT}:log-group:/aws/lambda/{name}-*:*"},
                {"Effect": "Allow", "Action": ["xray:PutTraceSegments", "xray:PutTelemetryRecords"], "Resource": "*"},
                {"Effect": "Allow", "Action": "dynamodb:GetItem", "Resource": {"Fn::GetAtt": ["Table", "Arn"]}},
                {"Effect": "Allow", "Action": "s3:GetObject", "Resource": f"arn:aws:s3:::{bucket}/fixture.txt"}
            ]}}]}}
    }
    for arch, lambda_arch in (("amd64", "x86_64"), ("arm64", "arm64")):
        suffix = arch.title()
        resources["Collector" + suffix] = {"Type": "AWS::Lambda::LayerVersion", "Properties": {"LayerName": name + "-collector-" + arch, "CompatibleArchitectures": [lambda_arch], "Content": {"S3Bucket": bucket, "S3Key": f"collector-{arch}.zip"}, "Description": f"Temporary OpenTelemetry collector {COLLECTOR}; integration test only"}}
        for backend in ("otel",):
            key = backend.title() + suffix
            function = name + "-" + backend + "-" + arch
            resources["Logs" + key] = {"Type": "AWS::Logs::LogGroup", "Properties": {"LogGroupName": "/aws/lambda/" + function, "RetentionInDays": 1}}
            env = {"TRACE_BACKEND": backend, "TEST_TABLE": {"Ref": "Table"}, "TEST_BUCKET": bucket, "POWERTOOLS_LOG_LEVEL": "INFO", "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318", "OPENTELEMETRY_COLLECTOR_CONFIG_URI": "/var/task/collector.yaml"}
            props = {"FunctionName": function, "Runtime": "provided.al2023", "Handler": "bootstrap", "Architectures": [lambda_arch], "MemorySize": 512, "Timeout": 20, "Role": {"Fn::GetAtt": ["Role", "Arn"]}, "Code": {"S3Bucket": bucket, "S3Key": f"function-{arch}.zip"}, "Environment": {"Variables": env}, "TracingConfig": {"Mode": "Active"}}
            if backend == "otel":
                props["Layers"] = [{"Ref": "Collector" + suffix}]
            resources[key] = {"Type": "AWS::Lambda::Function", "DependsOn": "Logs" + key, "Properties": props}
    return {"AWSTemplateFormatVersion": "2010-09-09", "Description": "Disposable Powertools Go integration tests, Hong Kong only, no production dependencies", "Resources": resources}


def wait_stack(run, deletion=False):
    last = None
    for _ in range(90):
        try:
            current = aws("cloudformation", "describe-stacks", "--stack-name", run.get("stack_id", run["name"]))["Stacks"][0]["StackStatus"]
        except RuntimeError as error:
            if deletion and "does not exist" in str(error):
                return
            raise
        if current != last:
            print(f"CloudFormation: {current}", flush=True)
            last = current
        if current == ("DELETE_COMPLETE" if deletion else "CREATE_COMPLETE"):
            return
        if "FAILED" in current or "ROLLBACK" in current:
            events = aws("cloudformation", "describe-stack-events", "--stack-name", run["name"])
            save(OUT / "stack-failure.json", events)
            raise RuntimeError(f"Stack entered {current}; see stack-failure.json")
        time.sleep(8)
    raise TimeoutError("CloudFormation did not reach the expected state within 12 minutes")


def deploy():
    identity()
    run = state()
    if run.get("stack_created"):
        wait_stack(run)
        return
    if not run.get("bucket_created"):
        aws("s3api", "create-bucket", "--bucket", run["bucket"], "--create-bucket-configuration", f"LocationConstraint={REGION}")
        run["bucket_created"] = True
        save(STATE, run)
    aws("s3api", "put-public-access-block", "--bucket", run["bucket"], "--public-access-block-configuration", "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true")
    (OUT / "fixture.txt").write_text("powertools-integration\n", encoding="utf-8")
    existing = {o["Key"]: o for o in aws("s3api", "list-objects-v2", "--bucket", run["bucket"]).get("Contents", [])}
    for filename in ("function-amd64.zip", "function-arm64.zip", "collector-amd64.zip", "collector-arm64.zip", "fixture.txt"):
        local = OUT / filename
        # Single-part ETags permit resuming the initial uploads after a network interruption.
        checksum = hashlib.md5(local.read_bytes(), usedforsecurity=False).hexdigest()
        if filename in run.get("uploaded", []) or existing.get(filename, {}).get("ETag", "").strip('"') == checksum:
            continue
        print(f"Uploading {filename} with multipart-capable transfer", flush=True)
        aws("s3", "cp", local, f"s3://{run['bucket']}/{filename}", "--only-show-errors", "--no-progress", raw=True)
        run.setdefault("uploaded", []).append(filename)
        save(STATE, run)
    created = aws("cloudformation", "create-stack", "--stack-name", run["name"], "--template-body", "file://" + str(OUT / "template.json"), "--capabilities", "CAPABILITY_IAM", "--tags", "Key=Purpose,Value=PowertoolsIntegration")
    run.update(stack_created=True, stack_id=created["StackId"])
    save(STATE, run)
    wait_stack(run)
    print("Both OpenTelemetry test functions are ready in ap-east-1", flush=True)


def invoke_cases(run, backend, arch):
    import boto3
    client = boto3.Session(profile_name=PROFILE, region_name=REGION).client("lambda")
    function = run["name"] + "-" + backend + "-" + arch
    results = []
    for mode in ("native", "success", "error", "unsampled", "panic"):
        test_id = backend + "-" + arch + "-" + mode
        trace_id = "1-" + format(int(time.time()), "08x") + "-" + secrets.token_hex(12)
        event = {"id": test_id, "mode": mode, "add_temporary": mode == "native"}
        trace_header = f"Root={trace_id};Parent={secrets.token_hex(8)};Sampled={0 if mode == 'unsampled' else 1}"
        def inject_header(params, **kwargs):
            if mode != "native":
                params["headers"]["X-Amzn-Trace-Id"] = trace_header
        client.meta.events.register("before-call.lambda.Invoke", inject_header)
        payload, response_path = OUT / (test_id + "-event.json"), OUT / (test_id + "-response.json")
        save(payload, event)
        meta = client.invoke(FunctionName=function, Payload=json.dumps(event).encode(), LogType="Tail")
        client.meta.events.unregister("before-call.lambda.Invoke", inject_header)
        response = json.loads(meta.pop("Payload").read())
        save(response_path, response)
        meta.pop("ResponseMetadata", None)
        log_tail = base64.b64decode(meta.pop("LogResult", "")).decode("utf-8", errors="replace")
        item = {"id": test_id, "function": function, "backend": backend, "arch": arch, "mode": mode, "expected_trace_id": trace_id if mode != "native" else None, "metadata": meta, "response": response, "tail": log_tail}
        results.append(item)
        save(OUT / (test_id + "-result.json"), item)
        print(f"Invoked {test_id}: {'function error' if 'FunctionError' in meta else 'returned'}", flush=True)
    return results


def all_nodes(document):
    yield document
    for child in document.get("subsegments", []):
        yield from all_nodes(child)


def test():
    identity()
    run = state()
    if not run.get("stack_created"):
        raise RuntimeError("No deployed test stack")
    run["test_started_ms"] = int(time.time() * 1000)
    save(STATE, run)
    results = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        tasks = [pool.submit(invoke_cases, run, backend, arch) for backend in ("otel",) for arch in ("amd64", "arm64")]
        for task in tasks:
            results.extend(task.result())
    save(OUT / "invocations.json", results)
    collect(run, results)


def collect(run, results):
    # CloudWatch and X-Ray ingestion are asynchronous; stop polling once evidence arrives.
    logs = {}
    traces = {}
    required = [r["expected_trace_id"] for r in results if r["mode"] in ("success", "error", "panic")]
    for attempt in range(5):
        time.sleep(12)
        for function in sorted({r["function"] for r in results}):
            logs[function] = aws("logs", "filter-log-events", "--log-group-name", "/aws/lambda/" + function, "--start-time", str(run["test_started_ms"]))
        trace_ids = [r["expected_trace_id"] for r in results if r["expected_trace_id"]]
        for offset in range(0, len(trace_ids), 5):
            batch = aws("xray", "batch-get-traces", "--trace-ids", *trace_ids[offset:offset + 5])
            for item in batch.get("Traces", []):
                traces[item["Id"]] = item
        complete = all(any("integration" in s["Document"] for s in traces.get(t, {}).get("Segments", [])) for t in required)
        seen_logs = all(r["id"] in json.dumps(logs[r["function"]]) for r in results)
        print(f"Telemetry ingestion check {attempt + 1}: {len(traces)} traces, logs {'ready' if seen_logs else 'pending'}", flush=True)
        if complete and seen_logs:
            break
    save(OUT / "cloudwatch.json", logs)
    save(OUT / "xray.json", traces)
    return evaluate(run, results, logs, traces)


def evaluate(run, results, logs, traces):
    checks = []
    def check(name, passed, detail=""):
        checks.append({"check": name, "passed": bool(passed), "detail": detail})
    for result in results:
        mode, response = result["mode"], result["response"]
        prefix = result["id"]
        expected_error = mode in ("error", "panic")
        check(prefix + ": result", ("FunctionError" in result["metadata"]) == expected_error)
        if expected_error:
            check(prefix + ": error preserved", "intentional integration " + mode in response.get("errorMessage", ""))
        else:
            check(prefix + ": runtime context", response.get("runtime_header_present") and bool(response.get("request_id")))
            check(prefix + ": result identity", response.get("id") == prefix)
            if mode != "native":
                expected = result["expected_trace_id"]
                if result["backend"] == "otel":
                    expected = expected[2:].replace("-", "")
                check(prefix + ": parent trace", response.get("trace_id") == expected)
                check(prefix + ": sampling", response.get("sampled") == (mode != "unsampled"))
        parsed = []
        for event in logs[result["function"]].get("events", []):
            try:
                record = json.loads(event["message"])
            except ValueError:
                continue
            if isinstance(record, dict) and record.get("test_id") == prefix:
                parsed.append(record)
        start = next((r for r in parsed if r.get("message") == "integration start"), {})
        check(prefix + ": structured log", bool(start.get("function_request_id")) and start.get("level") == "INFO" and start.get("service") == "powertools-integration-" + result["backend"])
        check(prefix + ": temporary isolation", ("temporary_marker" in start) == (mode == "native"))
        if mode in ("native", "success"):
            check(prefix + ": cold/warm flag", start.get("cold_start") == (mode == "native"))
        if not expected_error and result["backend"] == "otel":
            check(prefix + ": OTel log correlation", start.get("trace_id") == response.get("trace_id") and len(start.get("span_id", "")) == 16)
        buffered = any(r.get("message") == "buffered diagnostic" for r in parsed)
        check(prefix + ": buffer lifecycle", buffered == expected_error)
        if mode == "unsampled":
            check(prefix + ": no application trace", result["expected_trace_id"] not in traces)
        elif mode != "native":
            nodes = [n for s in traces.get(result["expected_trace_id"], {}).get("Segments", []) for n in all_nodes(json.loads(s["Document"]))]
            handler = [n for n in nodes if n.get("name") in ("## integration", "powertools-integration-" + result["backend"])]
            business = [n for n in nodes if n.get("name") == "### business"]
            check(prefix + ": closed handler and business spans", bool(handler and business) and all(n.get("end_time") and not n.get("in_progress") for n in handler + business))
            check(prefix + ": indexed annotation", any(n.get("annotations", {}).get("TestID") == prefix for n in nodes))
            check(prefix + ": trace metadata", any(prefix in json.dumps(n.get("metadata", {})) for n in business))
            if mode == "success":
                operation = "DynamoDB/GetItem" if result["backend"] == "otel" else "GetItem"
                def matches_dynamodb(node):
                    details = node.get("aws", {})
                    if details.get("operation") != operation or not details.get("request_id") or details.get("region") != REGION:
                        return False
                    if result["backend"] == "otel":
                        return details.get("table_name") == run["name"] + "-table"
                    # The selected X-Ray SDK middleware omits table_name; record this
                    # compatibility gap instead of requiring an undocumented SDK field.
                    return node.get("name") == "DynamoDB" and node.get("namespace") == "aws"
                check(prefix + ": DynamoDB span", any(matches_dynamodb(n) for n in nodes))
                check(prefix + ": HTTP span", any(n.get("http", {}).get("response", {}).get("status") == 200 for n in nodes))
            else:
                check(prefix + ": trace error", any(n.get("fault") or n.get("error") for n in business))
    for backend in sorted({result["backend"] for result in results}):
        for arch in ("amd64", "arm64"):
            pair = [r for r in results if r["backend"] == backend and r["arch"] == arch and r["mode"] in ("native", "success")]
            check(backend + "-" + arch + ": warm reuse", pair[0]["response"].get("instance") == pair[1]["response"].get("instance") and pair[1]["response"].get("invocation") == 2)
    for function, value in logs.items():
        text = json.dumps(value)
        check(function + ": instrumentation diagnostics", "TRACER_FAILURE" not in text and "LOGGER_FAILURE" not in text)
    report = {"account": ACCOUNT, "region": REGION, "stack": run["name"], "checked_at": dt.datetime.now(dt.UTC).isoformat(), "checks": checks, "passed": sum(c["passed"] for c in checks), "total": len(checks)}
    save(OUT / "report.json", report)
    print(f"Acceptance: {report['passed']}/{report['total']} passed", flush=True)
    for item in checks:
        if not item["passed"]:
            print("FAIL: " + item["check"], flush=True)
    if report["passed"] != report["total"]:
        raise RuntimeError("Acceptance checks failed; evidence retained for diagnosis")


def cleanup():
    identity()
    run = state()
    if run.get("stack_created"):
        aws("cloudformation", "delete-stack", "--stack-name", run["stack_id"])
        wait_stack(run, deletion=True)
        run["stack_created"] = False
        save(STATE, run)
    if run.get("bucket_created"):
        for upload in aws("s3api", "list-multipart-uploads", "--bucket", run["bucket"]).get("Uploads", []):
            aws("s3api", "abort-multipart-upload", "--bucket", run["bucket"], "--key", upload["Key"], "--upload-id", upload["UploadId"])
        objects = aws("s3api", "list-objects-v2", "--bucket", run["bucket"])
        for item in objects.get("Contents", []):
            aws("s3api", "delete-object", "--bucket", run["bucket"], "--key", item["Key"])
        aws("s3api", "delete-bucket", "--bucket", run["bucket"])
        run["bucket_created"] = False
    run["cleaned_at"] = dt.datetime.now(dt.UTC).isoformat()
    save(STATE, run)
    print("Temporary stack, functions, layers, table, logs, role, and bucket removed", flush=True)


def refresh():
    """Rebuild changed fixtures and update only resources recorded for this test run."""
    identity()
    run = state()
    if not run.get("stack_created"):
        raise RuntimeError("No stack to update")
    for filename in ("invocations.json", "cloudwatch.json", "xray.json", "report.json"):
        path = OUT / filename
        if path.exists():
            (OUT / ("initial-" + filename)).write_bytes(path.read_bytes())
    revision = secrets.token_hex(3)
    revised = template(run)
    for arch in ("amd64", "arm64"):
        target = OUT / arch / "bootstrap"
        subprocess.run(["go", "build", "-trimpath", "-tags", "lambda.norpc", "-o", str(target), "./integration/lambda"], cwd=ROOT, env={**os.environ, "CGO_ENABLED": "0", "GOWORK": str(ROOT / "go.work"), "GOOS": "linux", "GOARCH": arch}, check=True)
        path = OUT / f"function-{arch}.zip"
        with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as archive:
            entry = zipfile.ZipInfo("bootstrap")
            entry.create_system = 3
            entry.external_attr = 0o100755 << 16
            entry.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(entry, target.read_bytes())
            archive.write(ROOT / "integration" / "collector.yaml", "collector.yaml")
        code_key = f"function-{arch}-{revision}.zip"
        layer_key = f"collector-custom-{arch}-{revision}.zip"
        aws("s3", "cp", path, f"s3://{run['bucket']}/{code_key}", "--only-show-errors", "--no-progress", raw=True)
        aws("s3", "cp", OUT / f"collector-custom-{arch}.zip", f"s3://{run['bucket']}/{layer_key}", "--only-show-errors", "--no-progress", raw=True)
        revised["Resources"]["Collector" + arch.title()]["Properties"]["Content"]["S3Key"] = layer_key
        for backend in ("otel",):
            revised["Resources"][backend.title() + arch.title()]["Properties"]["Code"]["S3Key"] = code_key
        run["hashes"][str(target.relative_to(OUT))] = hashlib.sha256(target.read_bytes()).hexdigest()
        run["hashes"][code_key] = hashlib.sha256(path.read_bytes()).hexdigest()
    save(OUT / "template.json", revised)
    save(STATE, run)
    aws("cloudformation", "update-stack", "--stack-name", run["stack_id"], "--template-body", "file://" + str(OUT / "template.json"), "--capabilities", "CAPABILITY_IAM")
    for _ in range(90):
        current = aws("cloudformation", "describe-stacks", "--stack-name", run["stack_id"])["Stacks"][0]["StackStatus"]
        if current == "UPDATE_COMPLETE":
            print("Updated test fixtures and collector in Hong Kong", flush=True)
            return
        if "FAILED" in current or "ROLLBACK" in current:
            raise RuntimeError("Fixture update failed: " + current)
        time.sleep(8)
    raise TimeoutError("Fixture update did not complete")


if __name__ == "__main__":
    def verify():
        evidence = [json.loads((OUT / filename).read_text(encoding="utf-8")) for filename in ("invocations.json", "cloudwatch.json", "xray.json")]
        return evaluate(state(), *evidence)
    phases = {"prepare": prepare, "deploy": deploy, "refresh": refresh, "test": test, "collect": lambda: collect(state(), json.loads((OUT / "invocations.json").read_text(encoding="utf-8"))), "verify": verify, "cleanup": cleanup}
    if len(sys.argv) != 2 or sys.argv[1] not in phases:
        raise SystemExit("Usage: run.py prepare|deploy|test|collect|cleanup")
    phases[sys.argv[1]]()
