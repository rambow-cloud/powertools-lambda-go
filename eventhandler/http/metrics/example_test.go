package metrics_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
	"github.com/rambow-cloud/powertools-lambda-go/metrics"
)

func ExampleNew() {
	var output bytes.Buffer
	m, err := metrics.New(metrics.WithNamespace("Example/Orders"),
		metrics.WithServiceName("orders"), metrics.WithOutput(&output))
	if err != nil {
		panic(err)
	}
	app := httpapi.New(httpapi.Options{})
	app.Use(httpmetrics.New(m))
	if err := app.Get("/health", func(*httpapi.RequestContext) (any, error) {
		return "ok", nil
	}); err != nil {
		panic(err)
	}
	event := json.RawMessage(`{"version":"2.0","routeKey":"$default","rawPath":"/health","rawQueryString":"","headers":{},"requestContext":{"http":{"method":"GET"},"domainName":"api.example.test"},"isBase64Encoded":false}`)
	response, err := app.Resolve(context.Background(), event)
	fmt.Println(response.StatusCode, output.Len() > 0, err)
	// Output: 200 true <nil>
}
