# HTTP CORS and compression: API and behavior notes

Start with the [usage guide](../HTTP_MIDDLEWARE.md) for code and results. See [compatibility and provenance](COMPATIBILITY.md) for contributor reference material.


Use `app.Use` to apply CORS or response compression to your HTTP routes. Both are
built into the HTTP module. Start with the [GET/POST routing example](../HTTP.md#return-an-order-as-json),
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

