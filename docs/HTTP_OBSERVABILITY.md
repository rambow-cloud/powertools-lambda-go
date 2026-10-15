---
description: "Add structured logging, OpenTelemetry tracing and CloudWatch metrics to Go Lambda HTTP routes with Powertools middleware."
---

# HTTP observability

Add route metrics and OpenTelemetry spans to an existing HTTP router. Each integration is an optional module.

## Count requests by route

Install `eventhandler/http/metrics@v1.2.0`. Import it as `httpmetrics` and import the root Metrics module. Run setup snippets inside a function returning `error`; the application logs a returned setup error through Logger.

**Input**

```http
GET /orders/ORD-123
```

**Code**

```go
metric, err := metrics.New(metrics.WithNamespace("Orders"))
if err != nil {
    return err
}
app.Use(httpmetrics.New(metric, httpmetrics.Options{CaptureRequestCount: true}))
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

Install `eventhandler/http/tracer@v1.2.0` and import it as `httptracer`. Given `tr` from the [Tracer guide](TRACER.md):

```go
app.Use(httptracer.New(tr, httptracer.Options{DisableCaptureResponse: true}))
```

A request produces a route span under the invocation span. Keep the outer `tracer.WrapHandler` for invocation lifetime and exporter flushing. Configure your OpenTelemetry exporter/collector as described in the Tracer guide.

**Trace effect** — span name observed with the local in-memory exporter:

```text
GET /orders/ORD-123
```

This is a span name, not a log line automatically written by the middleware. Trace IDs and timing vary.

## Add application logs

Inside a route, use your root logger's `WithContext(request.Context)` to include the active trace identity. Log only the request fields your application needs.

The [complete HTTP example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/http/main.go) combines these integrations with CORS and compression.

## Things to know

- Create root Metrics and Tracer objects once; middleware creates request scopes.
- `DisableCaptureResponse` avoids reading/parsing a body merely for trace capture.
- Route latency measures middleware execution. For streamed responses it does not include the complete body transfer.
- Tracing uses OpenTelemetry; new examples must not use the frozen X-Ray SDK adapter.

## Next steps

[API and behavior notes](reference/HTTP_OBSERVABILITY.md) · [HTTP](HTTP.md) · [Modules](MODULES.md)
