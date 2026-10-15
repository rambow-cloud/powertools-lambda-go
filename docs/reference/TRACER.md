# Tracer: API and behavior notes

Start with the [Tracer guide](../TRACER.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

## Initialize and compose

Create a tracer before `lambda.Start`:

~~~go
appLog := logger.New(logger.WithServiceName("orders"))
t, err := tracer.New(
    tracer.WithServiceName("orders"),
    tracer.WithCaptureResponse(false),
    tracer.WithErrorHandler(func(err error) {
        if logErr := appLog.Error("Tracing failed", err); logErr != nil { panic(logErr) }
    }),
)
if err != nil {
    return err
}
~~~

The fragment runs in a function returning `error` and requires the Powertools `logger` and `tracer` imports. Wrap your typed handler with `tracer.WrapHandler(t, handler)`. When using Logger, put Tracer outside the Logger wrapper so logs can read the active span.

Create a child operation with `tracer.Capture(ctx, t, name, operation)`, or use `StartSpan` and invoke the returned completion function exactly once. Pass the returned context into downstream operations.


## Collector and AWS X-Ray

The default backend owns a private OTel provider with an OTLP/HTTP exporter. It does not replace global providers.

Set `OTEL_EXPORTER_OTLP_ENDPOINT` to a reachable collector base URL, such as `http://localhost:4318` for a local Lambda extension. Alternatively use `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` for a signal-specific URL. The default localhost destination does not install or start a collector.

To deliver to AWS X-Ray, configure the collector's `awsxray` exporter, AWS region, and execution-role permissions. The library uses OTel's X-Ray-compatible ID generator and propagator without the X-Ray SDK. Enable Lambda active tracing when a native Lambda parent/service relationship is required. See the [collector configuration](../XRAY_MIGRATION.md#maintained-path).


## HTTP and AWS SDK v2

Given an initialized tracer and AWS SDK configuration:

~~~go
client := t.HTTPClient(http.DefaultClient)
ddb := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
    t.InstrumentAWS(&o.APIOptions)
})
~~~

Import `net/http` and `github.com/aws/aws-sdk-go-v2/service/dynamodb` for this fragment. Pass the invocation context to each request. Instrument a transport or SDK client once; duplicate tracing middleware produces duplicate spans.

HTTP event extraction is opt-in. Set `tracer.HandlerOptions.ExtractContext` to `tracer.ExtractLambdaHTTPContext` for supported API Gateway, Function URL, ALB, and AppSync HTTP header envelopes, or provide an application extractor. Existing valid OTel parents take precedence; otherwise valid W3C context takes precedence over X-Ray headers.


## Configuration

| Setting | Default / purpose |
| --- | --- |
| `WithServiceName` / `POWERTOOLS_SERVICE_NAME` | Service identity; fallback `service_undefined` |
| `WithEnabled` / `POWERTOOLS_TRACE_ENABLED` | Enabled in Lambda unless disabled; environment `false` vetoes |
| `WithLocalTracing(true)` | Explicitly allow tracing outside Lambda |
| `WithCaptureResponse` / `POWERTOOLS_TRACER_CAPTURE_RESPONSE` | Response capture enabled by default |
| `WithCaptureError` / `POWERTOOLS_TRACER_CAPTURE_ERROR` | Error capture enabled by default |
| `WithCaptureHTTP` / `POWERTOOLS_TRACER_CAPTURE_HTTPS_REQUESTS` | HTTP capture enabled by default |
| `WithFlushTimeout` | Two-second maximum by default, bounded by the invocation deadline |
| `OTEL_TRACES_SAMPLER` / `OTEL_TRACES_SAMPLER_ARG` | SDK sampling policy; default parent-based always-on |
| `POWERTOOLS_DEV=true` | Disable tracing, including local tracing |

Disable response/error capture where payloads should not be recorded. `PutAnnotation` and `PutMetadata` attach explicit values and return errors. `TraceID`, `XRayTraceID`, and `IsTraceSampled` query current context.


## Provider ownership and delivery

Supply an application-owned provider with `tracer.WithBackend(tracer.NewOTelBackend(provider))`. The application configures its resource, exporter, sampler, limits, and any required X-Ray ID generator. It also owns that provider's shutdown; invocation wrappers flush supported providers but do not shut down an injected provider.

Invocation completion ends spans and attempts a bounded flush. Hard timeouts or a failing collector can prevent delivery. Use `WithErrorHandler` for instrumentation and flush diagnostics; callbacks ignore errors by default.

