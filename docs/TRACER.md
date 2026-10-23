---
title: OpenTelemetry tracing for Go Lambda
description: "Trace Go Lambda handlers, HTTP requests and AWS SDK calls with OpenTelemetry. Deliver spans to AWS X-Ray through an OTLP collector."
---

# Tracer

Trace Lambda handlers, child operations, HTTP requests and AWS SDK calls with OpenTelemetry. Send spans to an OTLP collector and, if needed, onward to AWS X-Ray.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/tracer@v1.3.0
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## See a handler span locally

This example records spans in memory so you can see the result without a collector or AWS credentials. Install `go.opentelemetry.io/otel/sdk@v1.46.0` for this local exporter.

**Input**

```text
Ada
```

**Code**

```go
package main

import (
	"context"
	"os"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	if err := run(appLog); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func run(appLog *logger.Logger) error {
	ctx := context.Background()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(ctx)
	tr, err := tracer.New(tracer.WithLocalTracing(true),
		tracer.WithBackend(tracer.NewOTelBackend(provider)))
	if err != nil {
		return err
	}
	handler := tracer.WrapHandler(tr, func(ctx context.Context, name string) (string, error) {
		return "Hello, " + name, nil
	})
	result, err := handler(ctx, "Ada")
	if err != nil {
		return err
	}
	spans := make([]string, 0, len(exporter.GetSpans()))
	for _, span := range exporter.GetSpans() {
		spans = append(spans, span.Name)
	}
	return appLog.Info("Local trace complete", logger.Fields{"greeting": result, "spans": spans})
}
```

The Logger timestamp varies on each run.

**Output**

```json
{
  "level": "INFO",
  "message": "Local trace complete",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "greeting": "Hello, Ada",
  "spans": [
    "## bootstrap"
  ]
}
```

## Use it in Lambda

Create one tracer with `tracer.New`, then wrap the business handler:

```go
lambda.Start(tracer.WrapHandler(tr, handler))
```

For production, use the default OTLP exporter or your own provider. Configure a reachable collector with `OTEL_EXPORTER_OTLP_ENDPOINT`. To deliver traces to AWS X-Ray, use a collector with the `awsxray` exporter; see [collector setup](XRAY_MIGRATION.md).

## Trace a child operation

```go
result, err := tracer.Capture(ctx, tr, "charge-payment", func(ctx context.Context) (string, error) {
    return "approved", nil
})
```

The trace gains a child span named `### charge-payment`; the business result remains `"approved"`.

## Trace outbound calls

| Call | API |
| --- | --- |
| HTTP | `tr.HTTPClient(baseClient)` |
| AWS SDK v2 | `tr.InstrumentAWS(&cfg.APIOptions)` |
| Searchable attribute | `tr.PutAnnotation(ctx, "order_id", "ORD-123")` |
| Structured detail | `tr.PutMetadata(ctx, "order", value)` |

Pass the current context into downstream calls. When composing with Logger, put Tracer outside the Logger wrapper so records can include the active trace ID.

## Control captured data

Use `WithCaptureResponse(false)` or `WithCaptureError(false)` when those values should not be stored in spans. `WithErrorHandler` observes instrumentation/export failures. Sampling is configured through `OTEL_TRACES_SAMPLER` and `OTEL_TRACES_SAMPLER_ARG`.

## Things to know

- The local example uses a test exporter only for demonstration; it does not show a service-side X-Ray result.
- Tracing is normally disabled outside Lambda; `WithLocalTracing(true)` opts in. `POWERTOOLS_DEV=true` disables tracing.
- Your application owns the shutdown of an injected provider. Invocation wrappers finish and flush spans, but do not shut down that provider.
- The legacy `tracer/xray` adapter is deprecated and frozen. Use OpenTelemetry for new integrations; retain the [migration warning](XRAY_MIGRATION.md).

## Next steps

See the [API and behavior notes](reference/TRACER.md) for advanced options, and [module installation](MODULES.md) for dependencies.
