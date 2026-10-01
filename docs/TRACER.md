# Tracer

Tracer captures Lambda handlers, application operations, outbound HTTP requests and AWS SDK v2 calls as OpenTelemetry spans. Export through an OTLP collector; use its `awsxray` exporter for AWS X-Ray. Import `github.com/rambow-cloud/powertools-lambda-go/tracer`.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Complete example

This complete offline example injects an in-memory OTel exporter. Run `go run ./examples/local` from the checkout with `CGO_ENABLED=0`; no collector or AWS credentials are needed for this example. For Lambda, use [the complete handler](GETTING_STARTED.md#create-utilities-once) and [configure a collector](#collector-and-aws-x-ray).

~~~go
--8<-- "examples/local/main.go"
~~~

## Input and output

The program writes one `INFO` JSON log with `message: "Hello"`, `service: "demo"` and `name: "Go"`, followed by a line like `Span: ## bootstrap, trace: <32 hexadecimal characters>`. The log contains the same `trace_id` and the handler span's `span_id`. IDs and the timestamp change per run. The handler returns `"Hello, Go"`; this example does not print that return value. A production exporter sends spans to its collector instead of printing `Span:` lines.

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `tr` | Reusable tracer configuration; initialize once and handle the construction error. |
| `ctx` | Active parent/span context; pass it to `Capture`, HTTP requests and AWS SDK calls. |
| `provider` / `exporter` | Application-owned OTel infrastructure; this example shuts the provider down explicitly. |
| `handler` | Wrapped callable; it ends the handler span and flushes on success, error or panic. |

## TypeScript feature coverage

Compared with the [official v2.35.0 tracer guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/tracer.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| Lambda handler / methods | `WrapHandler`, `Capture`, `StartSpan` | Typed callbacks replace decorators and Middy. |
| Annotations and metadata | `PutAnnotation`, `PutMetadata` | OTel attributes/events; metadata differs from native X-Ray documents. |
| AWS / HTTP instrumentation | `InstrumentAWS`, `HTTPClient` | Explicit SDK v2/client instrumentation, without global patching. |
| Response / error capture | `WithCaptureResponse`, `WithCaptureError` | Opt out at initialization; business results are preserved. |
| X-Ray root ID | `XRayTraceID(ctx)` | Formats an OTel ID or uses runtime context. |
| Escape hatch | `WithBackend`, `NewOTelBackend` | Injected provider ownership is explicit; no shared raw X-Ray Segment API. |
| Tracing enablement and sampling | `WithLocalTracing`, OTel sampler environment | OTel extension; deprecated SDK backend is frozen. |

Executable evidence: [tracer/tracer_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/tracer/tracer_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.

## Initialize and compose

Create a tracer before `lambda.Start`:

~~~go
t, err := tracer.New(
    tracer.WithServiceName("orders"),
    tracer.WithCaptureResponse(false),
    tracer.WithErrorHandler(func(err error) {
        log.Printf("Tracing failed: %v", err)
    }),
)
if err != nil {
    log.Fatal(err)
}
~~~

The fragment requires `log` and `tracer` imports. Wrap your typed handler with `tracer.WrapHandler(t, handler)`. When using Logger, put Tracer outside the Logger wrapper so logs can read the active span.

Create a child operation with `tracer.Capture(ctx, t, name, operation)`, or use `StartSpan` and invoke the returned completion function exactly once. Pass the returned context into downstream operations.

## Collector and AWS X-Ray

The default backend owns a private OTel provider with an OTLP/HTTP exporter. It does not replace global providers.

Set `OTEL_EXPORTER_OTLP_ENDPOINT` to a reachable collector base URL, such as `http://localhost:4318` for a local Lambda extension. Alternatively use `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` for a signal-specific URL. The default localhost destination does not install or start a collector.

To deliver to AWS X-Ray, configure the collector's `awsxray` exporter, AWS region, and execution-role permissions. The library uses OTel's X-Ray-compatible ID generator and propagator without the X-Ray SDK. Enable Lambda active tracing when a native Lambda parent/service relationship is required. See the [collector configuration](XRAY_MIGRATION.md#maintained-path).

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

OTel metadata attributes and legacy X-Ray metadata documents differ. Local validation is not proof of every AWS indexing, freeze/thaw, or service behavior. See [compatibility](COMPATIBILITY.md), [local evidence](LOCAL_VALIDATION.md), and the [remaining Tracer work](CHECKLIST.md#tracer).
