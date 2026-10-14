# HTTP event handler: API and behavior notes

Start with the [HTTP event handler guide](../HTTP.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `app` | Reusable registry; register routes and middleware before serving requests |
| `request` | One RequestContext with invocation context, native HTTP request, parameters, response and request store |
| `app.Shared` / request Store | Warm shared state versus one request's state; mutable values remain application-owned |


## Add middleware and observability

Start with the routing example above, then add features as needed:

- [Middleware](../HTTP_MIDDLEWARE.md) shows `app.Use`, route middleware, CORS and compression.
- [Observability](../HTTP_OBSERVABILITY.md) adds structured logs, per-request metrics and OpenTelemetry tracing.
- [Streaming](../HTTP_STREAMING.md) covers streaming responses and their deployment requirements.

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

Buffered responses preserve each `Set-Cookie` line verbatim, including commas inside `Expires`. V2 places these values in `cookies`; v1/ALB retain repeated values in `multiValueHeaders`, including arbitrary custom headers. Single `Date`/`Last-Modified` values remain scalar. Known non-cookie list headers still expand comma-separated single lines for v1/ALB. These correct issue #65; original fixtures retain four cookie and two custom-header expectation corrections. The [headers-only streaming contract](../HTTP_STREAMING.md) joins non-cookie values and retains only the final cookie.

`NewHTTPError` maps nine ordinary built-in status codes to reference error names. `HTTPError` exposes `Type`, `Details` and unwrapped `Cause`; validation uses its own names and 422/500 statuses. `OnError` registers an exact name, `HttpError`, or `Error` fallback. `NotFound` and `MethodNotAllowed` are conveniences. Unhandled errors return the reference 500 shape. Development mode reuses Commons environment parsing with an optional `Options.Debug` override; Go stack text differs from JavaScript. `Options.Diagnostic` receives warnings/debug messages. Error redispatch is iterative: each selected registration runs at most once per dispatch, and cycles return the default 500 response. Cancellation is checked between callbacks. The handler registry is snapshotted before dispatch; registrations changed by a callback apply to the next dispatch. Finite chains of distinct handlers retain their existing status and response behavior. Default console/ALC logging and exhaustive error inheritance remain open.


## Optional schema validation

`Validate` accepts request body/header/path/query checks and response body/header checks. `Check` returns a transformed value, structured issues and a separate operational error. This adapts the reference's Standard Schema contract without importing schema modules.

Request checks run in body/header/path/query order before the handler. Issues aggregate with the field name prefixed to their paths. Response checks run after the inner middleware/handler. JSON parse errors produce the reference validation response. Absent response bodies skip body checks. Transformed values live in `request.Valid`; requests and responses are not rewritten. Query validation uses the final value of a repeated key.

Body extraction parses `Content-Type` tokens case-insensitively, including valid parameters. `application/json` and nonempty structured `+json` subtypes are decoded before either body check, following [RFC 6839](https://datatracker.ietf.org/doc/html/rfc6839#section-3.1). Other or malformed media types remain text. Invalid JSON produces request status 422 or response status 500 before the body schema runs. Original wire bytes remain unchanged.

The [local fixture](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/integration/lambda/http.go) uses actual Parser body schemas and JSON Schema Validation for path and response checks. Applications provide these small adapters, preserving dependency isolation. Callbacks must support concurrent reuse; extraction and per-request results remain local to each invocation.


## Public contract map

The audit uses the installed package's export manifest, declarations, Router, route/error registries, converters, utilities, middleware and Store. The npm package is pinned to the upstream baseline and used only for development.

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
| cors, compress | CORS, Compress | Implemented defaults, preflight, negotiation and route policies; compressed bytes differ by runtime; see [HTTP_MIDDLEWARE.md](../HTTP_MIDDLEWARE.md) |
| metrics, tracer middleware | Optional eventhandler/http/metrics and eventhandler/http/tracer modules | Request-scoped EMF, OTel route spans, exact JSON capture and streaming policy; see [HTTP_OBSERVABILITY.md](../HTTP_OBSERVABILITY.md) |
| streamify / resolveStream | Streamify / ResolveStream | Implemented owned-reader transfer and native SDK adapter; cloud/integration/platform gates remain in [HTTP_STREAMING.md](../HTTP_STREAMING.md) |

The OpenAPI inventory audit is complete (2026-09-23): the upstream baseline exports no OpenAPI generator. The package manifest exposes HTTP, middleware and optional Metrics/Tracer middleware paths; actual runtime inspection found 27 HTTP exports, 17 Router prototype members and only compress/cors/metrics/tracer in the middleware namespace. The complete Router declarations agree with that inventory. Original HTTP-09/H-08 is retired as outside the pinned TypeScript baseline, not marked implemented. Request/response schema validation remains part of the supported HTTP contract; OpenAPI generation from another Powertools language is a separate possible extension.

