# Compatibility contract

Reference: TypeScript v2.35.0, commit `7bcc27b1574493f9452688673658f52b80c53847`. The goal is feature and observable-behavior parity within the maintained scope. Implementation and scoped verification exist across the current utility families; exhaustive parity remains unfinished. A checked implementation item does not imply exhaustive reference parity.

Start with the [complete feature comparison and evidence map](FEATURE_PARITY.md). Each utility guide has a TypeScript feature table, complete example, output explanation and boundaries. [Usage patterns](USAGE_PATTERNS.md) maps decorators, middleware and objects to Go; [environment variables](ENVIRONMENT_VARIABLES.md) records supported configuration. [CHECKLIST.md](CHECKLIST.md) describes project progress.

Scope adjustment (2026-09-13): feature parity work now prioritizes Logger and OpenTelemetry. Exhaustive legacy X-Ray SDK behavior and native document parity are outside the maintained compatibility scope. The adapter's known DynamoDB enrichment gap is retained as a documented limitation, not an active release requirement.

Scope adjustment (2026-09-14): the legacy SDK adapter is now deprecated and frozen, with module/API notices and a constructor warning. Maintained X-Ray support uses OpenTelemetry and the collector's awsxray exporter. The current integration program is OTel-only. See [XRAY_MIGRATION.md](XRAY_MIGRATION.md).

## Logger API mapping

| TypeScript concept | Go surface | Status and differences |
| --- | --- | --- |
| Logger constructor | `logger.New(options...)` | Functional options, private configuration, invalid values fall back |
| trace/debug/info/warn/error/critical | `Trace/Debug/Info/Warn/Error/Critical` | Message is a string; additional values can be Fields, error, or string; unsupported extra types are ignored |
| appendKeys/removeKeys/resetKeys | `AppendKeys/RemoveKeys/ResetKeys` | Temporary keys; ResetKeys clears temporary keys only |
| persistent keys | `AppendPersistentKeys/RemovePersistentKeys/PersistentKeys` | Shared Commons object/indexed-array merge; returned nested values must be treated as immutable |
| createChild | `Child(options...)` | Separate persistent/temporary snapshots and shared sink lock; inherited temporary precedence and independent reset/removal |
| Lambda context injection | `WrapHandler` plus `WithContext` | Typed functions replace decorators/Middy; request state always resets |
| correlation ID extraction | `SetCorrelationID`, `ExtractCorrelationID`, or handler callback/source/extractor | All nine built-in source locations and optional compiled JMESPath expressions; query failures use instrumentation diagnostics |
| custom formatter/JSON replacer | `WithFormatter/WithReplacer` | Go callback model; structs/marshalers use their JSON representation for descendant replacement, preserving JSON tags and numeric tokens |
| bufferLogs/flushBuffer/clearBuffer | `WithBuffer/FlushBuffer/ClearBuffer` | Active-trace flush/clear, sequential trace replacement, byte-size eviction, oversize error details and error auto-flush; runtime X-Ray root or valid OTel context, including unsampled contexts |
| getLevelName/getLogEvent/getCorrelationId | `GetLevelName/GetLogEvent/GetCorrelationID` | Effective scoped level, constructor event setting, and temporary correlation ID |
| log sampling | `WithSampleRate` and invocation entry | Constructor decision/diagnostic, first-call reuse, and independent warm sampling; deterministic reference fixture verified |
| uncaught error flush | `HandlerOptions.FlushBufferOnError` | Defaults to false, consistent with reference middleware option |

Level thresholds and core JSON keys follow the reference. ALC takes precedence over configured levels. Log attributes use persistent, temporary, then per-call precedence; reserved core fields cannot be overridden. Additional fields have deterministic alphabetic order, rather than JavaScript insertion order. The initial differential fixture compares parsed JSON and removes only timestamp. It exercises three emitted records and one filtered record. An additional 48-scenario corpus covers separate child stores, shallow top-level empty/null cleanup before replacement, and trace-aware buffering, with a fixed clock for UTF-8 capacity boundaries. Overflow errors are compared by message because names, stacks and locations are native. These corpora do not certify all Logger behavior. Empty cleanup applies to map-shaped documents; custom marshalers and struct-shaped formatter results retain Go encoding behavior.

Go byte slices use encoding/json base64 output, arbitrary integers retain Go encoding semantics, and errors expose Go type names and unwrap causes. Go does not automatically capture JavaScript-style stacks or source locations. Circular maps/slices/pointers use markers; deep values are truncated. Struct fields and custom JSON marshalers follow Go encoding rules. Attribute values and callbacks are application-owned and must be concurrency-safe; do not mutate nested values while a logger retains them. Custom replacers are not a complete JavaScript JSON.stringify emulation.

Commons migration (2026-09-14): persistent and temporary key updates share the reference indexed-array/object merge contract, with reserved root fields filtered by Logger. Shared configuration preserves strict versus extended boolean modes. See [COMMONS.md](COMMONS.md) and [COMMONS_REUSE.md](COMMONS_REUSE.md); constructor fallback diagnostics and complete per-record JavaScript serialization remain separate gates.

## Tracer API mapping

