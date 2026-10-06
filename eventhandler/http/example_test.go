package http_test

import (
	"context"
	"encoding/json"
	"fmt"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
)

func ExampleRouter_Resolve() {
	app := httpapi.New(httpapi.Options{})
	if err := app.Get("/orders/:id", func(request *httpapi.RequestContext) (any, error) {
		return httpapi.Response{StatusCode: 200, Body: request.Params["id"]}, nil
	}); err != nil {
		panic(err)
	}
	event := json.RawMessage(`{"version":"2.0","routeKey":"$default","rawPath":"/orders/order-123","rawQueryString":"","headers":{},"requestContext":{"http":{"method":"GET"},"domainName":"api.example.test"},"isBase64Encoded":false}`)
	response, err := app.Resolve(context.Background(), event)
	fmt.Println(response.StatusCode, response.Body, err)
	// Output: 200 order-123 <nil>
}
