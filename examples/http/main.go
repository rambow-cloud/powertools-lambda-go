// Package main shows a native Lambda HTTP router composed with Logger and OTel.
package main

import (
	"context"
	jsonv1 "encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
	httptracer "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/metrics"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

func main() {
	requestLog := logger.New(logger.WithServiceName("orders"))
	trace, err := tracer.New(tracer.WithServiceName("orders"))
	if err != nil {
		log.Fatal(err)
	}
	app := httpapi.New(httpapi.Options{})
	requestMetrics, err := metrics.New(metrics.WithNamespace("Orders"), metrics.WithServiceName("orders"))
	if err != nil {
		log.Fatal(err)
	}
	// Disable body capture when tracing outside compression; compressed bytes
	// cannot be parsed as JSON by the reference-compatible capture path.
	app.Use(httpmetrics.New(requestMetrics), httptracer.New(trace, httptracer.Options{DisableCaptureResponse: true}))
	app.Use(httpapi.CORS(httpapi.CORSOptions{Origins: []string{"https://app.example.com"}}))
	app.Use(httpapi.Compress(httpapi.CompressionOptions{}))
	if err := app.Get("/orders/:id", func(request *httpapi.RequestContext) (any, error) {
		_ = requestLog.WithContext(request.Context).Info("Get order", logger.Fields{"order_id": request.Params["id"]})
		return map[string]any{"id": request.Params["id"]}, nil
	}); err != nil {
		log.Fatal(err)
	}
	handler := func(ctx context.Context, event jsonv1.RawMessage) (httpapi.ProxyResponse, error) {
		return app.Resolve(ctx, event)
	}
	lambda.Start(tracer.WrapHandler(trace, logger.WrapHandler(requestLog, handler)))
}
