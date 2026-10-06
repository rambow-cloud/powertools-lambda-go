package tracer_test

import (
	"context"
	"encoding/json"
	"fmt"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httptracer "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func ExampleNew() {
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	t, err := tracer.New(tracer.WithServiceName("orders"),
		tracer.WithBackend(tracer.NewOTelBackend(provider)), tracer.WithLocalTracing(true))
	if err != nil {
		panic(err)
	}
	app := httpapi.New(httpapi.Options{})
	app.Use(httptracer.New(t))
	// Environment flags can disable tracing without changing the HTTP response.
	if err := app.Get("/health", func(*httpapi.RequestContext) (any, error) {
		return "ok", nil
	}); err != nil {
		panic(err)
	}
	event := json.RawMessage(`{"version":"2.0","routeKey":"$default","rawPath":"/health","rawQueryString":"","headers":{},"requestContext":{"http":{"method":"GET"},"domainName":"api.example.test"},"isBase64Encoded":false}`)
	response, err := app.Resolve(context.Background(), event)
	fmt.Println(response.StatusCode, response.Body, err)
	// Output: 200 "ok" <nil>
}