OTel metadata attributes and legacy X-Ray metadata documents differ. Local validation is not proof of every AWS indexing, freeze/thaw, or service behavior. See [compatibility](../COMPATIBILITY.md), [local evidence](../LOCAL_VALIDATION.md), and the [remaining Tracer work](../CHECKLIST.md#tracer).
### Try tracing offline

The maintained local example uses an in-memory exporter and also shows Logger
correlation. It needs no collector or AWS credentials:

```sh
CGO_ENABLED=0 go run ./examples/local
```

~~~go
--8<-- "examples/local/main.go"
~~~

It writes a `Hello` log and a structured `Span recorded` log with the span name and trace ID. Use this
combined example after the initialization fragment above.


## Objects and lifecycle

| Object | How to use it |
| --- | --- |
| `trace` | Create once before `lambda.Start`; reuse the tracer across invocations. |
| Wrapped handler | Creates the invocation span and performs a bounded flush. |
| `ctx` inside a handler | Pass it into child operations, HTTP requests and AWS SDK calls. |
| Injected provider | Optional application-owned exporter/provider; your application owns its shutdown. |


## Finite lifecycle and capture audit (#182)

This audit covers maintained OTel APIs and reuses the 128 pinned TypeScript 2.35.0
HTTP middleware contract cases. The recording reference contract checks high-level
calls; it does not execute the frozen X-Ray SDK or establish native SDK document
equivalence. The new local tests use real OTel SDK spans and injected transports.

| Boundary / APIs | Evidence | Classification |
| --- | --- | --- |
| Response/error/HTTP capture options and environment vetoes | Fifteen control combinations in `TestCaptureConfigurationBoundaries`, with handler-level response suppression | Verified constructor opt-out, case-insensitive environment veto, and preserved typed results/native error status |
| Tracing enablement | `TestTracingEnablementVetoBoundaries` and existing disabled-provider tests | Verified explicit opt-out, environment/development veto and local-runtime defaults without creating spans |
| `HTTPClient`, `InstrumentAWS` | Existing propagation/client tests; `TestSDKTracingGateAndHTTPControlIndependence` | Verified copied clients, request immutability, enabled/disabled SDK tracing and one SDK identity marker. HTTP capture opt-out does not disable explicit SDK instrumentation |
| `WrapHandler` flush, errors and panic | Five cases in `TestInvocationFlushBoundaries` plus the existing bounded waiting-flush test | Verified span completion before flush, caller values, detached cancellation within the original deadline/timeout, expired deadlines, diagnostic errors and original business result/error/panic |
| Provider ownership | Injected/owned provider counters in the flush cases and existing provider tests | Invocation cleanup never shuts down an injected provider; only an owned backend delegates shutdown |
| Span completion, late writes and concurrent scopes | `TestClosedSpansAndConcurrentInvocationIsolation` across 32 calls; `TestOTelSiblingSpanSurvivesCompletion` | Verified idempotent completion, distinct span/request attributes and sibling survival. Closed SDK spans ignore late metadata/annotations while retaining trace identity; no new export is produced |
| Metadata and backend start failures | `TestStartFailureAndMetadataFailurePreserveResults` and existing namespace-key regression | Verified instrumentation diagnostics preserve the business result; metadata uses strict JSON v2 and separate escaped namespace/key components |
| Context fallback and sampling | Existing HTTP/event-envelope precedence, malformed-input/cancellation, default-provider sampler/X-Ray-ID tests | Verified existing OTel parent, W3C then X-Ray propagation, runtime fallback, baggage, deadlines and sampled/unsampled decisions |
| HTTP route defaults, ownership and closure | Existing 128 contract cases, body/error/panic/cancellation/streaming tests and eight `TestResponseCaptureStorageAndParsingControls` cases | Verified route parent restoration, exact Content-Type/default capture, body replay/close ownership, and independent parsing/storage controls |

No new production defect was reproduced by this finite audit. Native differences
remain explicit: OTel attributes/events and errors replace X-Ray documents; closed
SDK spans are no-ops rather than Logger-style closed-scope errors. An injected
custom backend must implement its documented concurrency, completion, ownership
and cancellation contracts; the wrapper cannot force-stop arbitrary callbacks.

For HTTP middleware, disabling Tracer-wide response capture suppresses metadata
storage but still permits middleware JSON parsing and its errors. Set
`httptracer.Options.DisableCaptureResponse` to bypass body reading/parsing. This
preserves the pinned high-level middleware contract rather than changing defaults.

Both affected packaged modules pass tests/vet/tidy/isolation and independent
consumers with CGO disabled. Collector/X-Ray service indexing, freeze/timeout and
Managed Instances behavior remain separate in #197/#202 and T-08. Exhaustive native
type/error, propagator, exporter and resource-budget acceptance is outside this
case matrix; #200 tracks resource budgets. The frozen `tracer/xray` adapter is
unchanged.
