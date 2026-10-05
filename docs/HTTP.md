# HTTP event handler

The independent `github.com/rambow-cloud/powertools-lambda-go/eventhandler/http` module adapts API Gateway REST, HTTP API v2, ALB and Lambda Function URL events. It depends only on root Commons and the standard library. Logger, OpenTelemetry Tracer, Parser and Validation compose through the original context and explicit callbacks; ordinary routing imports none of those modules.

This is an initial implementation against Powertools TypeScript v2.35.0, not a full-parity claim. Remaining requirements are tracked in [HTTP_PLAN.md](HTTP_PLAN.md).

## Complete example

The complete handler below composes routes, CORS/compression, Logger and optional Metrics/OTel middleware. Build `./examples/http` with `CGO_ENABLED=0`. The core router also works without observability modules.

~~~go
--8<-- "examples/http/main.go"
~~~

## Input and output

Send an API Gateway v2/Function URL event such as:

~~~json
{
  "version": "2.0",
  "rawPath": "/orders/ORD-123",
  "rawQueryString": "",
  "headers": {},
  "requestContext": {
    "domainName": "example.test",
    "stage": "$default",
    "http": {
      "method": "GET",
      "path": "/orders/ORD-123",
      "protocol": "HTTP/1.1",
      "sourceIp": "127.0.0.1",
      "userAgent": "docs"
    }
  },
  "routeKey": "$default",
  "isBase64Encoded": false
}
~~~

The handler returns status 200 and the string-valued proxy `body` containing `{"id":"ORD-123"}`. Proxy headers/cookies and Base64 selection are part of the response envelope. The route additionally writes an INFO JSON record with `message: "Get order"` and `order_id: "ORD-123"`; Metrics middleware emits EMF separately. Tracing requires the configured enabled provider/collector. A response body is not itself a log record. Unknown routes produce 404; handler errors follow the registered error policy.

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `app` | Reusable registry; register routes and middleware before serving requests |
| `request` | One RequestContext with invocation context, native HTTP request, parameters, response and request store |
| `requestLog` / `requestMetrics` in this example | Root utility objects despite their variable names; binding/scoping happens per request |
| `app.Shared` / request Store | Warm shared state versus one request's state; mutable values remain application-owned |

## TypeScript feature coverage

