# HTTP observability middleware

The optional `eventhandler/http/metrics` and `eventhandler/http/tracer` modules compose the existing Metrics and OpenTelemetry Tracer. Each has an independent manifest and release tag. HTTP core acquires neither dependency. The Tracer adapter does not import the deprecated X-Ray SDK.

```go
import (
    httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
    httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
    httptracer "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer"
    "github.com/rambow-cloud/powertools-lambda-go/metrics"
    "github.com/rambow-cloud/powertools-lambda-go/tracer"
)

metric, err := metrics.New(metrics.WithNamespace("Orders"))
if err != nil { return err }
tr, err := tracer.New()
if err != nil { return err }
app := httpapi.New(httpapi.Options{})
app.Use(httpmetrics.New(metric))
app.Use(httptracer.New(tr, httptracer.Options{DisableCaptureResponse: true}))
app.Use(httpapi.CORS(httpapi.CORSOptions{}))
app.Use(httpapi.Compress(httpapi.CompressionOptions{}))
```

The complete Lambda example is `examples/http/main.go`. Its outer Tracer wrapper owns bounded exporter flushing. The middleware creates internal child spans, never another provider or exporter.

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

## Tracer contracts and OTel mapping

The source contract is the actual TypeScript v2.35.0 HTTP tracer middleware. Its SDK representation is replaced with OTel. Internal route span names retain `METHOD /escaped/path`; `http.route` carries the template separately. The outer Lambda wrapper remains the server span.

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

Dedicated route tracing skips streaming and disabled tracers, matching the reference. Use the invocation wrapper inside `Streamify` for complete streaming lifetimes/errors/flushing; see [HTTP_STREAMING.md](HTTP_STREAMING.md).

## Evidence and remaining gates

`generate-http-metrics.mjs` executes the pinned Router and Metrics implementation for 176 cases. Only timestamps, nonnegative latency values, and dimension-key order are normalized.

`generate-http-tracer.mjs` executes the pinned Router and HTTP tracer middleware for 128 cases using a recording Tracer contract. This checks middleware calls and HTTP data, not the retired SDK. Go tests map the contract to real OTel SDK spans without dropping HTTP metadata fields. Separate tests verify that removed lengths stay absent and existing compressed lengths match actual Go wire bytes.

Concurrency, nested scope ownership, late writes, failed publication, panic identity, cancellation, response replay, parent restoration, and streaming bypass have dedicated tests. Verified 176 Metrics and 128 Tracer middleware reference cases (2,066 HTTP cases across the three modules), scope/span concurrency and body lifecycle tests, all 22 packaged modules/19 independent consumers, both CGO-disabled Linux builds, 622/622 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22, Asia/Shanghai). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used. Full HTTP and observability edge parity, AWS collector/indexing and streaming service acceptance, performance, and publication remain separate gates.
