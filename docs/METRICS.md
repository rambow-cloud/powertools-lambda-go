---
title: CloudWatch EMF metrics for Go Lambda
description: "Emit CloudWatch Embedded Metric Format metrics from Go Lambda with Powertools Metrics, dimensions, validation and cold-start handling."
---

# Metrics

Publish custom CloudWatch metrics by writing Embedded Metric Format (EMF) JSON to your Lambda logs.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/metrics@v1.3.0
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

## Record an order count

`AddMetric` buffers a value. `Flush` writes the EMF document below to stdout. The timestamp varies.

**Input**

```text
metric: OrdersReceived
unit: Count
value: 1
```

**Code**

```go
package main

import (
	"os"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/metrics"
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
	appMetrics, err := metrics.New(
		metrics.WithNamespace("Orders"),
		metrics.WithServiceName("checkout"),
	)
	if err != nil {
		return err
	}
	if err := appMetrics.AddMetric("OrdersReceived", metrics.Count, 1); err != nil {
		return err
	}
	if err := appMetrics.Flush(); err != nil {
		return err
	}
	return nil
}
```

**Output**

```json
{
  "_aws": {
    "Timestamp": 1790812800000,
    "CloudWatchMetrics": [
      {
        "Namespace": "Orders",
        "Dimensions": [
          [
            "service"
          ]
        ],
        "Metrics": [
          {
            "Name": "OrdersReceived",
            "Unit": "Count"
          }
        ]
      }
    ]
  },
  "service": "checkout",
  "OrdersReceived": 1
}
```

## Add a dimension

Dimensions identify the time series in CloudWatch. Add them before recording a metric:

```go
if err := appMetrics.AddDimension("environment", "production"); err != nil {
    return err
}
if err := appMetrics.AddMetric("OrdersReceived", metrics.Count, 1); err != nil {
    return err
}
return appMetrics.Flush()
```

The emitted document contains `"environment":"production"`; its dimension set includes `service` and `environment`. Put request IDs in metadata instead of dimensions to avoid creating a time series per request.

## Flush automatically in Lambda

```go
handler := func(ctx context.Context, event map[string]any) (map[string]any, error) {
    if err := appMetrics.WithContext(ctx).AddMetric("OrdersReceived", metrics.Count, 1); err != nil {
        return nil, err
    }
    return map[string]any{"status": "accepted"}, nil
}
lambda.Start(metrics.WrapHandler(appMetrics, handler,
    metrics.HandlerOptions{CaptureColdStart: true}))
```

The handler returns `{"status":"accepted"}`, and the wrapper flushes the metric at invocation completion. On the first on-demand invocation it can also emit a separate `ColdStart` metric.

## Configure metrics

| Setting | Example | Purpose |
| --- | --- | --- |
| Namespace | `metrics.WithNamespace("Orders")` | Groups related metrics |
| Service | `metrics.WithServiceName("checkout")` | Adds the default service dimension |
| Environment defaults | `POWERTOOLS_METRICS_NAMESPACE`, `POWERTOOLS_SERVICE_NAME` | Configure without code changes |
| Disable emission | `POWERTOOLS_METRICS_DISABLED=true` | Suppresses output |
| Flush diagnostics | `metrics.WithErrorHandler(callback)` | Observes wrapper publication errors |

## Things to know

- CloudWatch must receive the EMF log document to extract the metric. The package does not make CloudWatch API calls.
- Reuse the root Metrics instance; call `WithContext(ctx)` inside the wrapper for request-local dimensions and values.
- Metrics are automatically flushed at capacity. A flush clears request state.
- `POWERTOOLS_DEV=true` disables metric output unless explicitly overridden.

## Next steps

See the [API and behavior notes](reference/METRICS.md) for advanced options, and [module installation](MODULES.md) for dependencies.