Compared with the [official v2.35.0 HTTP guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/event-handler/http.md). The detailed [public contract map](#public-contract-map) below records core APIs and native differences.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| Route events / dynamic routes / methods | `New`, verb methods, `Handle`, `HandleRegex`, `Resolve` | API Gateway v1/v2, Function URL and ALB adapters |
| Prefixes / split routers / store | `Options.Prefix`, `IncludeRouter`, request Store and Shared | Explicit Go context and inclusion snapshots |
| Middleware / CORS / compression | `Use`, `Next`, `CORS`, `Compress` | Synchronous middleware and owned bodies; encoded bytes differ by runtime |
| Request details / data validation | `RequestContext`, `Validate`, `Check` | Explicit Standard Schema adapter without forced Parser/Validation dependencies |
| Errors / debug | `HTTPError`, `OnError`, Debug and Diagnostic options | Go errors/panics versus JS inheritance/stacks |
| Native / binary / streaming responses | Response conversion, `ResolveStream`, `Streamify` | Owned-reader transfer; live streaming platform gates remain |
| Metrics / Tracer | Separate optional HTTP observability modules | Per-request EMF and OTel scopes |
| OpenAPI | Not an implemented v2.35.0 upstream capability | Official guide labels it Coming soon; not a Go parity failure |

[Core reference tests](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/eventhandler/http/reference_test.go), middleware/streaming suites and optional module tests provide scoped evidence. See [feature comparison](FEATURE_PARITY.md), [observability](HTTP_OBSERVABILITY.md) and [streaming](HTTP_STREAMING.md).

## Native Lambda usage

```go
app := httpapi.New(httpapi.Options{})
err := app.Get("/orders/:id", func(request *httpapi.RequestContext) (any, error) {
    return map[string]any{"id": request.Params["id"]}, nil
})
if err != nil {
    log.Fatal(err)
}
handler := func(ctx context.Context, event json.RawMessage) (httpapi.ProxyResponse, error) {
    return app.Resolve(ctx, event)
}
lambda.Start(handler)
```

Import the module as `httpapi` when also using `net/http`. Create the router before `lambda.Start`. The [complete example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/http/main.go) adds Logger and OTel Tracer wrappers. Node.js is not part of the Lambda executable.

`RequestContext` retains the original `context.Context`, native `http.Request`, owned `http.Response`, JSON event snapshot, matched route and decoded parameters. Function URLs share the v2 response format. Conversion failures and cancellation remain Go errors. Handler errors become HTTP responses under the registered policy. Panics retain their identity for outer cleanup and observability wrappers.

## Routes and middleware

`Handle` and seven verb methods register string paths. `HandleRegex` accepts an anchored Go regular expression, including named captures. Static routes precede dynamic routes; dynamic routes sort by fewer parameters, then more segments, retaining registration order for ties. Explicit regex routes run last in registration order. Duplicate registrations replace the old handler. Compilation and sorting happen at registration.

Dynamic parameters use `:name` and are percent-decoded after matching. Encoded slashes can appear inside a parameter; empty or whitespace-only parameters fail route processing. Trailing request slashes are removed except for `/`. `Options.Prefix` and `IncludeRouter` accept string prefixes. Included global middleware applies to the parent, including its other routes. Routes, error handlers and shared keys are copied at inclusion time.

The pinned router has no special string `*` syntax: use a regex such as `/files/.*`. HEAD does not automatically use GET. Supported methods without a route return 404. Unsupported methods return a bare 405 before middleware and custom error handlers. Fetch normalizes six standard method names but preserves lowercase `patch`; this behavior is retained.

```go
app.Use(func(request *httpapi.RequestContext, next httpapi.Next) error {
    request.Store.Set("request_id", request.Request.Header.Get("X-Request-ID"))
    if err := next(); err != nil {
        return err
    }
    request.Response.Header.Set("X-Service", "orders")
    return nil
})
```

Global middleware runs before route middleware. Code after `next` runs in reverse order; use `defer` for error/panic cleanup. Call `request.Respond(value)` and return without `next` to short-circuit. Calling `next` twice is an error. Do not launch it in a goroutine or retain it after middleware returns; promise/await misuse detection is not a Go concurrency API.

Each invocation gets a fresh `Store`. `Router.Shared` survives warm invocations. Key access is synchronized; mutable stored values need application synchronization. Registered routes are immutable and registry access is locked. Register routes during initialization in ordinary applications.

## Responses and errors

Ordinary results become JSON with status 200. `Response` provides status, headers, cookies and body: strings are verbatim, other JSON values are encoded. Plain JSON objects also acquire proxy-response semantics when they match the reference's extended-result shape. A `statusCode` with unrelated data keys remains ordinary JSON unless `body` is present.

Native `*http.Response` values retain status and override middleware headers. Rebuilding them with middleware headers drops custom status text, matching the pinned router. Byte slices and readers are binary results. Owned readers are consumed and closed; read/close errors are not hidden. Statuses 204, 205 and 304 normalize omitted and empty buffered bodies to `http.NoBody`, skipping response body schemas. They reject nonempty buffered content and supplied streams; streaming guards remain unchanged. The pinned reference fixtures retain four explicitly corrected empty-204 expectations (issue #62). Text output uses shared UTF-8 replacement and removes a leading BOM.

Binary results, gzip/deflate encoding and image/audio/video content types select Base64 output. A plain proxy object's input `isBase64Encoded` flag is not itself honored by the pinned router; this observed behavior is retained. Middleware can explicitly set `RequestContext.IsBase64Encoded`. Cookie and allowlisted multi-value header splitting follows the reference, including comma splitting of cookie Expires values.

`NewHTTPError` maps nine ordinary built-in status codes to reference error names. `HTTPError` exposes `Type`, `Details` and unwrapped `Cause`; validation uses its own names and 422/500 statuses. `OnError` registers an exact name, `HttpError`, or `Error` fallback. `NotFound` and `MethodNotAllowed` are conveniences. Unhandled errors return the reference 500 shape. Development mode reuses Commons environment parsing with an optional `Options.Debug` override; Go stack text differs from JavaScript. `Options.Diagnostic` receives warnings/debug messages. Default console/ALC logging and exhaustive error inheritance/recursion remain open.

## Optional schema validation

`Validate` accepts request body/header/path/query checks and response body/header checks. `Check` returns a transformed value, structured issues and a separate operational error. This adapts the reference's Standard Schema contract without importing schema modules.

Request checks run in body/header/path/query order before the handler. Issues aggregate with the field name prefixed to their paths. Response checks run after the inner middleware/handler. JSON parse errors produce the reference validation response. Absent response bodies skip body checks. Transformed values live in `request.Valid`; requests and responses are not rewritten. Query validation uses the final value of a repeated key.

Body extraction parses `Content-Type` tokens case-insensitively, including valid parameters. `application/json` and nonempty structured `+json` subtypes are decoded before either body check, following [RFC 6839](https://datatracker.ietf.org/doc/html/rfc6839#section-3.1). Other or malformed media types remain text. Invalid JSON produces request status 422 or response status 500 before the body schema runs. Original wire bytes remain unchanged.

The [local fixture](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/integration/lambda/http.go) uses actual Parser body schemas and JSON Schema Validation for path and response checks. Applications provide these small adapters, preserving dependency isolation. Callbacks must support concurrent reuse; extraction and per-request results remain local to each invocation.

## Public contract map

The audit uses the installed package's export manifest, declarations, Router, route/error registries, converters, utilities, middleware and Store. The npm package is pinned to 2.35.0 and used only for development.

| Reference | Go mapping | Remaining boundary |
| --- | --- | --- |
| Router, route/verb methods, resolve | New, Handle/verb methods, Resolve | Decorator binding and complete defaults/options |
| includeRouter, prefix, shared/context | IncludeRouter, Options.Prefix, Shared and explicit context | Regex prefixes and legacy context object |
| use, composeMiddleware | Use, route middleware, synchronous Next | Standalone composition and async misuse diagnostics |
| Three public converters | ProxyEventToWebRequest, HandlerResultToWebResponse, WebResponseToProxyResult | Complete options/native-value forms |
| Event/proxy/method guards | IsAPIGatewayProxyEventV1/V2, IsALBEvent, IsExtendedAPIGatewayProxyResult, IsHTTPMethod | Exhaustive malformed/non-JSON values |
| HttpVerbs, HttpStatusCodes | Standard net/http constants | No duplicated constant hierarchy |
| HTTP error classes | HTTPError, NewHTTPError, named OnError policies | Complete class/native error mapping |
| RouteMatchingError, ParameterValidationError | Registration errors and ParameterValidationError | Complete structured diagnostics/warnings |
| Store | Store, request.Store, Router.Shared | Go string keys replace arbitrary Map keys |
| Route validation / Standard Schema | Validate, Check, ValidationIssue, Valid | Body consumption, exceptions and native transforms |
| cors, compress | CORS, Compress | Implemented defaults, preflight, negotiation and route policies; compressed bytes differ by runtime; see [HTTP_MIDDLEWARE.md](HTTP_MIDDLEWARE.md) |
| metrics, tracer middleware | Optional eventhandler/http/metrics and eventhandler/http/tracer modules | Request-scoped EMF, OTel route spans, exact JSON capture and streaming policy; see [HTTP_OBSERVABILITY.md](HTTP_OBSERVABILITY.md) |
| streamify / resolveStream | Streamify / ResolveStream | Implemented owned-reader transfer and native SDK adapter; cloud/integration/platform gates remain in [HTTP_STREAMING.md](HTTP_STREAMING.md) |

The OpenAPI inventory audit is complete (2026-09-23): v2.35.0 exports no OpenAPI generator. The package manifest exposes HTTP, middleware and optional Metrics/Tracer middleware paths; actual runtime inspection found 27 HTTP exports, 17 Router prototype members and only compress/cors/metrics/tracer in the middleware namespace. The complete Router declarations agree with that inventory. Original HTTP-09/H-08 is retired as outside the pinned TypeScript baseline, not marked implemented. Request/response schema validation remains part of the supported HTTP contract; OpenAPI generation from another Powertools language is a separate possible extension.

## Evidence and limits

Observability acceptance: Verified 176 Metrics and 128 Tracer middleware reference cases (2,066 HTTP cases across the three modules), scope/span concurrency and body lifecycle tests, all 22 packaged modules/19 independent consumers, both CGO-disabled Linux builds, 622/622 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22, Asia/Shanghai). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used. See [HTTP_OBSERVABILITY.md](HTTP_OBSERVABILITY.md). The following paragraphs retain earlier milestone counts.

Streaming adds 272 reference cases (1,762 HTTP total), ResolveStream/Streamify and 95/95 real-Go-SDK/local-Runtime-API Docker assertions. The 609/609 RIE suite remains separate. See [HTTP_STREAMING.md](HTTP_STREAMING.md) for owned-reader transfer, framing, cancellation, callback placement and cloud/platform gates. The paragraphs below preserve earlier milestone counts.

The generator executes the actual package for 414 cases covering four event forms, methods, specificity, parameters, prefixes, inclusion, duplicate/invalid routes, request fields, decoding, plain/native/proxy/binary results, errors, middleware and validation. JSON bodies are compared as decoded values, not bytes; object-key order, whitespace and equivalent JSON spellings are outside that comparison. Text/binary bodies, headers, cookies, status values, array order and error fields are compared directly. Go maps serialize deterministically; raw JSON and structs can preserve deliberate field order. Invalid-event errors retain a Go diagnostic rather than the reference's empty default message. The verified conversion TypeError maps to RequestConversionError with the same message.

Functional tests cover 64 concurrent callers, state isolation, context identity, snapshots, repeated calls, cancellation, panic identity, middleware cleanup and response-body ownership. Parser's existing reference cases cover extraction of permissive Base64 and UTF-8 decoders into Commons. Strict `FromBase64` behavior is unchanged. CGO remains disabled; functional concurrency is not a race-detector claim.

Acceptance (2026-09-16, Asia/Shanghai): all 20 packaged modules passed tests/vet/tidy, seventeen consumers built with `GOWORK=off`, and both Linux architecture builds passed. Docker passed 501/501 assertions across five invocations with temporary resources cleaned; 14/14 Batch checks reused the same artifacts. The complete run executed module checks and builds. Docker ran amd64; arm64 was cross-compiled only. Root Commons, Parser and HTTP have no third-party module dependencies. No AWS resources were used. See [MODULE_ACCEPTANCE.json](MODULE_ACCEPTANCE.json), [LOCAL_ACCEPTANCE.json](LOCAL_ACCEPTANCE.json) and [BATCH_ACCEPTANCE.json](BATCH_ACCEPTANCE.json).

CORS/compression acceptance (2026-09-17, Asia/Shanghai): 1,076 additional reference cases bring HTTP coverage to 1,490 cases. Both built-ins reuse the existing middleware API and add no third-party dependencies. All 20 packaged modules/17 consumers passed; after correcting an absent-body integration fixture, both Linux binaries were rebuilt and Docker passed 609/609. The runtime continuation reused completed module checks; 14/14 Batch checks use the same saved artifacts. See [HTTP_MIDDLEWARE.md](HTTP_MIDDLEWARE.md) for configuration, reference quirks, compression representation differences and body ownership.

Complete WHATWG URL/Headers/body behavior, native objects, JavaScript-only regex syntax/flags, malformed escaped URLs, regex prefixes, async middleware, request logging, error recursion, resource budgets, cloud streaming, release and live-service acceptance remain open. Go request streams expose native bytes; Web API convenience-method differences require explicit adapters where needed.
