---
description: "Stream Go Lambda HTTP responses using Powertools, with streaming lifecycle, response contracts and complete examples."
---

# Native Go HTTP response streaming

Return a reader from your HTTP route and use `Streamify` with `app.ResolveStream`
to send the body incrementally. Routing and middleware work as in the
[buffered GET/POST example](HTTP.md#complete-example). Deploy through a
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

## Pinned behavior and representation boundaries

The reference is `@aws-lambda-powertools/event-handler@2.35.0`, specifically Router.resolveStream, the shared resolver, converters and middleware. The reference generator substitutes only Lambda's `HttpResponseStream.from` destination hook; the actual router, conversion and middleware code runs unchanged.

- The initial streaming response has `transfer-encoding: chunked`. CORS works through the ordinary middleware chain; Compress skips this transfer-encoded response by default.
- Stream bytes are raw, including binary values. Preencoded string bodies are decoded during response conversion (issue #63); stream framing adds no Base64 encoding or text UTF-8/BOM replacement.
- Only `statusCode` and `headers` are passed to the streaming metadata hook. There is no statusDescription, Base64 flag or separate cookies array.
- Every adapter joins non-cookie header values into the headers-only metadata and retains the final Set-Cookie value intact. REST/v1 no longer drops repeated or list headers during projection (issue #65). Multiple streaming cookies remain limited to the final value; buffered v1/ALB/v2 responses preserve all cookie values.
- `Response.MultiValueHeaders` appends after existing/single-value headers, rather than overwriting them. Native response reconstruction preserves repeated Set-Cookie values; the final-cookie limitation belongs to streaming metadata projection. Original streaming fixtures remain unchanged, with 36 v1 metadata corrections independently sourced from their corresponding v2 fixtures.
- JSON values are compared structurally in fixtures; JSON object ordering and equivalent escaping are not byte-parity claims. Native stream bytes and non-JSON body bytes are compared exactly.

Go's synchronous destination remains caller-owned, unlike the Node pipeline's destination end operation. Streamify closes its own pipe writer, mapping completion to reader EOF. It implements the SDK's `ContentType() string`, reader and closer contracts and deliberately rejects JSON serialization so the SDK selects its reader path.

## Verification and limits

The 272 streaming reference cases add to the existing 414 router/validation and 1,076 CORS/compression cases. They cover all four event forms, JSON/text/binary/readers, null/empty/large bodies, HEAD, unsupported methods, HTTP errors, missing routes, CORS, compression skipping and header/cookie representation.

Go lifecycle tests prove first-byte delivery before a gated second chunk, reader cancellation, write/read/close failures, error and panic identity, body cleanup, 32 concurrent streams and bounded reads for a 32 MiB generated body. They do not assert an allocation budget or substitute for performance benchmarks.

The local Docker streaming fixture runs the real `aws-lambda-go` SDK on Linux against a controlled Runtime API. Its body producer waits for the server to observe the first chunk before producing the second, so a fully buffered implementation cannot pass. It records actual Content-Type, transfer encoding, metadata delimiter, errors before output, error trailers after output, deadline and panic paths, warm recovery, closed bodies, and correlated OTel/log completion. The normal Lambda RIE composition suite remains separate. See `integration/local/stream_run.py` and the acceptance report when generated.

AWS service acceptance, integration configuration/limits, deadline/freeze behavior in AWS and exhaustive native/header/Unicode parity remain open. The pinned Go SDK's observed transport behavior, including its response-mode header, must be distinguished from the full custom-runtime specification; a local fixture is not proof of cloud behavior. Resource/performance budgets and release gates also remain open in [HTTP_PLAN.md](HTTP_PLAN.md).

Acceptance (2026-09-17, Asia/Shanghai): the real SDK/local Runtime API suite passed **95/95 checks across ten invocations**, with temporary containers/network removed. It verified chunked transport and the SDK's HTTP integration content type. The observed `Lambda-Runtime-Function-Response-Mode` header was absent in SDK v1.55.0; no header injection, SDK modification or AWS deployment was performed. This transport observation is preserved in [STREAMING_ACCEPTANCE.json](STREAMING_ACCEPTANCE.json), not normalized away or treated as cloud acceptance. Deadline traces are inspected in the final collected snapshot; the test does not promise export after an actual AWS hard timeout.

The first protocol run exposed cancellation publishing a pipe error before producer cleanup; the adapter now waits for producer completion and preserves the cancellation cause. After that fix, the focused HTTP tests and rebuilt amd64/arm64 streaming binaries passed, followed by the 95-check runtime continuation. The already-passing 609-check RIE suite was not repeated for this streaming-only correction. The two suites have distinct environments and reports; their assertion counts are not a single RIE run. The reference generator is in the standard fixture command. No third-party dependency or CGO requirement was introduced.

Final packaged verification after the cancellation fix passed all 20 module tests/vet/tidy checks and seventeen independent consumer builds with `GOWORK=off` and `CGO_ENABLED=0`. [MODULE_ACCEPTANCE.json](MODULE_ACCEPTANCE.json) was refreshed from that completed run. This final step reused both passing runtime suites instead of repeating their invocations.
