# Disposable AWS integration tests

Run only when AWS testing is explicitly authorized. `run.py` requires explicit AWS_PROFILE and POWERTOOLS_TEST_ACCOUNT environment variables and fixes all AWS operations to `ap-east-1` (Hong Kong). It checks the expected account before mutation. No production resources are modified.

The fixture builds CGO-disabled Go binaries for Linux amd64 and arm64. The stock OpenTelemetry `layer-collector/0.23.0` package does not register the AWS X-Ray exporter. `build_collector.py` therefore downloads that pinned release's sources, records the source SHA256, registers `awsxrayexporter` v0.157.0, and builds private temporary layers with CGO disabled. All test resources remain in Hong Kong; no cross-region shared layer ARN is required. The release source is [OpenTelemetry Lambda](https://github.com/open-telemetry/opentelemetry-lambda/releases/tag/layer-collector%2F0.23.0).

New uniquely named CloudFormation stacks own two OpenTelemetry Lambda functions, two collector layers, a dedicated DynamoDB table, execution role, and explicit log groups. An associated private S3 bucket holds deployment packages and a non-sensitive fixture object. Functions use `provided.al2023`, Active tracing, and the minimum data permissions required by the test. No public function URL is created. The legacy SDK fixture is retired; historical four-function acceptance evidence is preserved.

New tests invoke OpenTelemetry on both architectures. Cases cover native runtime context, consecutive invocation state, a handled error, an explicitly unsampled parent, and a panic. HTTP instrumentation calls a signed GET against the private fixture object; DynamoDB instrumentation reads the temporary table. Boto3 1.43.93 sends known parent headers on the real Lambda Invoke HTTP request, allowing Lambda to create a consistent runtime context. Historical saved dual-backend evidence can still be evaluated; the maintained handler does not support legacy SDK mode.

Local evidence under ignored `dist/integration/` includes the exact template, hashes, invocation results, CloudWatch events, trace documents, and cleanup state. The runner emits concise progress and uses a bounded poll only for asynchronous CloudFormation and telemetry ingestion. Cleanup removes only this run's recorded stack and private bucket. X-Ray trace data follows AWS retention and cannot be individually deleted. No credentials or signed URLs are written to evidence.

```powershell
$env:CGO_ENABLED = '0'
$env:AWS_PROFILE = 'your-test-profile'
$env:POWERTOOLS_TEST_ACCOUNT = '123456789012' # Replace with your authorized test account.
uv run python integration/run.py prepare
uv run python integration/build_collector.py
uv run python integration/run.py deploy
uv run python integration/run.py refresh
uv run --with boto3==1.43.93 python integration/run.py test
uv run python integration/run.py cleanup
```

The phases allow code and the generated CloudFormation template to be reviewed before deployment. Do not run cleanup against another run's state file. A failed test must still be followed by cleanup after collecting diagnostic evidence.

The completed run is protected against accidental overwrite: archive `dist/integration/` before starting a new run. `refresh` updates the recorded disposable stack after fixture changes and installs the custom collector packages; run it before acceptance when following the preparation flow above. `verify` re-evaluates saved evidence locally without AWS calls. The historical AWS_ACCEPTANCE.json and AWS_VALIDATION.md records are sanitized snapshots. Their one-time generator was removed because it embedded the original test date, retired backend and run-specific conclusions. For a new authorized cloud run, retain raw evidence locally and review a new sanitized summary before publishing it. Account/profile names, stack identifiers and trace IDs must remain only in ignored local evidence.
