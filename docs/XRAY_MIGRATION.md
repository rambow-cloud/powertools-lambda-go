---
description: "Migrate Go Lambda tracing from the deprecated X-Ray SDK adapter to OpenTelemetry with a collector's awsxray exporter."
---

# X-Ray through OpenTelemetry

Decision date: 2026-09-14. OpenTelemetry is the maintained tracing implementation, including delivery to AWS X-Ray. The independent `tracer/xray` SDK adapter is deprecated and frozen. No new legacy SDK features or compatibility fixes are planned.

AWS's current support timeline says X-Ray SDKs and the daemon entered maintenance mode on February 25, 2026, with security updates only and no specified end date. The X-Ray service continues to accept traces through OpenTelemetry solutions. This project freezes its adapter as a separate maintenance decision; do not describe AWS's maintenance mode as a complete end to security support. [AWS support timeline](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-daemon-timeline.html), [AWS migration guide](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-migration.html).

## Maintained path

The Lambda application uses `tracer` to emit OpenTelemetry spans over OTLP/HTTP. An ADOT or compatible OpenTelemetry Collector receives OTLP and uses its `awsxray` exporter to send traces to the X-Ray service. This path does not import `github.com/aws/aws-xray-sdk-go` or require the legacy X-Ray daemon. Configure the collector as a Lambda extension appropriate to the deployment. Enabling Lambda active tracing remains necessary when expecting a native Lambda X-Ray parent and service relationship.

```go
tr, err := tracer.New(tracer.WithServiceName("orders"))
if err != nil {
    return err
}
```

Set `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` when a local collector extension listens there. This setting names a collector endpoint, not the AWS X-Ray service API. Do not point the OTLP exporter directly at an X-Ray API endpoint. Configure the collector's AWS region and permissions, including `xray:PutTraceSegments` and `xray:PutTelemetryRecords`, for the selected deployment.

A minimal collector configuration is retained at [integration/collector.yaml](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/integration/collector.yaml):

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: localhost:4318
exporters:
  awsxray:
    region: ${env:AWS_REGION}
    indexed_attributes: [ColdStart, Service]
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [awsxray]
```

The default provider generates X-Ray-compatible root trace IDs, reads native Lambda X-Ray parents, and propagates both W3C and X-Ray HTTP headers. Existing OTel parents take precedence. The Go SDK's `OTEL_TRACES_SAMPLER` and `OTEL_TRACES_SAMPLER_ARG` settings are honored; the unset default is parent-based always-on. Use a parent-based sampler when preserving Lambda's sampling decision. An application-owned provider retains its own sampler, ID generator, and shutdown ownership.

For HTTP-triggered events, use `tracer.ExtractHTTPContext(ctx, headers)` from `HandlerOptions.ExtractContext`, or set `ExtractContext: tracer.ExtractLambdaHTTPContext` for API Gateway, Function URL, ALB, and AppSync HTTP header envelopes. These helpers retain an existing valid OTel context, otherwise read W3C/X-Ray headers with valid W3C context taking precedence. They preserve context cancellation and deadlines. Applications decide which event headers to accept as tracing parents; extraction remains opt-in.

`Tracer.TraceID` and `Tracer.IsTraceSampled` also recognize external OTel contexts. `Tracer.XRayTraceID` formats the ID for X-Ray correlation without loading the X-Ray SDK. Logger derives `trace_id`, `span_id`, and `xray_trace_id` from the active OTel context, overriding stale runtime trace identity. Its buffer supports both sampled and unsampled OTel contexts even when no X-Ray runtime header exists.

## Legacy adapter behavior

The package, `Backend`, constructor, and module metadata carry Go `Deprecated:` notices. Calling `tracer/xray.New` prints an English migration warning to stderr once per process. Merely importing the maintained Tracer does not load the SDK or print a warning. Existing adapter tests remain as regression evidence; passing them does not restore maintained status.

Remove the SDK import and the `tracer.WithBackend(legacy.New(...))` option to use the default OTel provider. If the application owns an OTel provider, use `tracer.WithBackend(tracer.NewOTelBackend(provider))` instead. Remove unused SDK dependencies from the application's own module.

The maintained Lambda integration fixture is now OTel-only and rejects `TRACE_BACKEND=xray` with a migration message. Future cloud test stacks use OTel on both architectures. Historical dual-backend AWS evidence remains unchanged and is explicitly historical.

## Validation boundaries

Local tests cover context precedence, invalid headers, cancellation, external trace queries, environment sampling, timestamp-based root IDs, header-free OTel log buffering, and legacy regression behavior. Docker verifies real OTLP requests and Logger/Metrics/Parameters/Metadata composition. New live AWS collector delivery, indexing, freeze/thaw, and hard-timeout validation have not been performed in this iteration. OTLP JSON metadata attributes still differ from legacy X-Ray metadata documents; see [COMPATIBILITY.md](COMPATIBILITY.md).
