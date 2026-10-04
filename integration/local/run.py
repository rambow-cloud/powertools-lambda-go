"""Run isolated Lambda RIE + OTLP integration checks without AWS credentials."""

import base64
import argparse
import gzip
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import uuid
import zlib

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / "dist" / "local"
IMAGE = "public.ecr.aws/lambda/provided@sha256:0b17e5c778aef6ed7f61cbfa5dd416e17540e127846250646fceb712e0e7be5f"


def run(*args, env=None):
    result = subprocess.run(args, cwd=ROOT, env=env, text=True, encoding="utf-8", capture_output=True)
    if result.returncode:
        (OUT / "command-failure.log").write_text(result.stdout + "\n" + result.stderr, encoding="utf-8")
        raise RuntimeError(f"{args[0]} {args[1:]} failed:\n{result.stdout[-4000:]}\n{result.stderr[-4000:]}")
    return result.stdout.strip()


def request(container, url, payload=None):
    args = ["docker", "exec", container, "curl", "--silent", "--show-error", "--max-time", "30", "--header", "Content-Type: application/json", "--write-out", "\n%{http_code}"]
    if payload is not None:
        args.extend(["--data-binary", json.dumps(payload)])
    args.append(url)
    body, status = run(*args).rsplit("\n", 1)
    response = json.loads(body)
    response["_http_status"] = int(status)
    return response


def attributes(span):
    return {entry["key"]: next(iter(entry["value"].values())) for entry in span.get("attributes", [])}


