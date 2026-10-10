---
description: "Configure CORS and response compression middleware for Powertools Go HTTP event handlers, including observable response behavior."
---

# HTTP CORS and compression

Use `app.Use` to apply CORS or response compression to your HTTP routes. Both are
built into the HTTP module. Start with the [GET/POST routing example](HTTP.md#complete-example),
then add the middleware you need before serving requests.

## Usage

This fragment assumes an initialized `app` and the `httpapi` import from the routing guide:

```go
app.Use(httpapi.CORS(httpapi.CORSOptions{
    Origins: []string{"https://app.example.com"},
    AllowMethods: []string{"GET", "POST"},
}))
app.Use(httpapi.Compress(httpapi.CompressionOptions{}))
```

Put CORS before compression so preflight responses can bypass downstream processing.
The complete Lambda application is in [examples/http](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/http/main.go).

## Route-specific middleware

Pass middleware after the route handler to apply it to one route. This fragment
also uses the standard `log` import:

```go
threshold := float64(512)
err := app.Get("/large", func(request *httpapi.RequestContext) (any, error) {
    return map[string]any{"message": "response data"}, nil
}, httpapi.Compress(httpapi.CompressionOptions{
    Encoding: "deflate",
    Threshold: &threshold,
}))
if err != nil {
    log.Fatal(err)
}
```

The short sample body does not exceed the threshold; use a larger response to
observe compression.

## CORS contract

Use the options below when you need allowed headers, credentials, exposed headers
or preflight cache duration. Start with an explicit allowed origin.

| Option | Default / mapping |
| --- | --- |
| `Origin *string` | Nil means `"*"`; a pointer also permits the empty string |
| `Origins []string` | Non-nil selects the reference array form and takes precedence over `Origin`; an empty array allows no origins |
| `AllowMethods []string` | Nil means DELETE, GET, HEAD, PATCH, POST, PUT; values are uppercased |
| `AllowHeaders []string` | Nil means Authorization, Content-Type, X-Amz-Date, X-Api-Key, X-Amz-Security-Token; values are lowercased |
| `ExposeHeaders []string` | Empty by default; preserves configured spelling and order |
| `Credentials bool` | False |
| `MaxAge *float64` | Omitted by default; an explicit zero is retained |

Non-nil empty lists stay empty. Caller-owned slices and pointer values are copied before requests run. Origin callbacks are not part of the pinned public type or implementation, despite an example in its comments; they are not invented as a Go API.

An absent Origin does not receive CORS headers. Allowed origins are exact, case-sensitive matches, unless the configured string/list contains `*`. A wildcard writes `*` even with credentials enabled. A non-wildcard origin array sets `Vary: Origin`; a single origin string does not. Setting Vary replaces an earlier value rather than appending it. These are reference behaviors, not browser-policy enhancements.

OPTIONS short-circuits with 204 only when the origin is allowed, the requested method is configured, and every requested header is configured. Request header names are lowercased and trimmed; `*` in AllowHeaders is a literal allowed name, not an arbitrary-header wildcard. Invalid preflights continue routing, commonly producing 404. Successful preflights include all configured methods/headers and optional max age, not expose headers. Unsupported HTTP methods are rejected before middleware by the router.

Ordinary CORS headers are set before the next middleware/handler. A handler can overwrite them. Header values are appended for exposed headers and preflight method/header lists. Global valid preflight handling takes precedence over any route policy because it short-circuits the chain. To apply route-specific preflight policies, register an OPTIONS route and attach the policy there without a broader global policy intercepting it.

## Compression contract

`CompressionOptions.Encoding` defaults to `gzip`; `deflate` uses the zlib wrapper, matching the Web CompressionStream format. `Threshold` defaults to 1024; a pointer allows explicit zero, negative and fractional values. Compression requires the content length to be strictly greater than the threshold.

After a successful `next`, the middleware first skips an exact `Transfer-Encoding: chunked`. Otherwise it fills a missing Content-Length for any non-null body, even if the response will not be compressed. A supplied Content-Length is trusted for the threshold decision; numeric parsing reuses Commons JavaScript Number semantics. An empty header bypasses the length comparison. A null body remains distinct from a supplied empty body.

Compression is skipped for HEAD, any existing Content-Encoding/Transfer-Encoding header, a comma-separated `no-transform` Cache-Control directive, or a null body. The no-transform directive is case-insensitive; `no-transform=1` and `x-no-transform` do not match. There is no Content-Type filter in the pinned implementation.

Accept-Encoding handling intentionally corrects the pinned substring behavior using [RFC 9110 coding tokens and weights](https://www.rfc-editor.org/rfc/rfc9110.html#section-12.5.3):

- An absent header behaves as `*`; an explicitly empty header does not.
- Coding tokens are case-insensitive exact matches, so `GZIP` is gzip while `xgzipx` is not. A specific coding entry overrides `*`, including an explicit q=0 exclusion.
- A positive weight permits the configured coding. An explicit identity weight wins only when it is higher; `gzip, identity` may compress, and `identity;q=0, gzip` compresses. Without an explicit identity preference, an offered coding may be used.
- Weights follow the 0-to-1 range with up to three decimal digits; malformed or duplicate weight parameters exclude that offer. Repeated coding offers use their highest weight.
- There is no fallback from the configured encoding to another encoding and no automatic `Vary: Accept-Encoding`.

This middleware decides whether to apply compression and preserves the original response/status when it does not. Applications that reject requests excluding every available representation must enforce that policy separately.

Compression replaces the owned response body, removes Content-Length and sets Content-Encoding. The ordinary proxy conversion selects Base64 from that header. With nested compression middleware, the outer middleware may populate Content-Length from an already compressed inner body and then skip re-encoding it. Header values must describe the actual bytes emitted by that runtime.

Go uses its standard gzip/zlib implementations. Compressed bytes and their lengths can differ from Node even for identical content. This is an explicit wire representation difference: exact compressed-byte parity is not claimed. Resolve currently buffers responses; this middleware does not establish native Lambda response streaming. Read/write/close failures retain their causes; consumed bodies are closed once. Cancellation is checked before and during compression, but it cannot forcibly interrupt an application reader that blocks inside Read. Readers must cooperate with their request context where necessary.

## Evidence and remaining scope

The baseline is the [official v2.35.0 HTTP guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/event-handler/http.md) and its CORS/compression implementation. Middleware snapshots configuration for concurrent router reuse.

`tools/reference/generate-http-middleware.mjs` executes the actual pinned middleware/router and records 1,076 cases across API Gateway REST, HTTP API v2, ALB and Function URL events. It covers defaults, empty and wildcard configurations, preflight allow/deny, route policies, credentials, max-age numeric formatting, encoding/quality strings, exact thresholds, null/empty/Unicode bodies, pre-encoded and transfer-encoded responses, cache directives, errors, HEAD, and both middleware orders.

The comparison checks status, Base64 flags, headers, cookies, handler invocation counts and decoded payload bytes. For compressed bodies only, it compares decompressed bytes instead of compressor-specific wire bytes. If nested middleware supplies Content-Length, each runtime's length is checked against its own compressed bytes before comparison. Original fixtures remain unchanged; 24 named token/quality expectations are explicitly corrected from their original input bodies. Additional gzip/deflate tests cover weights, identity preference, wildcard overrides, token casing/boundaries and malformed values; the runtime also verifies gzip accepted alongside identity.

Additional Go tests exercise 64 simultaneous callers using one middleware configuration, post-construction option mutation, read/close/write failures, error and panic identity, cancellation, and exactly-once body cleanup. Original HTTP reference cases remain part of the same module tests.

Full HTTP parity remains open, including malformed native header values, Go/JavaScript Unicode case conversion edges, converter/URL/regex/error contracts and streaming. Performance/allocation limits and release gates are separate from these correctness checks. Current packaged/runtime acceptance is recorded in [LOCAL_VALIDATION.md](LOCAL_VALIDATION.md) and [HTTP_PLAN.md](HTTP_PLAN.md).

Acceptance (2026-09-17, Asia/Shanghai): all 20 packaged modules/17 consumers passed, both CGO-disabled Linux binaries built and Docker passed 609/609 assertions. The new runtime probes add 108 assertions covering all four event adapters, preflight short-circuit/fallthrough, both compression algorithms, actual compressed lengths, CORS/request identity composition, and identity/no-transform skips. An initial runtime failure exposed a development GET fixture using an empty body rather than null. After that fixture-only fix, the continuation rebuilt binaries with `--skip-module-checks`; public library source was unchanged. The same successful artifacts passed 14/14 Batch checks, with temporary containers/network removed. Docker executed amd64 only; arm64 remains cross-build evidence.
