// Package main streams native Lambda HTTP responses with complete log/span lifetimes.
package main

import (
	"context"
	jsonv1 "encoding/json"
	"io"
	"log"
	nethttp "net/http"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

type streamCall struct {
	event  jsonv1.RawMessage
	writer io.Writer
}

func main() {
	requestLog := logger.New(logger.WithServiceName("streaming-orders"))
	trace, err := tracer.New(tracer.WithServiceName("streaming-orders"))
	if err != nil {
		log.Fatal(err)
	}
	app := httpapi.New(httpapi.Options{})
	app.Use(httpapi.CORS(httpapi.CORSOptions{Origins: []string{"https://app.example.com"}}))
	if err := app.Get("/events", func(request *httpapi.RequestContext) (any, error) {
		// Replace the reader with a context-aware incremental producer.
		return httpapi.Response{StatusCode: 200, Headers: nethttp.Header{"Content-Type": []string{"text/event-stream"}}, Body: strings.NewReader("data: ready\n\n")}, nil
	}); err != nil {
		log.Fatal(err)
	}
	complete := tracer.WrapHandler(trace, logger.WrapHandler(requestLog, func(ctx context.Context, call streamCall) (struct{}, error) {
		return struct{}{}, app.ResolveStream(ctx, call.event, call.writer)
	}), tracer.HandlerOptions{DisableCaptureResponse: true})
	lambda.Start(httpapi.Streamify(func(ctx context.Context, event jsonv1.RawMessage, destination io.Writer) error {
		_, err := complete(ctx, streamCall{event: event, writer: destination})
		return err
	}))
}
