# AWS acceptance results

Region: **ap-east-1 (Hong Kong)**. Account, profile, resource names and trace IDs are omitted from this public summary.

Acceptance checked at `2026-09-12T20:29:21.120793+00:00` UTC. Temporary resources were removed at `2026-09-12T20:31:18.874657+00:00` UTC. The run crossed into September 13, 2026 in Asia/Shanghai.

**206/206 scoped acceptance checks passed across 20 real Lambda invocations.** Both architectures ran native Go executables on `provided.al2023`. Application and custom collector builds used `CGO_ENABLED=0`.

| Backend | Architecture | Invocations | Case/instance checks passed |
| --- | --- | --- | --- |
| otel | amd64 | 5 | 52 |
| otel | arm64 | 5 | 52 |
| xray | amd64 | 5 | 49 |
| xray | arm64 | 5 | 49 |

The table counts checks associated with individual cases and warm instance reuse; four additional checks verified the absence of Logger/Tracer failure diagnostics, bringing the total to 206.

## What passed

- Native Lambda request context and request IDs; explicitly propagated sampled/unsampled parents through the real Invoke API.
- Cold-start and warm-invocation flags, reuse of the same execution environment, temporary attribute isolation, and OTel log/span correlation.
- Buffered DEBUG logs discarded on success and flushed on handler error/panic.
- Original error/panic messages, closed handler and business spans, error flags, indexed TestID annotations, and metadata delivery.
- Actual DynamoDB GetItem calls to the dedicated table and signed HTTP GET calls to the private S3 fixture.
- OTel export through a Lambda collector extension into X-Ray, native X-Ray SDK subsegments, and absence of application traces for unsampled cases.

## Findings and limitations

The initial stock community collector package did not register `awsxrayexporter`; its initialization failed with the supplied config. The final test uses the official `layer-collector/0.23.0` sources, with a trace-only factory containing the OTLP receiver and AWS X-Ray exporter v0.157.0. Both collector architectures were compiled locally. This avoids relying on an unavailable cross-account Hong Kong layer or changing account-wide telemetry settings.

The test fixture's initial S3 request lacked `X-Amz-Content-Sha256`; its signing was corrected. Initial synthetic parent replacement inside the handler was also replaced by actual Invoke request propagation. The library implementation required no behavior changes during this cloud run.

OTel records the DynamoDB operation as `DynamoDB/GetItem`, while the SDK backend records `GetItem`. The OTel path includes the table name; the selected X-Ray SDK v2 middleware omits it. Acceptance checks recognize the documented backend schemas and verify actual service request IDs. Missing SDK table enrichment remains an open compatibility gap. These tests do not establish complete TypeScript parity.

Hard timeouts, recovery from forced freeze, concurrent stress, Managed Instances, and performance budgets were not tested. The race detector remains excluded by the always-disabled CGO policy.

## Evidence and cleanup

The source-controlled [machine-readable record](AWS_ACCEPTANCE.json) includes test results, exact source/build hashes, and evidence digests. Full invocation responses, CloudWatch records, X-Ray documents, initial failures, the exact CloudFormation template, and cleanup state are retained locally under ignored `dist/integration/`.

The disposable CloudFormation stack and its four functions, two final layers, replaced stock layers, role, log groups, and DynamoDB table were deleted. The private artifact bucket and its objects were removed. X-Ray traces follow AWS retention and cannot be individually deleted; no active compute resources were retained.

Raw trace IDs and resource identifiers remain in ignored local evidence. Function and log-group resources were removed after cleanup.

The reusable harness and its commands are documented in [integration/README.md](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/integration/README.md). Collector provenance: [official release](https://github.com/open-telemetry/opentelemetry-lambda/releases/tag/layer-collector%2F0.23.0).
