---
description: "Compare Powertools for Go with Powertools for AWS Lambda TypeScript v2.35.0 using utility mappings and scoped verification evidence."
---

# TypeScript feature comparison and verification

This project uses **Powertools for AWS Lambda (TypeScript) v2.35.0**, source commit `7bcc27b1574493f9452688673658f52b80c53847`, as its behavioral baseline. The [official source](https://github.com/aws-powertools/powertools-lambda-typescript/tree/7bcc27b1574493f9452688673658f52b80c53847) and `tools/reference/package-lock.json` identify the version being compared. Latest upstream documentation can describe newer behavior; it does not silently change this baseline.

Each guide maps the corresponding official feature documentation to Go APIs, shows a complete example and describes its output. Go function names, contexts, native values and error handling differ from JavaScript. The detailed mappings below distinguish implemented capabilities from remaining compatibility gates. [CHECKLIST.md](CHECKLIST.md) remains the authoritative project progress record.

## Scheduled remaining work

[The phased backlog](ROADMAP.md) links 38 open work issues across six GitHub milestones. It separates absent/optional capabilities from compatibility audits and service/resource acceptance. This scheduling update implements no feature and does not advance the pinned baseline.

## How to read verification claims

| Evidence | What it establishes | What it does not establish |
| --- | --- | --- |
| Source/API inspection | A Go implementation or documented alternative exists | Correctness for every argument or native value |
| TypeScript differential fixture | Recorded cases match the pinned package under the documented normalization | Exhaustive behavior or newer TypeScript versions |
| Native Go regression tests | Scope isolation, contexts, ownership, errors and concurrency covered by those tests | Race-detector equivalence; CGO remains disabled |
| Docker Lambda acceptance | The built amd64 executable produces expected logs, EMF, responses and OTLP against local fixtures | Live IAM, CloudWatch ingestion, X-Ray indexing or managed-service semantics |
| arm64 build | Source compiles for Linux arm64 with CGO disabled | Execution on arm64 |
| Historical AWS acceptance | The stated scenarios passed on their original deployment/artifacts | A fresh cloud run or service acceptance for later utilities |

No utility currently claims exhaustive cross-language parity. A checked progress item records its own acceptance criteria, not all possible TypeScript behavior.

## Utility map

Use each guide's **TypeScript feature coverage** table for the individual features and its remaining-boundary section before migrating an application.

| TypeScript utility | Go guide and module | Implemented capability groups | Remaining compatibility or service gate |
| --- | --- | --- | --- |
| Logger | [Logger](LOGGER.md), `logger` | JSON, Lambda context, opt-in raw event logging before typed decoding, levels/ALC, attributes, children, correlation, sampling, buffering, formatters/replacers, timezone/key ordering | Complete diagnostics/configuration, native serialization, child/buffer edge cases |
| Tracer | [Tracer](TRACER.md), `tracer` | Handler/operation spans, annotations/metadata, HTTP/SDK v2, capture controls, context IDs, OTel provider injection | OTel representation differs from native X-Ray; exhaustive lifecycle, freeze/timeout and service-map behavior |
| Metrics | [Metrics](METRICS.md), `metrics` | EMF, units/resolution, dimension sets, multiple values, timestamps, cold start, selective clears, single metrics and multi-instance wrappers | Native metadata/types, full framework lifecycle and broader CloudWatch service boundaries; scoped extraction passed in [AWS_SERVICE_ACCEPTANCE.md](AWS_SERVICE_ACCEPTANCE.md) |
| Parameters | [Parameters](PARAMETERS.md), `parameters` and service subpackages | Five providers, caches/transforms, force fetch, missing values, SSM writes/batches, SDK injection | Duration/invalid-input diagnostics and live service semantics |
| Batch | [Batch](BATCH.md), `batch` | SQS/FIFO/Kinesis/DynamoDB, partial failures, sequential/parallel processing, parser/custom processor composition | Exhaustive malformed input/order edges and live retry/checkpoint behavior |
| Idempotency | [Idempotency](IDEMPOTENCY.md), `idempotency` | Operation/handler wrappers, payload projections, validation, lifecycle/leases, DynamoDB, local response cache and replay hooks | Complete canonical/native serialization, durable/platform, service and performance gates |
| Cache persistence | [Redis/Valkey](IDEMPOTENCY_CACHE.md), `idempotency/cache` | Reference record shape, acquisition/TTL, guarded orphan recovery and local bidirectional records | Deliberate validation/recovery differences; cluster/failover/server-expiry acceptance |
| JMESPath | [JMESPath](JMESPATH.md), `jmespath` | Queries, thirteen envelopes, three decode functions, custom callbacks and Logger extraction | Decoder errors are explicit in Go; Unicode/numeric/native serialization edges |
| Parser | [Parser](PARSER.md), `parser` | Manual/safe/wrapped parsing, schemas, composition, 90 mapped runtime schema definitions and fourteen envelopes | Complete inferred-type mapping, native/error metadata, primitive/encoding edges |
| Validation | [Validation](JSON_SCHEMA_VALIDATION.md), `validation` | Standalone/compiled/wrapped JSON Schema, extraction, formats, local references, compiler injection and Unicode regex | Complete AJV dialect/keyword/extension/error equivalence |
| HTTP | [HTTP](HTTP.md), `eventhandler/http` | Event adapters, routes, middleware/store, errors, validation, binary responses and streaming | Fetch/URL/header/native/regex edges and live platform acceptance |
| HTTP observability | [HTTP observability](HTTP_OBSERVABILITY.md), optional `eventhandler/http/metrics` and `eventhandler/http/tracer` | Request-scoped EMF, route spans and capture policy | Full TS middleware defaults/types and service delivery |
| AppSync Events | [Events](APPSYNC_EVENTS.md), `eventhandler/appsyncevents` | Publish/subscribe, wildcard routes, aggregate processing, authorization, size diagnostics | Complete native event/output types and live AppSync acceptance |
| AppSync GraphQL | [GraphQL](APPSYNC_GRAPHQL.md), `eventhandler/appsyncgraphql` | Field/type routes, split routers, batches, exception handling and scalar helpers | Arbitrary JS scope/native Date/error/serialization edges and live AppSync |
| Bedrock Agents | [Bedrock](BEDROCK.md), `eventhandler/bedrock` | Function tools, parameters, response states, session attributes and diagnostics | OpenAPI action groups are not this API; native/error/encoding and live Bedrock gates |
| Kafka | [Kafka](KAFKA.md), `kafka` | Lazy primitives/JSON, keys/values/headers, metadata, parsing and Idempotency composition | Complete SOURCE/JSON/native/error behavior and actual Kafka event-source acceptance |
| Avro/Protobuf | [Binary formats](KAFKA_BINARY.md), optional `kafka/avro` and `kafka/protobuf` | Codec adapters, schema metadata and scoped prefix handling | Exhaustive codec/schema/native values and registry/service acceptance |
| Data Masking | [Masking](DATAMASKING.md), `datamasking` | Erasure, selectors/rules, custom/dynamic masks, encryption-provider orchestration | Native structured-clone behavior, ordering/Unicode and provider scheduling edges |
| KMS masking provider | [KMS provider](DATAMASKING_KMS.md), optional `datamasking/kms` | Uncached AWS Encryption SDK provider and message interoperability | **TypeScript data-key caching is not implemented** ([#164](https://github.com/rambow-cloud/powertools-lambda-go/issues/164)); actual KMS policies/wrapping and native provider parity |
| Signer | [Signer](SIGNER.md), `signer` | Standalone SigV4, signed transport, credentials/region injection, replayable bodies | Fetch/native body/redirect differences and service authorization |
| Metadata | [Metadata](METADATA.md), `commons/metadata` | Execution-environment metadata, cache/clear, timeout, local fallback and explicit clients | Snapshot/coalescing/redirect differences; real LMDS availability/authentication |
| Shared foundation | [Commons](COMMONS.md), root and optional `commons/*` modules | Environment/runtime helpers, invocation identity, Base64, merge/LRU, DynamoDB, SDK marker and shared regex | Complete JavaScript/native type, encoding and performance boundaries |

## Reference evidence by feature

The development-only generators execute the actual pinned npm packages. Checked-in fixtures are consumed by the independently packaged Go modules; Node.js is not part of a deployed Lambda binary. [Reference tooling](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/tools/reference/README.md) describes generator inputs and normalization. Do not regenerate fixtures just to make an unexplained mismatch disappear.

| Feature | Go evidence entry points | Comparison detail |
| --- | --- | --- |
| Logger | [JSON comparison](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/logger/reference_test.go), [sampling](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/logger/sampling_test.go), [child/empty-field/buffer corpus](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/logger/parity_reference_test.go) | Parsed JSON; initial and deterministic sampling cases; 48 scenarios for separate child stores, shallow cleanup, trace lifecycle and UTF-8 buffer boundaries; overflow errors compared by message |
| Tracer | [Regression tests](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/tracer/tracer_test.go), [local acceptance](LOCAL_VALIDATION.md) | Go/OTel context, lifecycle and local export contracts; no TS native-document parity claim |
| Metrics | [EMF](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/metrics/metrics_test.go), [timestamps](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/metrics/timestamp_test.go), [wrappers](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/metrics/lambda_test.go) | Store/diagnostic/cold/config/value/time/wrapper corpora; dimension-name normalization documented per corpus |
| Parameters | [Cache/transforms](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parameters/parameters_test.go), [providers](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parameters/providers_test.go) | Outputs plus SDK operation sequences; provider-specific boundaries retained |
| Batch | [Reference scenarios](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/batch/reference_test.go) | Failure responses, FIFO/groups, sequential/parallel and malformed envelopes |
| Idempotency/cache | [Lifecycle/keys](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/idempotency/idempotency_test.go), [cache](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/idempotency/cache/store_test.go) | Canonical JSON, record lifecycle, writer differences and guarded recovery |
| Parser | [Event corpora](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parser/reference_test.go), [unions](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parser/union_reference_test.go) | Data and recursive issue trees; JSON syntax suffix normalization is explicit |
| Validation | [Reference suites](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/validation/reference_test.go) | Ordered issues, keyword/format/reference/regex and condition-graph corpora |
| HTTP | [Core](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/eventhandler/http/reference_test.go), [middleware](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/eventhandler/http/middleware_reference_test.go), [streaming](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/eventhandler/http/stream_reference_test.go) | JSON bodies compared as values; text/binary, headers, cookies and status directly |
| AppSync/Bedrock | [Events](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/eventhandler/appsyncevents/reference_test.go), [GraphQL](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/eventhandler/appsyncgraphql/reference_test.go), [Bedrock](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/eventhandler/bedrock/reference_test.go) | Responses, diagnostics and calls; Bedrock body strings are not parsed/reordered |
| Kafka | [Core](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/kafka/reference_test.go), [Avro](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/kafka/avro/avro_test.go), [Protobuf](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/kafka/protobuf/protobuf_test.go) | Lazy values, delivery modes and codec outputs with native representation boundaries |
| Masking | [Core](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/datamasking/masking_test.go), [regex](REGEX.md), [encryption interoperability](DATAMASKING_KMS.md) | Erasure/provider outputs; local wrapping fixtures are not actual KMS security acceptance |
| JMESPath/Signer/Commons | Utility tests linked from [JMESPath](JMESPATH.md), [Signer](SIGNER.md) and [Commons](COMMONS.md) | Queries/signatures/shared primitives; deliberate Go errors/types remain explicit |

## Scope corrections

OpenAPI generation is labelled **Coming soon** in the [pinned HTTP documentation](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/event-handler/http.md#openapi), and is absent from that version's runtime export audit. It is not an implemented v2.35.0 feature missing from Go. Request/response schema validation is a separate implemented feature.

Parser/Idempotency composition with Batch already exists in the maintained examples and local fixture. Metrics numeric/Date timestamps and configurable wrapper error precedence already have differential tests. Those capabilities must not remain listed as wholly unimplemented. Their exhaustive edge cases remain open.

TypeScript decorators, Middy hooks, class inheritance and Promise behavior map to explicit Go functions/interfaces/contexts. The [usage patterns](USAGE_PATTERNS.md) explain those changes. The maintained tracing backend is OpenTelemetry; the frozen `tracer/xray` adapter does not define new acceptance requirements.

## Reproduce the accepted scope

From the checkout, run `uv run python integration/local/run.py` for packaged-module tests/vet/dependency checks, standalone consumers, both Linux architecture builds and amd64 Docker acceptance. Run `uv run python integration/local/batch_report.py` once against the resulting artifacts for additional saved Batch checks. The maintained runner supplies `CGO_ENABLED=0`; [local runner details](LOCAL_INTEGRATION.md) describe flags, resources and cleanup.

Do not use root `go test ./...` as coverage for nested modules. For module-only acceptance, use `uv run python tools/modules.py check`. Reuse a passing scope instead of repeating it; record skipped/reused phases explicitly. Keep newly observed mismatches and unsupported features open in [project progress](CHECKLIST.md).

The [acceptance history](LOCAL_VALIDATION.md) and sanitized reports identify the artifacts and scope of recorded runs. Browser presentation and live AWS behavior remain separate checks. Cloud testing requires an explicit request and explicit local test configuration.

## Latest documentation audit

The 2026-10-01 [combined acceptance record](DOCUMENTATION_ACCEPTANCE.json) covers eighteen guides, nineteen compiled programs, eighteen local/direct executions, all 31 modules and 28 public consumers, both Linux builds and 868/868 RIE, 95/95 streaming and 14/14 saved Batch checks. Passed phases were reused after dependency download failures. Direct example execution complements the maintained Runtime API fixture; it does not establish live Lambda service acceptance. Browser presentation and exhaustive parity remain open.
