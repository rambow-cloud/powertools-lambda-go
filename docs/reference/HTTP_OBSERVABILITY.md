# HTTP observability middleware: API and behavior notes

Start with the [usage guide](../HTTP_OBSERVABILITY.md) for code and results. See [compatibility and provenance](COMPATIBILITY.md) for contributor reference material.


Add request metrics or OpenTelemetry tracing to an existing HTTP router with
`app.Use`. Both integrations are optional. Start with [basic routing](../HTTP.md#return-an-order-as-json)
and choose the middleware your application needs.

## Request metrics

Install the metrics middleware:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics@v1.2.0
```

Save this as `http_metrics.go` beside your HTTP entry point. Call
`addLatencyMetrics(app)` once during setup and handle the returned error. Imports
for the router, root Metrics utility and HTTP adapter are included:

```go
package main

import (
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
	"github.com/rambow-cloud/powertools-lambda-go/metrics"
)

func addLatencyMetrics(app *httpapi.Router) error {
	metric, err := metrics.New(metrics.WithNamespace("Orders"))
	if err != nil {
		return err
	}
	app.Use(httpmetrics.New(metric))
	return nil
}
```

Each request emits latency and error/fault metrics. To also count requests, use
`httpmetrics.Options{CaptureRequestCount: true}`; see [request count](#optional-request-count).

## Request tracing

Install the tracer middleware:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer@v1.2.0
```

Save this as `http_tracing.go` and call `addRequestTracing(app, tr)` during setup.
Create `tr` once as shown in the [Tracer guide](../TRACER.md#see-a-handler-span-locally).
The root utility and HTTP adapter have separate imports:

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

The middleware creates route spans. Keep the outer `tracer.WrapHandler` to own the
Lambda invocation span and exporter flush, and configure an OTLP collector.

## Complete application

The maintained [HTTP application](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/http/main.go)
combines logging, request metrics, tracing, CORS and compression. The full source,
including every import and the Lambda entry point, is included here:

```go
--8<-- "examples/http/main.go"
```

The root objects below are called `metric` and `tr` in the contract discussion.
Create each once; request-specific state is supplied by middleware. The HTTP core
does not acquire these dependencies and the tracer adapter uses OpenTelemetry.

## Metrics contracts

- Each request creates a fresh scope through `Metrics.StartScope`, sharing the writer/configuration and inheriting active default dimensions. Pending metrics, temporary dimensions, and metadata are not copied from the parent.
- Downstream code uses `metric.WithContext(request.Context)`. Both request contexts carry this scope and are restored on success, error, and panic.
- The scope publishes once at middleware completion and rejects late writes. An outer Lambda Metrics wrapper retains separate invocation metrics. This explicit isolation adapts the TypeScript singleton model for concurrent Go requests.
- Metrics are `latency` in Milliseconds, `fault` in Count for status >= 500, and `error` in Count for 400 <= status < 500. The route dimension uses matched `METHOD /template`, or `NOT_FOUND`; raw paths are metadata only.
- Metadata includes method, escaped path without query, string status code, optional user agent/client IP, and available API Gateway identifiers. Null identifiers remain null; absent identifiers are omitted. Function URLs follow API Gateway v2 behavior.
- API Gateway source IP comes from event context; ALB uses the first forwarded address. Both middleware modules reuse `RequestContext.ClientIP`.
- Returned HTTP errors contribute their status; ordinary errors and panics contribute 500. Error conversion outside middleware can subsequently produce a different final response status, as in the reference.
- Recording/publication errors propagate. Go retains simultaneous business and metric failures with `errors.Join`; panics keep their original value. Direct `StartScope` callers must call its idempotent finish function and handle the flush error.
- Streaming metrics measure middleware/route execution before body transfer. They do not claim full transfer latency or capture later stream failures.

### Optional request count

Enable an explicit request counter when application dashboards need request volume
or a denominator for error/fault ratios. Save this as `request_count.go` and call
`addRequestCount(app, metric)` instead of the metrics registration above. `metric`
is a root `*metrics.Metrics` created during setup:

```go
package main

import (
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
	"github.com/rambow-cloud/powertools-lambda-go/metrics"
)

func addRequestCount(app *httpapi.Router, metric *metrics.Metrics) {
	app.Use(httpmetrics.New(metric, httpmetrics.Options{CaptureRequestCount: true}))
}
```

The middleware adds `request=1` with unit `Count` to its existing request scope at
completion, including HTTP errors, ordinary failures, panics and unmatched routes.
It reuses the matched route template or `NOT_FOUND`, namespace and default
dimensions. The route is a scope-local default when counting is enabled, so it
survives the Metrics automatic flush limit and single-metric publication mode.
Disabled Metrics instances publish nothing. Streaming counts middleware
execution before body transfer; it does not count successful transfer completion.
An outer Lambda Metrics wrapper retains its own invocation scope.

Use CloudWatch `Sum` to aggregate published request samples. This is not a unique
client-request count: Lambda retries and EMF's at-least-once delivery can repeat
samples, while failed publication can lose them. Reserve the `request` metric and
`route` dimension for the middleware instead of overriding them downstream. Each
dimension combination adds a custom metric and can increase CloudWatch costs.

`New(metric)` and `Options{}` retain the three-metric TypeScript the upstream baseline contract.
Request counting is an intentional opt-in Go extension. Direct constructor calls
remain valid; code assigning `New` to an explicitly nonvariadic function type must
adapt to its optional `...Options` argument. HTTP core and core Metrics gain no
dependencies or runtime changes.

## Tracer contracts and OTel mapping

The source contract is the actual TypeScript the upstream baseline HTTP tracer middleware. Its SDK representation is replaced with OTel. Internal route span names retain `METHOD /escaped/path`; `http.route` carries the template separately. The outer Lambda wrapper remains the server span.

| Reference field | OTel attribute |
| --- | --- |
| `request.method` | `http.request.method` |
| `request.url`, without query | `url.full`, plus `url.path`, `url.scheme`, `server.address` |
| `request.user_agent` | `user_agent.original` |
| `request.client_ip` | `client.address` |
| `request.x_forwarded_for` | `powertools.http.x_forwarded_for` |
| `response.status` | `http.response.status_code` |
| `response.content_length` | `http.response.body.size`, when nonnegative and representable as int64 |

HTTP attribute names follow the [OpenTelemetry HTTP semantic conventions](https://opentelemetry.io/docs/specs/semconv/http/http-spans/). The reference-shaped request/response object is also retained as `http` metadata through Tracer's namespace encoder. Negative and decimal-prefix Content-Length values are retained in reference metadata; invalid/overflow boundaries remain in the full parity audit.

`Tracer.AnnotateInvocation` reuses shared cold-start identity and service annotations. `Tracer.AddResponseAsMetadata` applies existing capture configuration. Both helpers are reused by the original invocation/operation wrappers. No new X-Ray SDK behavior is maintained.

Returned errors and panics close spans using existing Tracer error-capture policy. Explicit 5xx responses set OTel error status; ordinary 4xx responses do not. Both request contexts are restored. The application owns flushing and provider shutdown.

Response capture defaults to the exact Content-Type `application/json`; charset variants are skipped. Bytes are replayed unchanged and owned readers close once. Invalid JSON propagates an error. `DisableCaptureResponse` bypasses parsing. Tracer-wide response capture configuration controls metadata storage, while middleware parsing follows its own option.

Tracer before Compress observes the headers that compression leaves behind. Actual compression deletes Content-Length, so tracing then omits body size. A length can remain when compression is skipped, including an outer compressor measuring a body already compressed by route middleware. JSON capture after compression reads compressed bytes and can fail. Disable capture in this order, as in the example. Tracer inside Compress captures JSON before compression; its attributes describe the response at that stage.

The [finite Tracer audit](TRACER.md#finite-lifecycle-and-capture-audit-182) verifies
the independent parsing/storage controls with valid and invalid JSON bodies,
including body replay and exact close ownership. It also records OTel lifecycle,
provider ownership and capture-policy acceptance separately from cloud delivery.

Dedicated route tracing skips streaming and disabled tracers, matching the reference. Use the invocation wrapper inside `Streamify` for complete streaming lifetimes/errors/flushing; see [HTTP_STREAMING.md](../HTTP_STREAMING.md).

