// Package main shows GET and POST routes in a native Go Lambda function.
package main

import (
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
)

type Order struct {
	Name string `json:"name"`
}

func newRouter() (*httpapi.Router, error) {
	app := httpapi.New(httpapi.Options{})

	if err := app.Get("/orders/:id", func(request *httpapi.RequestContext) (any, error) {
		return map[string]string{"id": request.Params["id"]}, nil
	}); err != nil {
		return nil, err
	}

	if err := app.Post("/orders", func(request *httpapi.RequestContext) (any, error) {
		var order Order
		if err := json.UnmarshalRead(request.Request.Body, &order); err != nil {
			return nil, httpapi.NewHTTPError(http.StatusBadRequest, "Expected a JSON order")
		}
		if strings.TrimSpace(order.Name) == "" {
			return nil, httpapi.NewHTTPError(http.StatusBadRequest, "name is required")
		}
		// Demonstrate the response shape; this example does not persist orders.
		return httpapi.Response{StatusCode: http.StatusCreated, Body: order}, nil
	}); err != nil {
		return nil, err
	}

	return app, nil
}

func main() {
	app, err := newRouter()
	if err != nil {
		log.Fatal(err)
	}
	// RawMessage preserves the Lambda event before the router adapts it to HTTP.
	lambda.Start(func(ctx context.Context, event jsonv1.RawMessage) (httpapi.ProxyResponse, error) {
		return app.Resolve(ctx, event)
	})
}
