---
description: "Add structured logging, OpenTelemetry tracing and CloudWatch metrics to Go Lambda HTTP routes with Powertools middleware."
---

# HTTP observability

Add route metrics and OpenTelemetry spans to an existing HTTP router. Each integration is an optional module.

## Count requests by route

Install the optional integration:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics@v1.3.0
```

Save the following as `http_metrics.go` in your HTTP application. In the routing
guide's `run` function, call `addRequestMetrics(app)` after creating the router
and return any setup error. `metrics` creates the root utility; `httpmetrics`
adapts it to HTTP requests.

**Input**

```http
GET /orders/ORD-123
```

**Code**

```go
package main

import (
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
	"github.com/rambow-cloud/powertools-lambda-go/metrics"
)

func addRequestMetrics(app *httpapi.Router) error {
	metric, err := metrics.New(metrics.WithNamespace("Orders"))
	if err != nil {
		return err
	}
	app.Use(httpmetrics.New(metric, httpmetrics.Options{CaptureRequestCount: true}))
	return nil
}
```

**Output**

```json
{
  "route": "GET /orders/:id",
  "request": 1
}
```

This is an excerpt of the emitted EMF document. It also contains the namespace, dimensions, latency, `error` and `fault` metrics. The latter two are zero for successful responses, with `error=1` for 4xx or `fault=1` for 5xx. The route template avoids a separate metric series for every order ID.

## Trace a route

Install the optional integration:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer@v1.3.0
```

Save this as `http_tracing.go`. Create `tr` once as shown in the
[Tracer guide](TRACER.md), then call `addRequestTracing(app, tr)` during setup.
`tracer` is the root utility; `httptracer` adds route instrumentation.

```go
package main

import (
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httptracer "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

func addRequestTracing(app *httpapi.Router, tr *tracer.Tracer) {
	app.Use(httptracer.New(tr, httptracer.Options{DisableCaptureResponse: true}))
}
```

A request produces a route span under the invocation span. Keep the outer `tracer.WrapHandler` for invocation lifetime and exporter flushing. Configure your OpenTelemetry exporter/collector as described in the Tracer guide.

**Trace effect** — span name observed with the local in-memory exporter:

```text
GET /orders/ORD-123
```

This is a span name, not a log line automatically written by the middleware. Trace IDs and timing vary.

## Add application logs

Save this as `http_logging.go` and call `registerLoggedOrder(app, appLog)` instead
of the routing guide's GET registration, returning any setup error. `appLog` is
the root Logger created in `main`. The route binds it to the active request/trace:

```go
package main

import (
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func registerLoggedOrder(app *httpapi.Router, appLog *logger.Logger) error {
	return app.Get("/orders/:id", func(request *httpapi.RequestContext) (any, error) {
		if err := appLog.WithContext(request.Context).Info("Get order", logger.Fields{
			"order_id": request.Params["id"],
		}); err != nil {
			return nil, err
		}
		return map[string]string{"id": request.Params["id"]}, nil
	})
}
```

Log only the request fields your application needs. The application entry point
uses Logger for returned setup errors, as shown in the routing guide.

The [complete HTTP example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/http/main.go) combines these integrations with CORS and compression.

## Things to know

- Create root Metrics and Tracer objects once; middleware creates request scopes.
- `DisableCaptureResponse` avoids reading/parsing a body merely for trace capture.
- Route latency measures middleware execution. For streamed responses it does not include the complete body transfer.
- Tracing uses OpenTelemetry; new examples must not use the frozen X-Ray SDK adapter.

## Next steps

[API and behavior notes](reference/HTTP_OBSERVABILITY.md) · [HTTP](HTTP.md) · [Modules](MODULES.md)
