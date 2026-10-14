---
description: "Stream Go Lambda HTTP responses using Powertools, with streaming lifecycle, response contracts and complete examples."
---

# HTTP response streaming

Return a reader and send its content through the native Go Lambda streaming adapter. Deploy through a Lambda integration that supports response streaming.

## Return a server-sent event

The fragment uses the router and Lambda imports from the [HTTP guide](HTTP.md), plus `io`, `net/http` and `strings`.

**Input**

```http
GET /events
```

**Code**

```go
err := app.Get("/events", func(request *httpapi.RequestContext) (any, error) {
    return httpapi.Response{
        StatusCode: 200,
        Headers: http.Header{"Content-Type": []string{"text/event-stream"}},
        Body: strings.NewReader("data: ready\n\n"),
    }, nil
})
if err != nil { log.Fatal(err) }
lambda.Start(httpapi.Streamify(func(ctx context.Context, event json.RawMessage, writer io.Writer) error {
    return app.ResolveStream(ctx, event, writer)
}))
```

Import `encoding/json` as `json` for the callback's `RawMessage` type.

**Output**

```http
HTTP/1.1 200 OK
Content-Type: text/event-stream

data: ready

```

This shows the HTTP client's body, without the custom runtime's integration framing. Replace the string reader with a context-aware producer for multiple incremental events.

## Add observability

Place Logger/Tracer wrappers around the synchronous `ResolveStream` callback **inside** `Streamify`, so cleanup happens after body transfer. Disable response capture for a live reader.

See the [complete streaming application](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/httpstream/main.go) for composition.

## Things to know

- `ResolveStream` closes the owned response body; the destination writer stays caller-owned.
- Readers must honor cancellation or have a `Close` that interrupts a blocked read.
- An error after response output starts cannot replace the response with a new status/body.
- A buffered console invocation does not demonstrate incremental client delivery. See AWS's [supported streaming invocation paths](https://docs.aws.amazon.com/lambda/latest/dg/configuration-response-streaming.html).

## Next steps

[API and behavior notes](reference/HTTP_STREAMING.md) · [HTTP](HTTP.md) · [Modules](MODULES.md)
