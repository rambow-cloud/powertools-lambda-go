---
description: "Configure CORS and response compression middleware for Powertools Go HTTP event handlers, including observable response behavior."
---

# HTTP CORS and compression

Add cross-origin response headers and compress large HTTP responses. These middleware are included in the HTTP module.

## Install

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/http@v1.3.0
```

Both examples include their imports and belong to the same `package main` as your
[HTTP application](HTTP.md#return-an-order-as-json). The `httpapi` alias names this
project's router; it is separate from the standard library's `net/http`.

## Allow a browser origin

Save this as `cors.go` and call `addCORS(app)` after creating the router, before
`lambda.Start`.

**Input**

```http
OPTIONS /orders
Origin: https://app.example.com
Access-Control-Request-Method: POST
Access-Control-Request-Headers: content-type
```

**Code**

```go
package main

import (
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
)

func addCORS(app *httpapi.Router) {
	app.Use(httpapi.CORS(httpapi.CORSOptions{
		Origins:      []string{"https://app.example.com"},
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{"Content-Type"},
	}))
}
```

**Output**

```http
HTTP/1.1 204 No Content
Access-Control-Allow-Origin: https://app.example.com
Vary: Origin
Access-Control-Allow-Methods: GET, POST
Access-Control-Allow-Headers: content-type
```

The response above shows the preflight status and CORS headers. An ordinary allowed-origin GET also receives the CORS headers. Requests without an Origin header do not receive them.

## Compress a response

Save this as `compression.go` and call `addCompression(app)` after `addCORS(app)`,
before starting Lambda.

```go
package main

import (
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
)

func addCompression(app *httpapi.Router) {
	app.Use(httpapi.Compress(httpapi.CompressionOptions{}))
}
```

For a response larger than 1,024 bytes and an acceptable `Accept-Encoding: gzip`, the HTTP response gains `Content-Encoding: gzip`. The Lambda proxy envelope carries a Base64-encoded compressed body; the HTTP client receives compressed bytes.

Use a `Threshold` pointer to change the minimum size, or `Encoding: "deflate"` for zlib-wrapped deflate.

## Things to know

- Put CORS before compression so valid preflight requests can short-circuit.
- Use explicit origins when enabling credentials; review your browser access policy.
- HEAD, pre-encoded, chunked and `Cache-Control: no-transform` responses are not compressed.
- Compression buffers ordinary responses. Use the [streaming adapter](HTTP_STREAMING.md) for streaming.

## Next steps

[API and behavior notes](reference/HTTP_MIDDLEWARE.md) · [HTTP](HTTP.md) · [Modules](MODULES.md)