def prepare(skip_module_checks=False):
    OUT.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env.update(CGO_ENABLED="0", GOWORK=str(ROOT / "go.work"), GOMAXPROCS="2", GOFLAGS="-p=1", GOMEMLIMIT="128MiB", GOGC="20", GOCACHE=str(ROOT / "dist" / "integration" / "go-cache"), GOTMPDIR=str(OUT / "go-tmp"))
    Path(env["GOTMPDIR"]).mkdir(exist_ok=True)
    env.pop("GOOS", None)
    env.pop("GOARCH", None)
    if not skip_module_checks:
        print("Verifying packaged Go modules, tests, vet, and standalone consumers", flush=True)
        subprocess.run([sys.executable, str(ROOT / "tools/modules.py"), "check"], cwd=ROOT, env=env, check=True)
    for arch in ("amd64", "arm64"):
        directory = OUT / arch
        directory.mkdir(exist_ok=True)
        env.update(GOOS="linux", GOARCH=arch)
        print(f"Cross-compiling linux/{arch} with CGO_ENABLED=0", flush=True)
        run("go", "build", "-trimpath", "-o", str(directory / "bootstrap"), "./integration/lambda", env=env)
        run("go", "build", "-trimpath", "-o", str(directory / "stream-bootstrap"), "./integration/streaming", env=env)
    env["GOARCH"] = "amd64"
    run("go", "build", "-trimpath", "-o", str(OUT / "amd64" / "capture"), "./integration/local/capture", env=env)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runtime-only", action="store_true", help="Use existing dist/local binaries; skip previously completed Go validation and builds")
    parser.add_argument("--skip-module-checks", action="store_true", help="Reuse completed module checks; still rebuild both architectures before Docker acceptance")
    args = parser.parse_args()
    if not args.runtime_only:
        prepare(args.skip_module_checks)
    prefix = "ptgo-local-" + uuid.uuid4().hex[:10]
    network, capture, function, cache = prefix, prefix + "-capture", prefix + "-lambda", prefix + "-cache"
    containers = []
    report = {"cgo_enabled": False, "image": IMAGE, "runtime_architecture": "amd64", "go_validation_executed": not args.runtime_only and not args.skip_module_checks, "builds_executed": not args.runtime_only, "binary_architectures": ["amd64", "arm64"], "checks": [], "invocations": [], "completed": False}

    def check(name, ok):
        report["checks"].append({"name": name, "passed": bool(ok)})

    report["utilities"] = ["logger", "tracer", "metrics", "parameters", "commons", "metadata", "signer", "jmespath", "batch", "idempotency", "idempotency/cache", "parser", "validation", "eventhandler/http", "eventhandler/appsyncevents", "eventhandler/appsyncgraphql", "eventhandler/bedrock", "kafka", "kafka/avro", "kafka/protobuf", "datamasking", "datamasking/kms"]
    report["cache_image"] = "valkey/valkey@sha256:d2e18f3410b6f616de1417f570fa55261af2898b9c5b2cfb6781ce2373ea43d1"
    run("docker", "network", "create", "--internal", network)
    try:
        run("docker", "run", "-d", "--name", cache, "--network", network, "--network-alias", "cache", "--platform", "linux/amd64", "--tmpfs", "/data", report["cache_image"], "valkey-server", "--save", "", "--appendonly", "no")
        containers.append(cache)
        for attempt in range(20):
            ready = subprocess.run(["docker", "exec", cache, "valkey-cli", "ping"], capture_output=True, text=True)
            if ready.returncode == 0 and ready.stdout.strip() == "PONG":
                break
            time.sleep(0.25)
        else:
            raise RuntimeError("Valkey did not become ready")
        bridge = ROOT / "tools/reference/bridge-idempotency-cache.mjs"
        seed = json.loads(run("node", str(bridge), "seed", cache))
        check("Cache: TypeScript persistence seed", seed.get("seeded") is True)
        volume = f"{OUT / 'amd64'}:/var/task:ro"
        run("docker", "run", "-d", "--name", capture, "--network", network, "--network-alias", "capture", "--platform", "linux/amd64", "-v", volume, "--entrypoint", "/var/task/capture", IMAGE)
        containers.append(capture)
        settings = {
            "LOCAL_TEST": "true", "TRACE_BACKEND": "otel", "AWS_REGION": "ap-east-1",
            # Keep constructor diagnostics separate from behavioral probe records.
            "TZ": "UTC",
            "AWS_ACCESS_KEY_ID": "LOCALTESTONLY", "AWS_SECRET_ACCESS_KEY": "local-test-only",
            "AWS_EC2_METADATA_DISABLED": "true", "TEST_TABLE": "local-orders",
            "AWS_LAMBDA_FUNCTION_NAME": "powertools-local", "AWS_LAMBDA_FUNCTION_MEMORY_SIZE": "256", "AWS_LAMBDA_INITIALIZATION_TYPE": "on-demand",
            "OTEL_EXPORTER_OTLP_ENDPOINT": "http://capture:4318", "OTEL_EXPORTER_OTLP_TIMEOUT": "1000",
        }
        args = ["docker", "run", "-d", "--name", function, "--network", network, "--network-alias", "function", "--platform", "linux/amd64", "-v", volume]
        for key, value in settings.items():
            args.extend(["-e", f"{key}={value}"])
        run(*args, IMAGE, "bootstrap")
        containers.append(function)
        invoke_url = "http://function:8080/2015-03-31/functions/function/invocations"
        for index, mode in enumerate(("success", "success", "error", "unsampled", "panic")):
            test_id = f"local-{index}-{mode}"
            root = f"1-{int(time.time()):08x}-{uuid.uuid4().hex[:24]}"
            sampled = mode != "unsampled"
            payload = {"id": test_id, "mode": mode, "add_temporary": index == 0, "trace_header": f"Root={root};Parent=1234567890abcdef;Sampled={int(sampled)}"}
            payload["query_body"] = json.dumps({"items": [{"id": "enabled", "enabled": True}, {"id": "disabled", "enabled": False}]})
            if index in (1, 2):
                payload.update(trace_header="", traceparent=f"00-{root.replace('-', '')[1:]}-1234567890abcdef-01")
            response = request(capture, invoke_url, payload)
            report["invocations"].append({"event": payload, "response": response})
            failed = mode in ("error", "panic")
            check(f"{test_id}: result", ("errorMessage" in response) == failed)
            if mode == "error":
                check(f"{test_id}: original error", f"intentional integration {mode}" in response.get("errorMessage", ""))
            elif mode == "panic":
                check(f"{test_id}: runtime panic response", response["_http_status"] in (200, 502) and response.get("errorType") in ("string", "Runtime.ExitError"))
            else:
                check(f"{test_id}: HTTP success", response["_http_status"] == 200)
                check(f"{test_id}: request identity", response.get("id") == test_id and bool(response.get("request_id")))
                check(f"{test_id}: trace identity", response.get("trace_id") == root.replace("-", "")[1:])
                check(f"{test_id}: sampling", response.get("sampled") == sampled)
                logger_parity = response.get("logger_parity", {})
                child_records = logger_parity.get("child_records", [])
                check(f"{test_id}: Logger child persistent snapshot", logger_parity.get("child_persistent") == {"shared": "child"})
                check(f"{test_id}: Logger inherited temporary precedence", len(child_records) == 4 and child_records[0].get("shared") == "temporary" and child_records[0].get("request") == "original")
                check(f"{test_id}: Logger child reset and parent isolation", len(child_records) == 4 and child_records[1].get("shared") == "child" and "request" not in child_records[1] and child_records[2].get("shared") == "temporary" and child_records[2].get("request") == "original")
                check(f"{test_id}: Logger shallow empty cleanup", len(child_records) == 4 and all(key not in child_records[3] for key in ("message", "empty", "null_value")) and child_records[3].get("zero") == 0 and child_records[3].get("flag") is False and child_records[3].get("nested") == {"empty": "", "null_value": None})
                check(f"{test_id}: Logger probe Lambda context", len(child_records) == 4 and all(item.get("function_request_id") == response.get("request_id") and item.get("trace_id") == response.get("trace_id") for item in child_records))
                trace_records = logger_parity.get("trace_records", [])
                check(f"{test_id}: Logger buffer trace isolation", logger_parity.get("wrong_trace_no_output") is True and len(trace_records) == 1 and trace_records[0].get("message") == "new trace" and trace_records[0].get("trace_id") == "87654321123456789012345678901234")
                overflow_records = logger_parity.get("overflow_records", [])
                check(f"{test_id}: Logger overflow diagnostic and fallback", len(overflow_records) == 2 and overflow_records[0].get("level") == "WARN" and overflow_records[0].get("error", {}).get("message") == "Item too big" and overflow_records[1].get("level") == "DEBUG" and overflow_records[1].get("message") == "oversize")
                handler_trace_records = logger_parity.get("handler_trace_records", [])
                check(f"{test_id}: Logger handler-started trace error flush", logger_parity.get("handler_result_preserved") is True and len(handler_trace_records) == 2 and handler_trace_records[0].get("message") == "handler trace detail" and handler_trace_records[0].get("trace_id") == "12345678123456789012345678901234" and handler_trace_records[1].get("level") == "ERROR")
                marshaler_records = logger_parity.get("marshaler_records", [])
                check(f"{test_id}: Logger empty-string custom marshaler value", len(marshaler_records) == 1 and marshaler_records[0].get("payload") == "custom value")
                check(f"{test_id}: Logger empty-string custom marshaler error", logger_parity.get("marshaler_error_preserved") is True)
                text_marshaler_records = logger_parity.get("text_marshaler_records", [])
                check(f"{test_id}: Logger empty-string text marshaler value", len(text_marshaler_records) == 1 and text_marshaler_records[0].get("payload") == "custom text")
                check(f"{test_id}: Logger empty-string text marshaler error", logger_parity.get("text_marshaler_error_preserved") is True)
                stores = response.get("metrics_stores", {})
                check(f"{test_id}: Metrics store presence and empty policy", stores.get("before_clear") is True and stores.get("after_clear") is False and stores.get("empty_error") is True)
                document = stores.get("serialized", {})
                directive = document.get("_aws", {}).get("CloudWatchMetrics", [{}])[0]
                check(f"{test_id}: Metrics selective clearing", set(document) == {"_aws", "service"} and directive.get("Metrics") == [] and directive.get("Dimensions") == [["service"]])
                check(f"{test_id}: Metrics timestamp cleared", isinstance(document.get("_aws", {}).get("Timestamp"), int) and document["_aws"]["Timestamp"] > stores.get("old_timestamp", 0))
                diagnostics = stores.get("diagnostics", {})
                diagnostic_document = diagnostics.get("serialized", {})
                check(f"{test_id}: Metrics warning sequence", diagnostics.get("warnings") == [
                    "The dimension invalid doesn't meet the requirements and won't be added. Ensure the dimension name and value are non empty strings",
                    'Dimension "service" has already been added. The previous value will be overwritten.',
                    "This metric doesn't meet the requirements and will be skipped by Amazon CloudWatch. Ensure the timestamp is within 14 days in the past or up to 2 hours in the future and is also a valid number or Date object.",
                    'EMF key "service" is defined as both a metadata and default dimension; the default dimension value will take precedence in the serialized output',
                    'EMF key "service" is defined as both a default dimension and dimension; the dimension value will take precedence in the serialized output',
                ])
                check(f"{test_id}: Metrics invalid dimension skipped", set(diagnostic_document) == {"_aws", "service", "Count"} and diagnostic_document.get("service") == "checkout" and diagnostic_document.get("Count") == 1)
                check(f"{test_id}: Metrics warned timestamp retained", isinstance(diagnostics.get("timestamp"), int) and diagnostic_document.get("_aws", {}).get("Timestamp") == diagnostics["timestamp"])
                check(f"{test_id}: Metrics callbacks observe cleared state", diagnostics.get("callback_saw_metrics") is False)
                check(f"{test_id}: Metrics diagnostic scope closed", diagnostics.get("closed") is True)
                cold_start = stores.get("cold_start", {})
                cold_documents = cold_start.get("documents", [])
                expected_cold_count = 1 if test_id == "local-0-success" else 0
                check(f"{test_id}: Metrics manual capture composes with wrapper", stores.get("manual_capture_ok") is True)
                check(f"{test_id}: Metrics manual cold and warm isolation", isinstance(cold_documents, list) and len(cold_documents) == expected_cold_count)
                check(f"{test_id}: Metrics scoped function name", len(cold_documents) == expected_cold_count and all(set(doc) == {"_aws", "service", "function_name", "ColdStart"} and doc.get("function_name") == "scoped" and doc.get("service") == "orders" and doc.get("ColdStart") == 1 and doc.get("_aws", {}).get("CloudWatchMetrics", [{}])[0].get("Namespace") == "ColdProbe" for doc in cold_documents))
                check(f"{test_id}: Metrics cold-start late writes rejected", cold_start.get("closed") is True)
                configuration = stores.get("configuration", {})
                check(f"{test_id}: Metrics custom configuration getter order", configuration.get("calls") == ["namespace", "service"])
                config_parent = configuration.get("parent", {})
                config_child = configuration.get("child", {})
                check(f"{test_id}: Metrics custom configuration values", config_parent.get("service") == "custom-service" and config_parent.get("Parent") == 2 and config_parent.get("_aws", {}).get("CloudWatchMetrics", [{}])[0].get("Namespace") == "Configured")
                check(f"{test_id}: Metrics single configuration reconstructed", configuration.get("parent_disabled") is True and configuration.get("child_disabled") is False)
                check(f"{test_id}: Metrics single default service restored", set(config_child) == {"_aws", "service", "Child"} and config_child.get("service") == "service_undefined" and config_child.get("Child") == 3 and config_child.get("_aws", {}).get("CloudWatchMetrics", [{}])[0].get("Namespace") == "Configured")
                check(f"{test_id}: Metrics reconstructed single scope closed", configuration.get("closed") is True)
                values = stores.get("values", {})
                value_document = values.get("document", {})
                value_definitions = value_document.get("_aws", {}).get("CloudWatchMetrics", [{}])[0].get("Metrics", [])
                check(f"{test_id}: Metrics non-finite JSON values", value_document.get("NonFinite") == [None, None, None, 0])
                check(f"{test_id}: Metrics numeric key ordering", [item.get("Name") for item in value_definitions] == ["2", "10", "01", "NonFinite"])
                check(f"{test_id}: Metrics reserved envelope precedence", values.get("reserved", {}).get("_aws") == {"custom": True})
                check(f"{test_id}: Metrics prototype-name behavior", "__proto__" not in value_document and values.get("prototype_error") == 'Metric "constructor" has already been added with unit "undefined", but we received unit "Count". Did you mean to use metric unit "undefined"?')
                check(f"{test_id}: Metrics value scope cleanup", values.get("closed") is True)
                timestamps = stores.get("timestamps", {})
                timestamp_values = [item.get("_aws", {}).get("Timestamp") for item in timestamps.get("documents", [])]
                check(f"{test_id}: Metrics invalid numeric timestamps", timestamp_values[:2] == [0, 0])
                check(f"{test_id}: Metrics invalid Date timestamp", len(timestamp_values) == 5 and timestamp_values[2] is None)
                check(f"{test_id}: Metrics inclusive timestamp limits", timestamp_values[3:] == [1788782400000, 1789999200000])
                check(f"{test_id}: Metrics explicit timestamp clock reads", timestamps.get("explicit_clock_calls") == 3 and timestamps.get("warnings") == 3)
                check(f"{test_id}: Metrics timestamp reset clock reads", timestamps.get("reset", {}).get("_aws", {}).get("Timestamp") == 1789992000000 and timestamps.get("reset_clock_calls") == 4 and timestamps.get("final_clock_calls") == 5)
                check(f"{test_id}: Metrics timestamp late writes rejected", timestamps.get("closed") is True)
                wrappers = stores.get("wrappers", {})
                wrapper_documents = wrappers.get("documents", [])
                wrapper_metrics = [item for item in wrapper_documents if "Count" in item]
                wrapper_cold = [item for item in wrapper_documents if "ColdStart" in item]
                check(f"{test_id}: Metrics wrapper result and order", wrappers.get("value") == 42 and [item.get("_aws", {}).get("CloudWatchMetrics", [{}])[0].get("Namespace") for item in wrapper_metrics] == ["WrapperA", "WrapperB"])
                check(f"{test_id}: Metrics wrapper default dimensions", len(wrapper_metrics) == 2 and all(item.get("Count") == 1 and item.get("stage") == "wrapper" and item.get("service") == "orders" for item in wrapper_metrics))
                check(f"{test_id}: Metrics wrapper cold-start composition", len(wrapper_cold) == 2 * expected_cold_count and all(item.get("stage") == "wrapper" and item.get("ColdStart") == 1 for item in wrapper_cold))
                check(f"{test_id}: Metrics wrapper strict propagation", wrappers.get("strict_error") is True and wrappers.get("reports") == 1)
                check(f"{test_id}: Metrics wrapper publication stops", wrappers.get("stopped") is True)
                check(f"{test_id}: Metrics wrapper scope closure", wrappers.get("closed") is True and wrappers.get("skipped_closed") is True)
                routed = response.get("http", {})
                app_events = response.get("appsync_events", {})
                app_items = app_events.get("individual", {}).get("events", [])
                check(f"{test_id}: AppSync Events Parser and response order", app_events.get("parsed") is True and [item.get("id") for item in app_items] == ["one", "two"])
                check(f"{test_id}: AppSync Events invocation composition", len(app_items) == 2 and all(item.get("payload", {}).get("request_id") == response.get("request_id") and item.get("payload", {}).get("trace_id") == response.get("trace_id") and item.get("payload", {}).get("correlation_id") == f"query:{test_id}" for item in app_items))
                app_original = [{"id": "one", "payload": {"id": "order-a"}}, {"id": "two", "payload": {"id": "order-b"}}]
                check(f"{test_id}: AppSync Events aggregate precedence", app_events.get("aggregate", {}).get("events") == app_original)
                check(f"{test_id}: AppSync Events missing route passthrough", app_events.get("passthrough", {}).get("events") == app_original)
                check(f"{test_id}: AppSync Events item authorization envelopes", app_events.get("item_errors", {}).get("events") == [{"id": "one", "error": "UnauthorizedException - denied"}, {"id": "two", "error": "UnauthorizedException - denied"}])
                check(f"{test_id}: AppSync Events subscription and invalid input", app_events.get("authorization") is True and app_events.get("invalid") is True)
                graphql = response.get("appsync_graphql", {})
                single = graphql.get("single", {})
                check(f"{test_id}: GraphQL Parser and single resolver", graphql.get("parsed") is True and single.get("request_id") == response.get("request_id"))
                check(f"{test_id}: GraphQL Logger and OTel context", single.get("trace_id") == response.get("trace_id") and single.get("correlation_id") == f"query:{test_id}")
                check(f"{test_id}: GraphQL aggregate first-event route", graphql.get("aggregate") == ["short"])
                check(f"{test_id}: GraphQL sequential graceful batch", graphql.get("individual") == ["individual", None, "third"] and graphql.get("individual_calls") == 3)
                check(f"{test_id}: GraphQL batch abort", graphql.get("abort") == {"error": "Error - failed"} and graphql.get("abort_calls") == 2)
                check(f"{test_id}: GraphQL included exception handler", graphql.get("exception") == {"handled": "failed"})
                check(f"{test_id}: GraphQL missing and invalid input", graphql.get("missing") is True and graphql.get("invalid") is True)
                check(f"{test_id}: GraphQL datetime scalar", graphql.get("datetime") == "1970-01-01T05:30:00.000+05:30:00")
                bedrock = response.get("bedrock", {})
                def tool_response(name):
                    return bedrock.get(name, {}).get("response", {}).get("functionResponse", {})
                def tool_body(name):
                    return tool_response(name).get("responseBody", {}).get("TEXT", {}).get("body")
                parsed_tool = json.loads(tool_body("order") or "null")
                check(f"{test_id}: Bedrock conversion and Parser", isinstance(parsed_tool, dict) and parsed_tool.get("parameters") == {"count": 3.5, "flag": True, "invalid": "oops", "list": "[1,2]"})
                check(f"{test_id}: Bedrock Logger and OTel context", isinstance(parsed_tool, dict) and parsed_tool.get("request_id") == response.get("request_id") and parsed_tool.get("trace_id") == response.get("trace_id") and parsed_tool.get("correlation_id") == f"query:{test_id}")
                ordinary_tool = bedrock.get("order", {})
                check(f"{test_id}: Bedrock inherited session envelope", ordinary_tool.get("messageVersion") == "1.0" and ordinary_tool.get("response", {}).get("actionGroup") == "orders" and ordinary_tool.get("sessionAttributes") == {"saved": "session"} and ordinary_tool.get("promptSessionAttributes") == {"saved": "prompt"} and ordinary_tool.get("knowledgeBasesConfiguration") == {"id": "kb"})
                explicit_tool = bedrock.get("explicit", {})
                check(f"{test_id}: Bedrock explicit response isolation", tool_body("explicit") == "retry" and tool_response("explicit").get("responseState") == "REPROMPT" and explicit_tool.get("sessionAttributes") == {} and explicit_tool.get("promptSessionAttributes") == {} and "knowledgeBasesConfiguration" not in explicit_tool)
                check(f"{test_id}: Bedrock missing tool", tool_body("missing") == 'Error: tool "missing" has not been registered.' and "responseState" not in tool_response("missing"))
                check(f"{test_id}: Bedrock tool error", tool_body("error") == "Unable to complete tool execution due to TypeError - failed")
                check(f"{test_id}: Bedrock invalid and empty result", bedrock.get("invalid") is True and tool_body("empty") == "")
                check(f"{test_id}: Bedrock panic and string body", tool_body("panic") == "Unable to complete tool execution due to 42" and tool_body("string") == '"hello"')
                kafka = response.get("kafka", {})
                kafka_responses = kafka.get("responses", [])
                check(f"{test_id}: Kafka lazy access and Parser", kafka.get("lazy") is True and kafka.get("parses") == 3 and kafka.get("parser_rejected") is True and kafka.get("diagnostics") == 0)
                check(f"{test_id}: Kafka Idempotency duplicate suppression", kafka.get("executions") == 1 and len(kafka_responses) == 2 and kafka_responses[0] == kafka_responses[1])
                kafka_response = kafka_responses[0] if kafka_responses else {}
                check(f"{test_id}: Kafka Logger and OTel context", kafka_response.get("request_id") == response.get("request_id") and kafka_response.get("trace_id") == response.get("trace_id") and kafka_response.get("correlation_id") == f"query:{test_id}")
                check(f"{test_id}: Kafka topic order and metadata", kafka.get("offsets") == [2, 1] and kafka.get("event_source") == "SelfManagedKafka")
                check(f"{test_id}: Kafka tombstone and empty key", kafka.get("tombstone") is True and kafka.get("empty_key") is True)
                check(f"{test_id}: Kafka UTF-8 header", kafka.get("headers") == [{"name": "\u00e9"}])
                binary = kafka.get("binary", {})
                check(f"{test_id}: Kafka Avro record, union and bytes", binary.get("avro") == {"id": "order", "count": 7, "tag": {"string": "yes"}, "raw": "AP8="})
                check(f"{test_id}: Kafka Avro precision guard", binary.get("avro_precision_rejected") is True)
                protobuf = binary.get("protobuf", {})
                check(f"{test_id}: Kafka Protobuf plain and Glue", protobuf.get("plain") == "order" and protobuf.get("glue") == "order")
                check(f"{test_id}: Kafka Protobuf Confluent", protobuf.get("confluent") == "order")
                check(f"{test_id}: Kafka Protobuf adaptive index convention", protobuf.get("zigzag") == "order" and protobuf.get("int32") == "order")
                check(f"{test_id}: Kafka Protobuf schema ID length coercion", protobuf.get("object_length") == "order")
                check(f"{test_id}: Kafka Protobuf missing metadata", binary.get("protobuf_metadata_rejected") is True)
                masking = response.get("datamasking", {})
                original_masking = {"users": [{"secret": "one"}, {"secret": None}], "public": "visible"}
                check(f"{test_id}: Data Masking field rule precedence", masking.get("masked") == {"users": [{"secret": "[redacted]"}, {"secret": None}], "public": "visible"})
                check(f"{test_id}: Data Masking input ownership", masking.get("original") == original_masking)
                check(f"{test_id}: Data Masking provider round trip", masking.get("restored") == original_masking)
                check(f"{test_id}: Data Masking provider invocation context", masking.get("provider_calls") == 4)
                check(f"{test_id}: Data Masking missing field diagnostic", masking.get("warnings") == ["Field not found: 'missing'"])
                check(f"{test_id}: Data Masking missing provider", masking.get("missing_provider") is True)
                kms_masking = masking.get("kms", {})
                kms_original = {"secret": "αβ 😀", "public": "visible"}
                check(f"{test_id}: KMS masking authenticated round trip and ownership", kms_masking.get("restored") == kms_original and kms_masking.get("original") == kms_original)
                check(f"{test_id}: KMS masking fresh uncached materials", kms_masking.get("fresh") is True and kms_masking.get("calls") == {"GenerateDataKey": 2, "Decrypt": 2})
                check(f"{test_id}: KMS masking authenticated context rejection", kms_masking.get("context_mismatch") is True)
                check(f"{test_id}: KMS masking shared SDK identity", kms_masking.get("identity") is True)
                check(f"{test_id}: Data Masking regex named Unicode capture", masking.get("regex") == {"secret": "[αβ] [one]"})
                check(f"{test_id}: Data Masking regex global reset", masking.get("regex_index") == 0)
                check(f"{test_id}: shared regex sticky UTF-16 offset", masking.get("sticky") == "#ab" and masking.get("sticky_index") == 2)
                for kind in ("v1", "v2", "alb", "url"):
                    result = routed.get(kind, {})
                    check(f"{test_id}: HTTP {kind} adapter status", result.get("statusCode") == 201 and (kind != "alb" or result.get("statusDescription") == "201 Created"))
                    body = json.loads(result.get("body", "null"))
                    check(f"{test_id}: HTTP {kind} Parser and Validation composition", body == {"status": "ok", "id": "body-a", "path": "order-a", "request_id": response.get("request_id"), "service": "orders"})
                    check(f"{test_id}: HTTP {kind} request middleware", result.get("headers", {}).get("x-request-id") == response.get("request_id"))
                    cors_headers = result.get("headers", {})
                    check(f"{test_id}: HTTP {kind} CORS response", cors_headers.get("access-control-allow-origin") == "https://app.example.test" and cors_headers.get("access-control-allow-credentials") == "true" and cors_headers.get("vary") == "Origin")
                    preflight = routed.get(f"{kind}_preflight", {})
                    check(f"{test_id}: HTTP {kind} CORS preflight", preflight.get("statusCode") == 204 and preflight.get("body") == "" and preflight.get("headers", {}).get("access-control-max-age") == "600" and "x-request-id" not in preflight.get("headers", {}))
                    denied = routed.get(f"{kind}_denied_preflight", {})
                    check(f"{test_id}: HTTP {kind} invalid preflight falls through", denied.get("statusCode") == 404 and "access-control-allow-origin" not in denied.get("headers", {}))
                    for encoding, decoder in (("gzip", gzip.decompress), ("deflate", zlib.decompress)):
                        compressed = routed.get(f"{kind}_{encoding}", {})
                        compressed_headers = compressed.get("headers", {})
                        wire = base64.b64decode(compressed.get("body", ""))
                        check(f"{test_id}: HTTP {kind} {encoding} payload", compressed.get("isBase64Encoded") is True and compressed_headers.get("content-encoding") == encoding and decoder(wire).decode("utf-8") == "hello λ 世界")
                        check(f"{test_id}: HTTP {kind} {encoding} composed headers", compressed_headers.get("content-length") == str(len(wire)) and compressed_headers.get("vary") == "Origin" and compressed_headers.get("x-request-id") == response.get("request_id"))
                    for mode in ("identity", "no_transform"):
                        plain = routed.get(f"{kind}_{mode}", {})
                        check(f"{test_id}: HTTP {kind} {mode} skips compression", plain.get("isBase64Encoded") is False and plain.get("body") == "hello λ 世界" and "content-encoding" not in plain.get("headers", {}))
                for name, source in (("parser_rejected", "body"), ("schema_rejected", "path")):
                    failure = routed.get(name, {})
                    issues = json.loads(failure.get("body", "{}")).get("details", {}).get("issues", [])
                    check(f"{test_id}: HTTP {source} validation rejects before handler", failure.get("statusCode") == 422 and len(issues) == 1 and issues[0].get("path") == [source, "id"])
                check(f"{test_id}: HTTP unmatched route", routed.get("missing", {}).get("statusCode") == 404)
                check(f"{test_id}: HTTP unsupported method", routed.get("unsupported", {}).get("statusCode") == 405 and routed.get("unsupported", {}).get("body") == "")
                check(f"{test_id}: HTTP binary response", routed.get("binary", {}).get("body") == "AAH/" and routed.get("binary", {}).get("isBase64Encoded") is True)
                check(f"{test_id}: HTTP business invocation count", routed.get("handler_calls") == 4)
                expected_parameters = {provider: {"enabled": True} for provider in ("ssm", "secrets", "dynamodb", "appconfig", "agent")}
                check(f"{test_id}: Parameters values", response.get("parameters") == expected_parameters)
                check(f"{test_id}: Metadata values", response.get("metadata") == {"AvailabilityZoneID": "ape1-az1", "future": {"enabled": True}})
                check(f"{test_id}: JMESPath decoded projection", response.get("query") == ["enabled"])
                for source, identifiers in {"sqs": ["2"], "fifo": ["2", "3", "4"], "fifo_groups": ["2", "4"], "kinesis": ["90071992547409930002"], "dynamodb": ["90071992547409930002"]}.items():
                    check(f"{test_id}: Batch {source} response", response.get("batch", {}).get(source) == {"batchItemFailures": [{"itemIdentifier": value} for value in identifiers]})
                idem = response.get("idempotency", {})
                check(f"{test_id}: Idempotency warm replay", idem.get("warm_executions") == 1 and idem.get("warm_response") == 1)
                check(f"{test_id}: Idempotency duplicate suppression", idem.get("record_executions") == 3)
                check(f"{test_id}: Idempotency Batch partial failure", idem.get("batch") == {"batchItemFailures": [{"itemIdentifier": "3"}]})
                check(f"{test_id}: Idempotency failed-record retry", idem.get("retry") == {"batchItemFailures": []})
                check(f"{test_id}: Idempotency payload validation", idem.get("validation_rejected") is True)
                check(f"{test_id}: Parser rejects invalid orders before Idempotency", idem.get("parser_rejected") is True)
                parsed = response.get("parser", {})
                check(f"{test_id}: Parser typed SQS envelope", parsed.get("orders") == [{"id": "a", "amount": 1}])
                safe_paths = [issue.get("path") for issue in parsed.get("safe_issues", [])]
                first_paths = [issue.get("path") for issue in parsed.get("first_issues", [])]
                check(f"{test_id}: Parser safe aggregation and first-error mode", safe_paths == [["Records", 0, "body", "id"], ["Records", 0, "body", "amount"], ["Records", 1, "body", "amount"]] and first_paths == safe_paths[:2])
                check(f"{test_id}: Parser typed EventBridge detail", parsed.get("detail") == {"id": "detail", "amount": 2})
                streams = parsed.get("streams", {})
                for envelope in ("sns", "snssqs", "kinesis", "firehose", "cloudwatch"):
                    check(f"{test_id}: Parser {envelope} envelope", streams.get(envelope) == [{"id": "stream", "amount": 1}])
                check(f"{test_id}: Parser DynamoDB images", streams.get("dynamodb") == [{"NewImage": {"id": "stream", "amount": 1}}])
                check(f"{test_id}: Parser DynamoDB large integer", streams.get("large_integer") == "9007199254740993")
                parsed_http = parsed.get("http", {})
                for envelope in ("apigateway", "apigatewayv2", "lambdaurl", "lattice", "latticev2"):
                    check(f"{test_id}: Parser {envelope} HTTP envelope", parsed_http.get(envelope) == {"id": "http", "amount": 1})
                check(f"{test_id}: Parser ALB multi-value headers", parsed_http.get("alb") == {"accept": ["json"]})
                check(f"{test_id}: Parser HTTP metadata and body issues", [issue.get("path") for issue in parsed_http.get("invalid_issues", [])] == [["method"], ["body", "id"], ["body", "amount"]])
                services = parsed.get("services", {})
                check(f"{test_id}: Parser Kafka ordered typed records", services.get("kafka") == [{"id": "z", "amount": 1}, {"id": "a", "amount": 2}])
                check(f"{test_id}: Parser Kafka safe issue paths", [issue.get("path") for issue in services.get("kafka_issues", [])] == [["records", "z-0", "id"], ["records", "z-0", "amount"], ["records", "a-0", "amount"]])
                cfn = services.get("cloudformation", {})
                check(f"{test_id}: Parser CloudFormation update fields", cfn.get("OldResourceProperties") == {"old": True} and cfn.get("RequestType") == "Update" and "PhysicalResourceId" not in cfn)
                check(f"{test_id}: Parser Transfer IPv4", services.get("transfer", {}).get("sourceIp") == "127.0.0.1")
                check(f"{test_id}: Parser Connect empty profiles", services.get("connect", {}).get("Items") == {"CustomerProfiles": []})
                ses_records = services.get("ses", {}).get("Records", [])
                check(f"{test_id}: Parser SES receipt", len(ses_records) == 1 and ses_records[0].get("ses", {}).get("receipt", {}).get("processingTimeMillis") == 1)
                s3_records = services.get("s3", {}).get("Records", [])
                check(f"{test_id}: Parser S3 key and size preservation", len(s3_records) == 1 and s3_records[0].get("s3", {}).get("object") == {"key": "a+b%2Fc", "size": -1})
                object_lambda = services.get("object_lambda", {})
                check(f"{test_id}: Parser Object Lambda transformations", object_lambda.get("configuration", {}).get("payload") == {} and object_lambda.get("userIdentity", {}).get("sessionContext", {}).get("attributes", {}).get("mfaAuthenticated") is False)
                identity = parsed.get("identity", {})
                check(f"{test_id}: Parser AppSync typed arguments", identity.get("resolver") == {"id": "resolver", "amount": 1})
                check(f"{test_id}: Parser AppSync batch", identity.get("batch_count") == 2)
                publish = identity.get("publish", {})
                check(f"{test_id}: Parser AppSync publish snapshot", publish.get("info", {}).get("operation") == "PUBLISH" and publish.get("events") == [{"payload": {"id": "order"}, "id": "1"}] and publish.get("stash") == {})
                subscribe = identity.get("subscribe", {})
                check(f"{test_id}: Parser AppSync subscribe", subscribe.get("info", {}).get("operation") == "SUBSCRIBE" and "events" in subscribe and subscribe["events"] is None)
                signup = identity.get("signup", {})
                check(f"{test_id}: Parser Cognito typed response", signup.get("request", {}).get("userAttributes", {}).get("email") == "synthetic@example.test" and signup.get("response", {}).get("autoConfirmUser") is True)
                check(f"{test_id}: Parser Cognito input rejection before handler", identity.get("signup_calls") == 1 and [issue.get("path") for issue in identity.get("signup_issues", [])] == [["response", "autoConfirmUser"]])
                token_v1, token_v3 = identity.get("token_v1", {}), identity.get("token_v3", {})
                check(f"{test_id}: Parser Cognito token variants", "scopes" not in token_v1.get("request", {}) and token_v3.get("request", {}).get("scopes") == ["read"] and token_v3.get("response") == {})
                check(f"{test_id}: Parser Cognito empty challenge session", [issue.get("path") for issue in identity.get("challenge_issues", [])] == [["request", "session"]])
                validated = response.get("validation", {})
                check(f"{test_id}: Validation typed response and outbound envelope bypass", validated.get("value") == "ok")
                check(f"{test_id}: Validation inbound snapshot", validated.get("isolated") is True)
                check(f"{test_id}: Validation inbound error stage", validated.get("inbound_error") == "Inbound schema validation failed")
                check(f"{test_id}: Validation formats and external references", sorted((issue.get("instancePath"), issue.get("keyword")) for issue in validated.get("issues", [])) == [("/amount", "minimum"), ("/id", "format")])
                check(f"{test_id}: Validation rejects before business handler", validated.get("calls") == 1)
                check(f"{test_id}: Validation outbound error stage", validated.get("outbound_error") == "Outbound schema validation failed")
                check(f"{test_id}: Validation preserves business result and error", validated.get("business_preserved") is True)
                check(f"{test_id}: Validation cancellation", validated.get("cancelled") is True)
                regex = validated.get("regex", {})
                check(f"{test_id}: Validation Unicode named backreference", regex.get("matched") == "αβαβ")
                check(f"{test_id}: Validation original regex diagnostics", regex.get("pattern") == r"^(?<word>\p{Script=Greek}+)\k<word>$")
                check(f"{test_id}: Validation encoded pattern property path", regex.get("property_issue") == {"instancePath": "/α", "schemaPath": "#/patternProperties/%5E%5Cp%7BLetter%7D%2B%24/type", "keyword": "type", "message": "must be integer", "params": {"type": "integer"}})
                check(f"{test_id}: Validation rejects non-JavaScript regex syntax", regex.get("invalid_syntax") is True)
                check(f"{test_id}: Validation regex resource failure remains operational", regex.get("resource_error") is True)
                applicators = validated.get("applicators", {})
                nested = applicators.get("nested", [])
                check(f"{test_id}: Validation nested ordering and conditional summary", [item.get("keyword") for item in nested] == ["dependencies", "dependencies", "if", "pattern", "propertyNames", "additionalItems", "type"] and nested[2].get("params") == {"failingKeyword": "then"})
                check(f"{test_id}: Validation complete dependency diagnostics", len(nested) == 7 and [item.get("params") for item in nested[:2]] == [{"property": "card", "missingProperty": name, "depsCount": 2, "deps": "zip, name"} for name in ("zip", "name")])
                check(f"{test_id}: Validation property-name scope and tuple limit", len(nested) == 7 and [item.get("instancePath") for item in nested] == ["/zebra"] * 5 + ["/alpha", "/alpha/0"] and nested[3].get("propertyName") == "BAD/~" and nested[4].get("params") == {"propertyName": "BAD/~"} and nested[5].get("params") == {"limit": 1})
                one_of = applicators.get("one_of", [])
                check(f"{test_id}: Validation oneOf retains failed branch diagnostics", len(one_of) == 2 and one_of[0].get("schemaPath") == "#/oneOf/0/type" and one_of[1].get("params") == {"passingSchemas": [1, 2]})
                references = validated.get("references", {})
                check(f"{test_id}: Validation mixed-type rule order", [item.get("keyword") for item in references.get("type_order", [])] == ["minLength", "type"])
                nullable = references.get("nullable", [])
                check(f"{test_id}: Validation nullable array diagnostics", len(nullable) == 1 and nullable[0].get("params") == {"type": ["string", "null"]} and nullable[0].get("message") == "must be string,null")
                check(f"{test_id}: Validation nested resource identity and literal references", [(item.get("instancePath"), item.get("schemaPath")) for item in references.get("nested_id", [])] == [("/zebra", "value/type"), ("/alpha", "https://example.test/value/type")])
                recursive = references.get("recursive", [])
                check(f"{test_id}: Validation recursive compiled reference scope", len(recursive) == 1 and recursive[0].get("instancePath") == "/child/child" and recursive[0].get("schemaPath") == "#/type")
                check(f"{test_id}: Validation strict UTF-16 overlap rejection", references.get("strict_utf16_rejected") is True)
                check(f"{test_id}: Validation strict overlap preserves Unicode payload matching", references.get("strict_unicode") == [{"instancePath": "/😀", "schemaPath": "#/patternProperties/%5E.%24/type", "keyword": "type", "params": {"type": "integer"}, "message": "must be integer"}])
                check(f"{test_id}: Validation strict property escape identity", references.get("strict_property_escape") == [{"instancePath": "/a", "schemaPath": "#/patternProperties/%5E%5Cp%7BLetter%7D%2B%24/type", "keyword": "type", "params": {"type": "integer"}, "message": "must be integer"}])
                setup = references.get("setup", {})
                check(f"{test_id}: Validation unused compilation checks deferred", setup.get("unused") == "value")
                check(f"{test_id}: Validation referenced defs retain Draft 7 setup behavior", setup.get("defs") == "value")
                check(f"{test_id}: Validation unused pattern matcher skipped", setup.get("pattern") == {"a": 1})
                check(f"{test_id}: Validation referenced unknown keyword rejected", setup.get("referenced_rejected") is True)
                check(f"{test_id}: Validation unused structural error rejected", setup.get("structure_rejected") is True)
                check(f"{test_id}: Validation additional property matcher required", setup.get("pattern_rejected") is True)
                for key, label in [
                    ("shape_scalar", "unvalidated scalar fragment"),
                    ("shape_ignored", "ignored condition"),
                    ("shape_unused", "unused malformed keyword"),
                    ("shape_annotations", "registered annotations"),
                ]:
                    check(f"{test_id}: Validation {label}", setup.get(key) == "value")
                check(f"{test_id}: Validation reachable malformed keyword rejected", setup.get("shape_rejected") is True)
                check(f"{test_id}: Validation unselected branch compiled", setup.get("branch_rejected") is True)
                check(f"{test_id}: Validation non-string dependency diagnostic", references.get("shape_dependency") == [{"instancePath": "", "schemaPath": "#/$defs/value/dependencies", "keyword": "dependencies", "params": {"property": "value", "missingProperty": True, "depsCount": 1, "deps": "true"}, "message": "must have property true when property value is present"}])
                check(f"{test_id}: Validation fractional string limits", references.get("keyword_size") == [
                    {"instancePath": "", "schemaPath": "#/$defs/value/maxLength", "keyword": "maxLength", "params": {"limit": -1}, "message": "must NOT have more than -1 characters"},
                    {"instancePath": "", "schemaPath": "#/$defs/value/minLength", "keyword": "minLength", "params": {"limit": 1.5}, "message": "must NOT have fewer than 1.5 characters"},
                ])
                check(f"{test_id}: Validation empty combinator summaries", references.get("keyword_empty") == [
                    {"instancePath": "", "schemaPath": "#/$defs/value/anyOf", "keyword": "anyOf", "params": {}, "message": "must match a schema in anyOf"},
                    {"instancePath": "", "schemaPath": "#/$defs/value/oneOf", "keyword": "oneOf", "params": {"passingSchemas": None}, "message": "must match exactly one schema in oneOf"},
                ])
                check(f"{test_id}: Validation non-string required parameters", references.get("keyword_required") == [
                    {"instancePath": "", "schemaPath": "#/$defs/value/required", "keyword": "required", "params": {"missingProperty": 1}, "message": "must have required property '1'"},
                    {"instancePath": "", "schemaPath": "#/$defs/value/required", "keyword": "required", "params": {"missingProperty": {"a": 1}}, "message": "must have required property '[object Object]'"},
                ])
                for label, key, path, divisor in [
                    ("floating division", "keyword_decimal", "#/multipleOf", 0.1),
                    ("zero divisor", "keyword_zero", "#/$defs/value/multipleOf", 0),
                    ("exponential quotient", "keyword_exponent", "#/multipleOf", 1),
                ]:
                    check(f"{test_id}: Validation multipleOf {label}", references.get(key) == [{"instancePath": "", "schemaPath": path, "keyword": "multipleOf", "params": {"multipleOf": divisor}, "message": f"must be multiple of {divisor}"}])
                check(f"{test_id}: Validation fractional array limits", references.get("keyword_array") == [
                    {"instancePath": "", "schemaPath": "#/$defs/value/maxItems", "keyword": "maxItems", "params": {"limit": -1}, "message": "must NOT have more than -1 items"},
                    {"instancePath": "", "schemaPath": "#/$defs/value/minItems", "keyword": "minItems", "params": {"limit": 1.5}, "message": "must NOT have fewer than 1.5 items"},
                ])
                graphs = validated.get("graphs", {})
                for key, label in [("ignored", "ignored missing reference"), ("dialect", "root dialect alias"), ("ordered", "ordered external registration")]:
                    check(f"{test_id}: Validation {label}", graphs.get(key) == "value")
                check(f"{test_id}: Validation active missing reference rejected", graphs.get("active_missing_rejected") is True)
                check(f"{test_id}: Validation reference into ignored condition", graphs.get("active") == [{"instancePath": "/value", "schemaPath": "#/if/type", "keyword": "type", "params": {"type": "number"}, "message": "must be number"}])
                check(f"{test_id}: Validation nested external resource identity", graphs.get("external") == [{"instancePath": "", "schemaPath": "https://example.test/nested/number/type", "keyword": "type", "params": {"type": "number"}, "message": "must be number"}])
                parser_errors = parsed.get("errors", {})
                check(f"{test_id}: Parser sole union branch diagnostic", parser_errors.get("sole") == [{"code": "custom", "message": "positive required"}])
                tree = parser_errors.get("tree", [])
                expected_tree = [{"code": "invalid_union", "message": "Invalid input", "path": ["payload"], "errors": [[{"code": "invalid_union", "message": "Invalid input", "path": ["value"], "errors": [[{"code": "invalid_type", "expected": "string", "message": "Invalid input: expected string, received boolean"}], [{"code": "invalid_type", "expected": "number", "message": "Invalid input: expected number, received boolean"}]]}], [{"code": "invalid_type", "expected": "array", "message": "Invalid input: expected array, received object"}]]}]
                check(f"{test_id}: Parser nested union error tree", tree == expected_tree)
                check(f"{test_id}: Parser continued refinements", [issue.get("message") for issue in parser_errors.get("refinements", [])] == ["positive required", "even required"])
                check(f"{test_id}: Parser array diagnostic order", [issue.get("code") for issue in parser_errors.get("array", [])] == ["invalid_type", "too_small"])
                nested_paths = [["event", "Records", 0, "body", "id"], ["event", "Records", 0, "body", "amount"], ["event", "Records", 1, "body", "amount"], ["label"]]
                check(f"{test_id}: Parser nested safe aggregation", [issue.get("path") for issue in parser_errors.get("nested_safe", [])] == nested_paths)
                check(f"{test_id}: Parser nested ordinary aggregation", [issue.get("path") for issue in parser_errors.get("nested_ordinary", [])] == nested_paths[:2] + [["label"]])
                cached = response.get("cache", {})
                check(f"{test_id}: Cache warm replay", cached.get("warm_executions") == 1 and cached.get("warm_response") == 1)
                check(f"{test_id}: Cache concurrent acquisition", cached.get("concurrent_executions") == 1 and cached.get("contended_calls") == 32)
                check(f"{test_id}: Cache orphan recovery", cached.get("recovered") is True)
                check(f"{test_id}: Cache validation retained", cached.get("validation_rejected") is True)
                check(f"{test_id}: Cache TypeScript to Go", cached.get("typescript_response") == {"owner": "typescript", "nested": [1, None, True]})
        bridge_result = json.loads(run("node", str(bridge), "verify", cache))
        check("Cache: Go to TypeScript replay", bridge_result.get("passed") is True and bridge_result.get("calls") == 0)
        report["cache_interop"] = {"typescript_seed": seed, "go_to_typescript": bridge_result}
        snapshot = request(capture, "http://capture:4318/snapshot")
        check("OTLP snapshot available", snapshot["_http_status"] == 200)
        signed_requests = [r for r in snapshot["requests"] if r["path"] == "/fixture.txt"]
        check("Signer signatures and OTel transport composition", len(signed_requests) == 3 and all(r.get("signature_valid") == "true" and r.get("traceparent") for r in signed_requests))
        (OUT / "otlp.json").write_text(json.dumps(snapshot, indent=2) + "\n", encoding="utf-8")
        # docker logs sends application stderr separately; retain both streams.
        raw = subprocess.run(["docker", "logs", function], capture_output=True, text=True, check=True)
        logs = raw.stdout + "\n" + raw.stderr
        (OUT / "lambda.log").write_text(logs, encoding="utf-8")
        records = []
        emf = []
        for line in logs.splitlines():
            try:
                record = json.loads(line)
                if isinstance(record, dict) and "level" in record:
                    records.append(record)
                elif isinstance(record, dict) and "_aws" in record:
                    emf.append(record)
            except json.JSONDecodeError:
                pass
        spans = [span for batch in snapshot["batches"] for resource in batch.get("resourceSpans", []) for scope in resource.get("scopeSpans", []) for span in scope.get("spans", [])]
        for index, invocation in enumerate(report["invocations"]):
            event = invocation["event"]
            mode, test_id = event["mode"], event["id"]
            if event.get("traceparent"):
                identifier = event["traceparent"].split("-")[1]
                root = f"1-{identifier[:8]}-{identifier[8:]}"
            else:
                root = event["trace_header"].split(";")[0].removeprefix("Root=")
            trace_id = root.replace("-", "")[1:]
            trace_spans = [s for s in spans if base64.b64decode(s["traceId"]).hex() == trace_id]
            invocation_logs = [r for r in records if r.get("test_id") == test_id]
            invocation_metrics = [r for r in emf if r.get("test_id") == test_id]
            check(f"{test_id}: isolated EMF metric", len(invocation_metrics) == 1 and invocation_metrics[0].get("Invocations") == 1)
            if invocation_metrics:
                directive = invocation_metrics[0]["_aws"]["CloudWatchMetrics"][0]
                check(f"{test_id}: EMF namespace and dimensions", directive["Namespace"] == "PowertoolsIntegration" and directive["Dimensions"] == [["service"]])
            starts = [r for r in invocation_logs if r.get("message") == "integration start"]
            check(f"{test_id}: one start log", len(starts) == 1)
            if starts:
                check(f"{test_id}: cold start", starts[0].get("cold_start") == (index == 0))
                check(f"{test_id}: temporary state", ("temporary_marker" in starts[0]) == (index == 0))
                check(f"{test_id}: log correlation", starts[0].get("xray_trace_id") == root)
                check(f"{test_id}: OTel log correlation", starts[0].get("trace_id") == trace_id and len(starts[0].get("span_id", "")) == 16)
                check(f"{test_id}: JMESPath Logger correlation", starts[0].get("correlation_id") == "query:" + test_id)
            check(f"{test_id}: buffer lifecycle", any(r.get("message") == "buffered diagnostic" for r in invocation_logs) == (mode in ("error", "panic")))
            if mode in ("success", "unsampled"):
                http_metrics = [r for r in emf if r.get("http_invocation_id") == invocation["response"].get("request_id")]
                check(f"{test_id}: HTTP middleware EMF isolation", len(http_metrics) == 32 and all("Invocations" not in r and "test_id" not in r and isinstance(r.get("latency"), (int, float)) and r["latency"] >= 0 for r in http_metrics))
                check(f"{test_id}: HTTP middleware EMF route dimensions", len(http_metrics) == 32 and all(r.get("route") in ("POST /orders/:id", "GET /compressed/gzip", "GET /compressed/deflate", "GET /binary", "NOT_FOUND") and r["_aws"]["CloudWatchMetrics"][0]["Dimensions"] == [["route", "service"]] for r in http_metrics))
                check(f"{test_id}: HTTP middleware EMF status", sum(r.get("statusCode") == "201" for r in http_metrics) == 4 and sum(r.get("statusCode") == "204" for r in http_metrics) == 4 and sum(r.get("error") == 1 for r in http_metrics) == 7 and all(r.get("fault") == 0 for r in http_metrics))
                http_logs = [r for r in invocation_logs if r.get("message") == "http route"]
                check(f"{test_id}: HTTP scoped route logs", len(http_logs) == 4 and all(r.get("trace_id") == trace_id and r.get("http_request_id") == invocation["response"].get("request_id") and r.get("http_route") == "POST /orders/:id" for r in http_logs))
                idem_logs = [r for r in invocation_logs if r.get("message") == "idempotent record"]
                check(f"{test_id}: Idempotency execution logs", len(idem_logs) == 3)
                check(f"{test_id}: Idempotency scoped correlation", len(idem_logs) == 3 and all(r.get("trace_id") == trace_id and r.get("correlation_id") == "query:" + test_id and r.get("function_request_id") == invocation["response"].get("request_id") for r in idem_logs))
            if mode == "unsampled":
                check(f"{test_id}: no exported spans", not trace_spans)
                continue
            handler = [s for s in trace_spans if s["name"] == "## integration"]
            business = [s for s in trace_spans if s["name"] == "### business"]
            check(f"{test_id}: handler and business spans", len(handler) == 1 and len(business) == 1)
            if handler and business:
                check(f"{test_id}: parent relationship", business[0].get("parentSpanId") == handler[0]["spanId"])
                check(f"{test_id}: supplied parent", base64.b64decode(handler[0]["parentSpanId"]).hex() == "1234567890abcdef")
                check(f"{test_id}: annotation", attributes(business[0]).get("TestID") == test_id)
                check(f"{test_id}: error status", (handler[0].get("status", {}).get("code") == "STATUS_CODE_ERROR") == (mode in ("error", "panic")))
            if mode == "success":
                http_spans = [s for s in trace_spans if s["name"] == "### http-route"]
                http_middleware_spans = [s for s in trace_spans if "http.request.method" in attributes(s) and s.get("kind") == "SPAN_KIND_INTERNAL"]
                route_parents = {s["spanId"] for s in http_middleware_spans if s["name"] == "POST /orders/order-a"}
                check(f"{test_id}: HTTP OTel route spans", len(http_spans) == 4 and len(business) == 1 and all(s.get("parentSpanId") in route_parents for s in http_spans))
                check(f"{test_id}: HTTP OTel middleware lifecycle", len(http_middleware_spans) == 32 and len(business) == 1 and all(s.get("parentSpanId") == business[0]["spanId"] for s in http_middleware_spans))
                check(f"{test_id}: HTTP OTel middleware attributes", len(http_middleware_spans) == 32 and all("http.response.status_code" in attributes(s) and "powertools.metadata.powertools-integration-otel.http" in attributes(s) and "?" not in attributes(s).get("url.full", "") for s in http_middleware_spans))
                idem_spans = [s for s in trace_spans if s["name"] == "### idempotent-record"]
                check(f"{test_id}: Idempotency record spans", len(idem_spans) == 3 and len(business) == 1 and all(s.get("parentSpanId") == business[0]["spanId"] for s in idem_spans) and sum(s.get("status", {}).get("code") == "STATUS_CODE_ERROR" for s in idem_spans) == 1)
                check(f"{test_id}: downstream SDK and HTTP", any(s["name"] == "DynamoDB.GetItem" for s in trace_spans) and any(s.get("kind") == "SPAN_KIND_CLIENT" and s["name"] != "DynamoDB.GetItem" for s in trace_spans))
        first, second = report["invocations"][:2]
        check("warm runtime reuse", first["response"].get("instance") == second["response"].get("instance") and second["response"].get("invocation") == 2)
        requests = snapshot["requests"]
        idem_fixture = snapshot.get("idempotency", {})
        idem_requests = idem_fixture.get("requests", [])
        # Preserve the original seven-record/28-request probe expectations. Each
        # successful Kafka probe adds one completed record, one acquisition, one
        # completion, and a duplicate acquisition that replays the stored result.
        kafka_invocations = sum(item["event"]["mode"] in ("success", "unsampled") for item in report["invocations"])
        expected_idem_requests = 28 + 3 * kafka_invocations
        check("Idempotency: persisted completed records", idem_fixture.get("record_count") == 7 + kafka_invocations)
        check("Idempotency: acquisition, completion, and cleanup requests", len(idem_requests) == expected_idem_requests and {operation: sum(r["operation"] == operation for r in idem_requests) for operation in ("PutItem", "UpdateItem", "DeleteItem")} == {"PutItem": 18 + 2 * kafka_invocations, "UpdateItem": 7 + kafka_invocations, "DeleteItem": 3})
        # Tracer installs the shared marker first on this client. Standalone
        # adapter tests separately verify the idempotency marker without Tracer.
        check("Idempotency: SDK identity and propagation", len(idem_requests) == expected_idem_requests and all(r.get("traceparent") and r.get("user_agent", "").count("PT/") == 1 and "PT/tracer/" in r.get("user_agent", "") for r in idem_requests))
        check("downstream propagation", len(requests) == 20 and all(r["traceparent"] for r in requests))
        metadata_requests = [r for r in requests if r["path"] == "/2026-01-15/metadata/execution-environment"]
        check("Metadata: warm cache reuse and explicit clear", len(metadata_requests) == 2)
        check("Metadata: authenticated requests", all(r["metadata_authenticated"] == "true" for r in metadata_requests))
        sdk_requests = [r for r in requests if r["target"] or r["path"] in ("/configuration", "/configurationsessions")]
        check("Commons: one Powertools SDK user-agent marker", all(r["user_agent"].count("PT/") == 1 and "aws-sdk-go-v2/" in r["user_agent"] for r in sdk_requests))
        for target in ("AmazonSSM.GetParameter", "secretsmanager.GetSecretValue"):
            check(f"Parameters {target}: cache reuse and forced refresh", sum(r["target"] == target for r in requests) == 2)
        check("Parameters DynamoDB: cache reuse and forced refresh", sum(r["target"] == "DynamoDB_20120810.GetItem" for r in requests) == 5)
        check("Parameters AppConfig: one session", sum(r["path"] == "/configurationsessions" for r in requests) == 1)
        check("Parameters AppConfig: token rotation and empty update", sum(r["path"] == "/configuration" for r in requests) == 2)
        check("Parameters Agent: no additional cache", sum(r["path"] == "/applications/local/environments/test/configurations/flags" for r in requests) == 3)
        check("no instrumentation failures", "LOGGER_FAILURE" not in logs and "TRACER_FAILURE" not in logs)
        check("original panic preserved in runtime log", '"errorMessage":"intentional integration panic"' in logs)
        cold_metrics = [r for r in emf if "ColdStart" in r]
        check("one isolated cold-start metric", len(cold_metrics) == 1 and cold_metrics[0]["ColdStart"] == 1 and "test_id" not in cold_metrics[0] and cold_metrics[0].get("function_name") == "powertools-local")
        expected_http_documents = 32 * sum(item["event"]["mode"] in ("success", "unsampled") for item in report["invocations"])
        check("invocation and HTTP EMF documents", len(emf) == 6 + expected_http_documents)
        check("no metric instrumentation failures", "METRICS_FAILURE" not in logs)
        report["completed"] = True
    finally:
        cleanup_errors = []
        for name in reversed(containers):
            inspection = subprocess.run(["docker", "inspect", name], capture_output=True, text=True)
            (OUT / f"{name}-inspect.json").write_text(inspection.stdout, encoding="utf-8")
            logs = subprocess.run(["docker", "logs", name], capture_output=True, text=True)
            (OUT / f"{name}.log").write_text(logs.stdout + "\n" + logs.stderr, encoding="utf-8")
            try:
                run("docker", "rm", "-f", name)
            except RuntimeError as error:
                cleanup_errors.append(str(error))
        try:
            run("docker", "network", "rm", network)
        except RuntimeError as error:
            cleanup_errors.append(str(error))
        report["cleanup_errors"] = cleanup_errors
        report["passed"] = report["completed"] and all(c["passed"] for c in report["checks"]) and not cleanup_errors
        (OUT / "report.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    failed = [c["name"] for c in report["checks"] if not c["passed"]]
    print(f"{len(report['checks']) - len(failed)}/{len(report['checks'])} checks passed; evidence: {OUT}", flush=True)
    if not report["passed"]:
        raise RuntimeError(f"Failed checks: {failed}; cleanup errors: {report['cleanup_errors']}")
    print(run(sys.executable, str(ROOT / "integration/local/stream_run.py"), "--reuse-builds"), flush=True)


if __name__ == "__main__":
    main()
