package main

import (
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"testing"

	"github.com/aws/aws-lambda-go/lambda"
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
)

func TestLambdaRoutes(t *testing.T) {
	app, err := newRouter()
	if err != nil {
		t.Fatal(err)
	}
	handler := lambda.NewHandler(func(ctx context.Context, event jsonv1.RawMessage) (httpapi.ProxyResponse, error) {
		return app.Resolve(ctx, event)
	})

	for _, test := range []struct {
		name, method, path, body string
		status                   int
		key, value               string
	}{
		{"path parameter", "GET", "/orders/ORD-123", "", 200, "id", "ORD-123"},
		{"decoded parameter", "GET", "/orders/ORD%20123", "", 200, "id", "ORD 123"},
		{"JSON creation", "POST", "/orders", `{"name":"Notebook"}`, 201, "name", "Notebook"},
		{"invalid JSON", "POST", "/orders", `{`, 400, "message", "Expected a JSON order"},
		{"duplicate JSON key", "POST", "/orders", `{"name":"A","name":"B"}`, 400, "message", "Expected a JSON order"},
		{"missing field", "POST", "/orders", `{}`, 400, "message", "name is required"},
		{"blank field", "POST", "/orders", `{"name":"  "}`, 400, "message", "name is required"},
		{"unknown route", "GET", "/missing", "", 404, "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			event, err := json.Marshal(map[string]any{
				"version": "2.0", "routeKey": "$default", "rawPath": test.path,
				"rawQueryString": "", "headers": map[string]string{},
				"requestContext": map[string]any{
					"domainName": "example.test", "stage": "$default",
					"http": map[string]string{"method": test.method, "path": test.path},
				},
				"body": test.body, "isBase64Encoded": false,
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := handler.Invoke(context.Background(), event)
			if err != nil {
				t.Fatal(err)
			}
			var response httpapi.ProxyResponse
			if err := json.Unmarshal(result, &response); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != test.status || response.IsBase64Encoded {
				t.Fatalf("unexpected response: %+v", response)
			}
			if test.key != "" {
				var body map[string]any
				if err := json.Unmarshal([]byte(response.Body), &body); err != nil {
					t.Fatal(err)
				}
				if body[test.key] != test.value {
					t.Fatalf("body[%q] = %v, want %q", test.key, body[test.key], test.value)
				}
			}
		})
	}
}
