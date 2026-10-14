# Native Go HTTP response streaming: API and behavior notes

Start with the [usage guide](../HTTP_STREAMING.md) for code and results. See [compatibility and provenance](COMPATIBILITY.md) for contributor reference material.


Return a reader from your HTTP route and use `Streamify` with `app.ResolveStream`
to send the body incrementally. Routing and middleware work as in the
[buffered GET/POST example](../HTTP.md#return-an-order-as-json). Deploy through a
streaming-capable Lambda integration.

## Lambda handler

This fragment uses the `httpapi` and `lambda` imports from the routing guide,
plus `context`, `encoding/json`, `io`, `log`, `net/http` and `strings`.

```go
app := httpapi.New(httpapi.Options{})
err := app.Get("/events", func(request *httpapi.RequestContext) (any, error) {
    return httpapi.Response{
        StatusCode: 200,
        Headers: http.Header{"Content-Type": []string{"text/event-stream"}},
        Body: strings.NewReader("data: ready\n\n"),
    }, nil
})
if err != nil {
    log.Fatal(err)
}
lambda.Start(httpapi.Streamify(func(ctx context.Context, event json.RawMessage, writer io.Writer) error {
    return app.ResolveStream(ctx, event, writer)
}))
```

Replace the sample reader with a context-aware incremental producer. Configure a streaming-capable Lambda integration when deploying; a buffered invocation does not prove client-side incremental delivery. The full [native streaming example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/httpstream/main.go) includes Logger and OpenTelemetry. All builds use `CGO_ENABLED=0` and target `provided.al2023`.

AWS describes the HTTP integration response framing and midstream error trailers in its [custom runtime contract](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-custom.html). The [streaming overview](https://docs.aws.amazon.com/lambda/latest/dg/configuration-response-streaming.html) describes supported invocation paths. Actual deployment acceptance remains a separate gate; local event conversion for ALB/API Gateway v2 does not establish that those integrations deliver streamed responses.

## Ownership and lifecycle

`ResolveStream(ctx, event, destination)` writes HTTP integration metadata, an
eight-zero-byte delimiter, then the response bytes. It copies and closes the owned
body before returning. `Streamify` adapts that operation to the reader consumed by
the Go Lambda SDK; the destination remains caller-owned.

`RequestContext.IsHTTPStreaming` is true throughout streaming routing. An `io.Reader`, `io.ReadCloser`, native `*http.Response` body, or `Response.Body` reader is retained without eagerly materializing it. Byte slices and ordinary JSON values still use their natural in-memory representation. ResolveStream copies through a bounded buffer. Explicit response-body validation consumes the full body before delivery, just as the pinned validator does; header-only validation does not.

Route middleware completes its ordinary before/after-next sequence before body transfer, matching the reference. Place invocation Logger/Tracer wrappers **inside the callback passed to Streamify**, around the synchronous ResolveStream call. Wrapping the reader-returning function instead would end the invocation span before its body finishes. Disable response capture for the streaming wrapper; do not attempt to serialize a live reader as response metadata. See the example for a reusable wrapped callback with per-call event/writer inputs.

The synchronous API preserves Go errors and panic identity. A failure after metadata has been written is returned to the caller; it cannot replace the already-started HTTP response. Consumed and abandoned owned response bodies are closed, including read/write failures and panics. Cancellation closes the current owned stream once and is checked during transfer. A reader's Close should interrupt blocking Read, or the reader should otherwise observe its context. Arbitrary uncooperative application code cannot be forcibly stopped.

The adapter runs the synchronous callback in one producer goroutine. It waits for the first write, so an error before response output remains an ordinary Lambda invocation error. Later errors emerge from the returned reader. Panics unwind application/Logger/Tracer cleanup first, then become `StreamPanicError`, which retains the original value and unwraps it when it is an error. This is an explicit adaptation for the goroutine boundary, not a process-exit guarantee. A callback returning without writing a response is an error.

`ResponseStream.Close` cancels the producer, unblocks pipe writes and waits for producer cleanup. EOF/error is published only after the callback and its wrappers finish. Configuration and event snapshots remain invocation-owned; handlers and shared application values must support concurrent reuse as usual. No background producer is intentionally retained across a completed invocation.

