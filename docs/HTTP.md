---
description: "Route API Gateway and ALB events in Go Lambda using Powertools HTTP handlers, typed requests, responses and middleware."
---

# HTTP event handler

Route API Gateway, ALB and Lambda Function URL requests with path parameters, JSON responses and middleware.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/http@v1.2.0
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.2.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

Import the routing module as `httpapi` to distinguish it from the standard library's `net/http`.

## Return an order as JSON

Register a GET route once, then pass Lambda events to `Resolve`. Connect the function to an HTTP proxy integration or a Function URL.

**Input**

```http
GET /orders/ORD-123
```

**Code**

```go
package main

import (
	"context"
	"encoding/json/jsontext"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	if err := run(appLog); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func run(appLog *logger.Logger) error {
	app := httpapi.New(httpapi.Options{})
	err := app.Get("/orders/:id", func(request *httpapi.RequestContext) (any, error) {
		return map[string]string{"id": request.Params["id"]}, nil
	})
	if err != nil {
		return err
	}
	lambda.Start(func(ctx context.Context, event jsontext.Value) (httpapi.ProxyResponse, error) {
		return app.Resolve(ctx, event)
	})
	return nil
}
```

**Output**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{"id":"ORD-123"}
```

## Read JSON and return a status code

```go
err := app.Post("/orders", func(request *httpapi.RequestContext) (any, error) {
    var order struct { Name string `json:"name"` }
    if err := json.UnmarshalRead(request.Request.Body, &order); err != nil {
        return nil, httpapi.NewHTTPError(400, "Expected a JSON order")
    }
    if order.Name == "" {
        return nil, httpapi.NewHTTPError(400, "name is required")
    }
    return httpapi.Response{StatusCode: 201, Body: order}, nil
})
```

Handle the registration error during initialization. This example uses `encoding/json/v2`.

**Request → response**

```text
POST /orders  {"name":"Notebook"}  →  201 {"name":"Notebook"}
POST /orders  {}                   →  400, "name is required"
```

## Read request values

| Value | Go expression |
| --- | --- |
| Path parameter | `request.Params["id"]` |
| Query string | `request.Request.URL.Query().Get("status")` |
| Header | `request.Request.Header.Get("Authorization")` |
| JSON body | `json.UnmarshalRead(request.Request.Body, &value)` |

## Test with a Lambda event

For API Gateway v2 or a Function URL, a console test can use:

```json
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
```

Lambda returns a proxy envelope whose `body` is a JSON string. The HTTP caller receives the decoded body shown above.

## Add more behavior

- [CORS and compression](HTTP_MIDDLEWARE.md).
- [Request logs, metrics and traces](HTTP_OBSERVABILITY.md).
- [Streaming responses](HTTP_STREAMING.md).

## Things to know

- Check route-registration errors before starting Lambda.
- A missing route returns a not-found response. Use `NewHTTPError` for expected HTTP failures.
- Reuse the router; keep per-request values in the request/context rather than shared mutable variables.

## Next steps

See the [API and behavior notes](reference/HTTP.md) for advanced options, and [module installation](MODULES.md) for dependencies.
