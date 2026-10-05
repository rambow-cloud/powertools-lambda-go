"""Run opt-in stateful acceptance against an owned DynamoDB Local backend."""

import argparse
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[2]
IMAGE = "amazon/dynamodb-local@sha256:ff89bd48ff32cd8d9be5fee8873b65b8854dc408f1afe881be6eb00247bc0dab"
REQUIRED = {
    "TestDynamoDBLocalConditionalClaimsAndReplay",
    "TestDynamoDBLocalExpiryLeaseAndCompositeKeys",
    "TestDynamoDBLocalParameters",
}


def command(*args, timeout=30, env=None, cwd=ROOT):
    return subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True,
                          encoding="utf-8", errors="replace", timeout=timeout, check=True).stdout.strip()


def test_checks(stdout, returncode):
    """A zero exit alone cannot turn skipped or missing acceptance into success."""
    events = []
    for line in stdout.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(event, dict):
            events.append(event)
    passed = {item["Test"] for item in events if item.get("Action") == "pass" and item.get("Test")}
    skipped = [item.get("Test", "package") for item in events if item.get("Action") == "skip"]
    failed = [item.get("Test", "package") for item in events if item.get("Action") == "fail"]
    return [
        {"name": "go-exit", "passed": returncode == 0},
        {"name": "no-skips-or-failures", "passed": not skipped and not failed, "skipped": skipped, "failed": failed},
        *({"name": name, "passed": name in passed} for name in sorted(REQUIRED)),
    ]


def write_report(path, report):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8", newline="\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--jar", type=Path, help="Use an existing official DynamoDB Local JAR instead of Docker")
    parser.add_argument("--report", type=Path, default=ROOT / "dist/service-acceptance/dynamodb-local.json")
    args = parser.parse_args()
    output = ROOT / "dist/service-acceptance"
    output.mkdir(parents=True, exist_ok=True)
    execution = {"source_sha": os.environ.get("GITHUB_SHA") or command("git", "rev-parse", "HEAD")}
    for key, name in (("run_id", "GITHUB_RUN_ID"), ("run_attempt", "GITHUB_RUN_ATTEMPT")):
        if os.environ.get(name):
            execution[key] = os.environ[name]
    report = {"execution": execution, "working_tree_dirty": bool(command("git", "status", "--porcelain")),
              "backend": "official DynamoDB Local", "mode": "java" if args.jar else "docker",
              "image": None if args.jar else IMAGE, "cgo_enabled": False,
              "completed": False, "passed": False, "checks": [], "cleanup_errors": []}
    write_report(args.report, report)
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    endpoint = f"http://127.0.0.1:{port}"
    env = os.environ.copy()
    env.update(CGO_ENABLED="0", AWS_EC2_METADATA_DISABLED="true", POWERTOOLS_DYNAMODB_LOCAL_ENDPOINT=endpoint)
    env.pop("GOOS", None)
    env.pop("GOARCH", None)
    name = "powertools-ddb-" + uuid.uuid4().hex
    process, backend_log, docker_attempted = None, None, False
    try:
        if args.jar:
            jar = args.jar.resolve(strict=True)
            java = ["java", "-Xms64m", "-Xmx256m", "-Djava.library.path=" + str(jar.parent / "DynamoDBLocal_lib"), "-jar", str(jar)]
            version = command(*java, "-version", cwd=jar.parent)
            backend_log = (output / "dynamodb-backend.log").open("w", encoding="utf-8")
            process = subprocess.Popen([*java, "-inMemory", "-sharedDb", "-disableTelemetry", "-port", str(port)],
                                       cwd=jar.parent, stdout=backend_log, stderr=subprocess.STDOUT,
                                       creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0)
        else:
            # Only this run's unique container is removed, including failed starts.
            docker_attempted = True
            command("docker", "run", "--detach", "--name", name, "--platform", "linux/amd64",
                    "--publish", f"127.0.0.1:{port}:8000", "--memory", "512m",
                    "--env", "JAVA_TOOL_OPTIONS=-Xms64m -Xmx256m", IMAGE,
                    "-jar", "DynamoDBLocal.jar", "-inMemory", "-sharedDb", "-disableTelemetry", timeout=120)
            version = command("docker", "exec", name, "java", "-jar", "DynamoDBLocal.jar", "-version")
        versions = re.findall(r"\b\d+\.\d+\.\d+\b", version)
        if not versions:
            raise RuntimeError("DynamoDB Local version was not reported")
        report["backend_version"] = versions[-1]
        deadline = time.monotonic() + 30
        while True:
            if process is not None and process.poll() is not None:
                raise RuntimeError("DynamoDB Local exited before becoming ready; see backend log")
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=1):
                    break
            except OSError:
                if time.monotonic() >= deadline:
                    raise RuntimeError("DynamoDB Local startup exceeded 30 seconds") from None
                time.sleep(0.2)
        print(f"Running stateful acceptance against DynamoDB Local {report['backend_version']} ({report['mode']})", flush=True)
        result = subprocess.run(["go", "test", "-json", "-count=1", "-timeout=2m", "./integration/dynamodblocal"],
                                cwd=ROOT, env=env, capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=240)
        (output / "dynamodb-tests.jsonl").write_text(result.stdout, encoding="utf-8")
        (output / "dynamodb-tests.stderr.log").write_text(result.stderr, encoding="utf-8")
        report["checks"] = test_checks(result.stdout, result.returncode)
        report["passed"] = all(item["passed"] for item in report["checks"])
        if not report["passed"]:
            print(result.stdout[-12000:] + result.stderr[-4000:])
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        report["error"] = str(error)
        print(str(error), flush=True)
    finally:
        if process is not None:
            try:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=10)
            except (OSError, subprocess.SubprocessError) as error:
                report["cleanup_errors"].append(str(error))
        if docker_attempted:
            try:
                logs = command("docker", "logs", name, timeout=15)
                (output / "dynamodb-backend.log").write_text(logs, encoding="utf-8")
            except (OSError, subprocess.SubprocessError):
                pass
            try:
                command("docker", "rm", "--force", name, timeout=15)
            except (OSError, subprocess.SubprocessError) as error:
                report["cleanup_errors"].append(str(error))
        if backend_log is not None:
            backend_log.close()
        report["completed"] = True
        report["passed"] = report["passed"] and not report["cleanup_errors"]
        write_report(args.report, report)
    print(f"DynamoDB Local acceptance {'passed' if report['passed'] else 'failed'}; report: {args.report}")
    raise SystemExit(0 if report["passed"] else 1)


if __name__ == "__main__":
    main()
