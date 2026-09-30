# Tracer

Tracer instruments native Go Lambda handlers with OpenTelemetry. It captures handler and operation spans, outbound HTTP, AWS SDK v2 calls, annotations, and optional response/error metadata.

Import `github.com/rambow-cloud/powertools-lambda-go/tracer`. See [installation](MODULES.md) and the [complete Lambda example](GETTING_STARTED.md#create-utilities-once).

!!! note "Maintained backend"
    Use OpenTelemetry for every new integration, including delivery to AWS X-Ray. The separate `tracer/xray` SDK adapter is deprecated and frozen. [Migration instructions](XRAY_MIGRATION.md) describe the maintained collector path.

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
