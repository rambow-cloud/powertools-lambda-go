---
description: "Route API Gateway and ALB events in Go Lambda using Powertools HTTP handlers, typed requests, responses and middleware."
---

# HTTP event handler

Build HTTP APIs with `app.Get`, `app.Post`, `app.Put`, `app.Patch`, `app.Delete`, `app.Head` and `app.Options`. Each method takes a path and a handler. Return a Go value for a JSON response, or an `httpapi.Response` to choose the status code and headers. These methods are already supported; their capitalized names follow Go's exported-method convention.

The independent `github.com/rambow-cloud/powertools-lambda-go/eventhandler/http` module adapts API Gateway REST, HTTP API v2, ALB and Lambda Function URL events. It depends only on root Commons and the standard library. Its compatibility baseline is Powertools TypeScript v2.35.0; [HTTP_PLAN.md](HTTP_PLAN.md) records the supported scope and remaining requirements.

## Install

Use Go 1.27 or newer in your application's Go module:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/http@v1.1.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

Import the routing module as `httpapi` to distinguish it from the standard library's `net/http`.

## Complete example

This Lambda has two routes:

- `GET /orders/:id` returns the decoded path parameter as JSON with status 200.
- `POST /orders` reads a JSON body and returns it with status 201, or returns 400 for invalid input. It demonstrates the response shape without saving an order.

Create the router once, register the routes, then call `lambda.Start`. Registration returns an error for invalid route definitions, so check it during initialization.

~~~go
--8<-- "examples/http-routing/main.go"
~~~

The small Lambda callback passes the raw event to `app.Resolve`, which converts the Lambda event into an HTTP request and converts your result into a proxy response. You do not need to write those conversions yourself.