| TypeScript concept | Go surface | Status and differences |
| --- | --- | --- |
| captureLambdaHandler | `tracer.WrapHandler` | Typed wrapper; operation name `## handler`; closes and flushes on return/error/panic |
| captureMethod | `tracer.Capture` | Typed operation; name `### operation`; closes span, outer wrapper flushes |
| subsegments | `StartSpan` and completion callback | Callback is idempotent; pass returned context explicitly |
| putAnnotation/putMetadata | `PutAnnotation/PutMetadata` | Active context required; default namespace is service |
| captureHTTPsRequests | `HTTPClient` | Explicit copied client; no global monkey patching |
| captureAWSv3Client | `InstrumentAWS` | Go AWS SDK v2 middleware; unsupported SDK v1 |
| getSegment/setSegment/provider access | `Backend`, OTel or X-Ray native context APIs | No cross-backend raw Segment type |
| environment capture controls | `WithCaptureResponse/Error/HTTP` and reference env flags | False environment flags veto options; local tracing explicitly enabled |
| OTel propagation and trace queries | `ExtractHTTPContext`, `TraceID`, `XRayTraceID`, `IsTraceSampled` | Go OTel extension: external OTel contexts, explicit HTTP extraction, and formatted X-Ray IDs without the legacy SDK |
| OTel sampling | `OTEL_TRACES_SAMPLER`, `OTEL_TRACES_SAMPLER_ARG` | Default provider follows SDK configuration; unset default is parent-based always-on |

## Backend data differences

| Concern | OpenTelemetry default | Deprecated, frozen X-Ray SDK adapter |
| --- | --- | --- |
| Trace entities | OTel server/internal/client spans | Native SDK subsegments beneath Lambda facade |
| Trace IDs returned by Tracer | 32 hexadecimal characters | X-Ray `1-epoch-random` form |
| Parent | Existing OTel context, then runtime X-Ray header | SDK segment or runtime X-Ray header |
| No runtime parent | Creates a root span when enabled | No-op without an existing segment |
| Annotations | Attributes plus `aws.xray.annotations` keys | Native annotations |
| Metadata | JSON string attribute `powertools.metadata.<namespace>.<key>` | Native namespaced JSON metadata |
| Errors | OTel exception events and error status | SDK cause/flags; Go SDK exception formatting |
| Export | OTLP/HTTP; collector performs backend translation | SDK daemon emission on end |
| Flush | Bounded provider ForceFlush | No-op adapter |

OTel metadata is not byte-for-byte X-Ray metadata. Each namespace/key component escapes `%` as `%25` and `.` as `%2E`, preventing collisions; ordinary names keep their existing attribute keys. Consumers querying dotted/percent-containing names must use the escaped form. OTel provider/collector limits can truncate or drop attributes, spans, and annotations. Native X-Ray documents, indexing, service-map topology, and error categories require cloud verification; the optional backend does not by itself certify TypeScript raw-document parity.

Hong Kong integration testing observed additional representation differences: the OTel AWS SDK instrumentation and collector record DynamoDB GetItem as `aws.operation: DynamoDB/GetItem`, while the X-Ray SDK records `aws.operation: GetItem`. Both include a completed downstream span, region, and service request ID. The OTel path also includes the table name; the selected X-Ray SDK v2 middleware does not include it. Adding that enrichment remains a compatibility gap. Backend acceptance tests check each supported representation explicitly; passing these tests does not establish raw-document parity or table-name enrichment in the X-Ray adapter.

Logger adds `trace_id`, `span_id`, and a formatted `xray_trace_id` from a valid OTel context, which takes precedence over runtime headers. Without OTel context, runtime trace identity remains the fallback. Header-free OTel buffering and OTel correlation are intentional extensions to the pinned SDK-based TypeScript Tracer behavior. Wrapper composition shares cold-start identity. Shared invocation identity marks the first process invocation; caller-created invocation contexts are request-owned. Metrics additionally requires on-demand initialization and consumes its per-instance Commons Utility flag for manual/wrapper capture. Bound warm scopes cannot emit ColdStart by constructing another instance. Runtime context headers take precedence over shared environment state, including empty headers. The environment fallback is disabled when `AWS_LAMBDA_MAX_CONCURRENCY` indicates multiple concurrent invocations.

## Explicitly unfinished

- Complete exported-symbol/default/error audit and comprehensive TypeScript differential fixtures.
- Logger full diagnostic/configuration behavior, JavaScript serialization edge cases, and complete child/buffer parity. Arbitrary correlation expressions and struct/marshaler descendant replacement are implemented.
- Tracer decorators/Middy-equivalent options, complete OTel error/HTTP/SDK parity, non-HTTP event extraction, and collector/X-Ray document fixtures. Opt-in HTTP event-envelope extraction is implemented; legacy native segment compatibility is frozen.
- Managed Instances lifecycle validation, durable execution semantics, and event streaming behavior.
- Exhaustive Lambda integration, hard-timeout/freeze recovery, performance/cold-start benchmarks, size budgets, and release/license audit. The scoped Hong Kong deployment results are in [AWS_VALIDATION.md](AWS_VALIDATION.md).
- Metrics native/metadata encoding, remaining type boundaries, exhaustive framework lifecycle and real CloudWatch extraction. Numeric/Date timestamp input and clock reads are implemented and verified, alongside store, warning, cold-start, configuration, values and wrapper error precedence. See [METRICS.md](METRICS.md). SingleMetric returns (*Metrics, error); callers must handle fresh-construction failures.
- Parameters providers and convenience helpers are implemented with scoped SDK, reference, and Docker coverage. Numeric/invalid-input/diagnostic boundaries and service acceptance remain open; see [PARAMETERS.md](PARAMETERS.md).
- Batch, Idempotency, Parser, Validation, event handlers, Kafka, Data Masking, Signer, JMESPath and Metadata have implementations and scoped reference/native/runtime evidence. Remaining gates are mapped in [FEATURE_PARITY.md](FEATURE_PARITY.md). Optional KMS masking is uncached; TypeScript data-key caching remains unsupported. HTTP OpenAPI generation was not implemented in the pinned TypeScript baseline.

`CGO_ENABLED=0` is mandatory. Race-detector execution is excluded because it requires CGO. Concurrent functional tests do run, but are not a substitute for a race detector.
