---
description: "Stream Go Lambda HTTP responses using Powertools, with streaming lifecycle, response contracts and complete examples."
---

# HTTP response streaming

Return a reader and send its content through the native Go Lambda streaming adapter. Deploy through a Lambda integration that supports response streaming.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/http@v1.2.0
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.2.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## Return a server-sent event

This complete `main.go` includes the Lambda entry point, imports and setup error
logging. `httpapi` is the Powertools router; `http` is the standard `net/http`
package used for response headers.

**Input**

```http
GET /events
```

**Code**

```go
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func main() {
	appLog := logger.New(logger.WithServiceName("streaming-orders"))
	if err := runStream(); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func runStream() error {
	app := httpapi.New(httpapi.Options{})
	if err := app.Get("/events", func(request *httpapi.RequestContext) (any, error) {
		return httpapi.Response{
			StatusCode: 200,
			Headers:    http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       strings.NewReader("data: ready\n\n"),
		}, nil
	}); err != nil {
		return err
	}
	lambda.Start(httpapi.Streamify(func(ctx context.Context, event json.RawMessage, writer io.Writer) error {
		return app.ResolveStream(ctx, event, writer)
	}))
	return nil
}
```

The callback uses the Lambda SDK's `encoding/json.RawMessage` type.

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