Build this example from the repository root for `provided.al2023`:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -tags lambda.norpc -o bootstrap ./examples/http-routing
```

Use `GOARCH=amd64` for x86_64. In your own application, replace `./examples/http-routing` with your main package. Connect an API Gateway proxy integration or a Function URL to the Lambda so that requests reach the router.

## Input and output

At the HTTP boundary, the example behaves as follows:

| Request | Status | JSON body |
| --- | --- | --- |
| `GET /orders/ORD-123` | 200 | `{"id":"ORD-123"}` |
| `POST /orders` with `{"name":"Notebook"}` | 201 | `{"name":"Notebook"}` |
| `POST /orders` with malformed JSON | 400 | An error with message `Expected a JSON order` |
| `POST /orders` with `{}` | 400 | An error with message `name is required` |
| `GET /missing` | 404 | A not-found error |

For a Lambda console test, use this API Gateway v2/Function URL event:

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

The Lambda result has status 200 and a string-valued proxy `body` containing `{"id":"ORD-123"}`. The HTTP client receives that body as JSON. To test POST, set `rawPath` and `requestContext.http.path` to `/orders`, set `requestContext.http.method` to `POST`, and add `"body": "{\"name\":\"Notebook\"}"` at the top level. API Gateway passes the request body as a JSON string inside the event.

This routing example does not configure logs, metrics or tracing. Add them with the [observability guide](HTTP_OBSERVABILITY.md) when needed.

## Read a request and return a response

| Task | API | Example |
| --- | --- | --- |
| Read a path parameter | `request.Params` | `request.Params["id"]` for `/orders/:id` |
| Read a query parameter | Standard `net/http` URL | `request.Request.URL.Query().Get("status")` |
| Read a header | Standard `net/http` headers | `request.Request.Header.Get("Authorization")` |
| Read a JSON body | Go JSON v2 | `json.UnmarshalRead(request.Request.Body, &order)` |
| Return JSON with status 200 | Return a Go value | `return map[string]string{"id": "ORD-123"}, nil` |
| Return a different status | `httpapi.Response` | `return httpapi.Response{StatusCode: 201, Body: order}, nil` |
| Return an HTTP error | `httpapi.NewHTTPError` | `return nil, httpapi.NewHTTPError(400, "name is required")` |

The router owns the request body and closes it after the request. Decode it once in your handler. The example checks the required field explicitly; [schema validation](#optional-schema-validation) is optional. Ordinary Go errors become status 500 responses; use `NewHTTPError` for an intentional client error.

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `app` | Reusable registry; register routes and middleware before serving requests |
| `request` | One RequestContext with invocation context, native HTTP request, parameters, response and request store |
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

## Add middleware and observability

Start with the routing example above, then add features as needed:

- [Middleware](HTTP_MIDDLEWARE.md) shows `app.Use`, route middleware, CORS and compression.
- [Observability](HTTP_OBSERVABILITY.md) adds structured logs, per-request metrics and OpenTelemetry tracing.
- [Streaming](HTTP_STREAMING.md) covers streaming responses and their deployment requirements.

The [composed example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/http/main.go) combines those utilities. The [minimal example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/http-routing/main.go) imports the routing module and Lambda runtime only; it does not require observability modules.

## Native Lambda usage

Keep registration outside the invocation callback as shown in the complete example. A route handler receives an HTTP request; the Lambda callback receives a Lambda event. `app.Resolve` connects the two. Node.js is not part of the Lambda executable.

`RequestContext` retains the original `context.Context`, native `http.Request`, owned `http.Response`, JSON event snapshot, matched route and decoded parameters. Function URLs share the v2 response format. Conversion failures and cancellation remain Go errors. Handler errors become HTTP responses under the registered policy. Panics retain their identity for outer cleanup and observability wrappers.

## Routes and middleware

`Handle` and seven verb methods register string paths. `HandleRegex` accepts an anchored Go regular expression, including named captures. Static routes precede dynamic routes; dynamic routes sort by fewer parameters, then more segments, retaining registration order for ties. Explicit regex routes run last in registration order. Duplicate registrations replace the old handler. Compilation and sorting happen at registration.

Dynamic parameters use `:name` and are percent-decoded after matching. Encoded slashes can appear inside a parameter; empty or whitespace-only parameters fail route processing. Trailing request slashes are removed except for `/`. `Options.Prefix` and `IncludeRouter` accept string prefixes. Included global middleware applies to the parent, including its other routes. Routes, error handlers and shared keys are copied at inclusion time.

The pinned router has no special string `*` syntax: use a regex such as `/files/.*`. HEAD does not automatically use GET. Supported methods without a route return 404. Unsupported methods return a bare 405 before middleware and custom error handlers. Fetch normalizes six standard method names but preserves lowercase `patch`; this behavior is retained.

All adapters discard GET/HEAD request bodies after method normalization and before Base64 decoding, including empty or encoded bodies. The request exposes `http.NoBody`, zero content length and no replay body; supplied headers and the original event snapshot remain intact. REST v1 also accepts an omitted body. POST/PATCH bodies retain their bytes and replay behavior.

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

Ordinary results become JSON with status 200. `Response` provides status, headers, cookies and body: strings are verbatim unless explicitly marked as preencoded, and other JSON values are encoded with `encoding/json/v2` defaults. Struct bodies retain zero/false fields tagged `omitempty`; use `omitzero` to omit Go zero values. Nil slices and maps become `[]` and `{}`. Raw event snapshots reject duplicate members and invalid Unicode. Plain JSON objects also acquire proxy-response semantics when they match the reference's extended-result shape. A `statusCode` with unrelated data keys remains ordinary JSON unless `body` is present.

Native `*http.Response` values retain status and override middleware headers. Rebuilding them with middleware headers drops custom status text, matching the pinned router. Byte slices and readers are binary results. Owned readers are consumed and closed; read/close errors are not hidden. Statuses 204, 205 and 304 normalize omitted and empty buffered bodies to `http.NoBody`, skipping response body schemas. They reject nonempty buffered content and supplied streams; streaming guards remain unchanged. The pinned reference fixtures retain four explicitly corrected empty-204 expectations (issue #62). Text output uses shared UTF-8 replacement and removes a leading BOM.

Binary bytes/readers (including `Response.Body`), buffered native bodies containing invalid UTF-8, gzip/deflate encoding, and image/audio/video/PDF/octet-stream media types select Base64 output. Media tokens are parsed case-insensitively with valid parameters. Other binary media containing valid UTF-8 can use explicit encoding.

`Response.IsBase64Encoded` is optional: nil uses automatic selection; true interprets string bodies as standard Base64 and keeps bytes/readers/JSON values raw; false selects text and overrides inference. Plain proxy objects honor their `isBase64Encoded` flag. Preencoded strings are decoded to raw web-response bytes, then encoded once on buffered proxy output; malformed Base64 fails conversion. Streaming writes the raw bytes. These correct the pinned reference defects tracked in issues #63/#64; four malformed-preencoded fixture expectations are explicitly corrected without editing the original fixtures.

`RequestContext.IsBase64Encoded` controls final buffered output. An explicit false survives binary inference; middleware can override selection after the handler. `WebResponseToProxyResult` continues to follow its explicit encoding argument. Text output still applies UTF-8 replacement and BOM removal when selected.

Buffered responses preserve each `Set-Cookie` line verbatim, including commas inside `Expires`. V2 places these values in `cookies`; v1/ALB retain repeated values in `multiValueHeaders`, including arbitrary custom headers. Single `Date`/`Last-Modified` values remain scalar. Known non-cookie list headers still expand comma-separated single lines for v1/ALB. These correct issue #65; original fixtures retain four cookie and two custom-header expectation corrections. The [headers-only streaming contract](HTTP_STREAMING.md) joins non-cookie values and retains only the final cookie.

`NewHTTPError` maps nine ordinary built-in status codes to reference error names. `HTTPError` exposes `Type`, `Details` and unwrapped `Cause`; validation uses its own names and 422/500 statuses. `OnError` registers an exact name, `HttpError`, or `Error` fallback. `NotFound` and `MethodNotAllowed` are conveniences. Unhandled errors return the reference 500 shape. Development mode reuses Commons environment parsing with an optional `Options.Debug` override; Go stack text differs from JavaScript. `Options.Diagnostic` receives warnings/debug messages. Error redispatch is iterative: each selected registration runs at most once per dispatch, and cycles return the default 500 response. Cancellation is checked between callbacks. The handler registry is snapshotted before dispatch; registrations changed by a callback apply to the next dispatch. Finite chains of distinct handlers retain their existing status and response behavior. Default console/ALC logging and exhaustive error inheritance remain open.

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

Complete WHATWG URL/Headers/body behavior, native objects, JavaScript-only regex syntax/flags, malformed escaped URLs, regex prefixes, async middleware, request logging, resource budgets, cloud streaming, release and live-service acceptance remain open. Go request streams expose native bytes; Web API convenience-method differences require explicit adapters where needed.
