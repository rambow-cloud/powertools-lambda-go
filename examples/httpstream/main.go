// Package main streams native Lambda HTTP responses with complete log/span lifetimes.
package main

import (
	"context"
	jsonv1 "encoding/json"
	"io"
	nethttp "net/http"
	"os"
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
	appLog := logger.New(logger.WithServiceName("streaming-orders"))
	if err := run(appLog); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func run(appLog *logger.Logger) error {
	trace, err := tracer.New(tracer.WithServiceName("streaming-orders"))
	if err != nil {
		return err
	}
	app := httpapi.New(httpapi.Options{})
	app.Use(httpapi.CORS(httpapi.CORSOptions{Origins: []string{"https://app.example.com"}}))
	if err := app.Get("/events", func(request *httpapi.RequestContext) (any, error) {
		// Replace the reader with a context-aware incremental producer.
		return httpapi.Response{StatusCode: 200, Headers: nethttp.Header{"Content-Type": []string{"text/event-stream"}}, Body: strings.NewReader("data: ready\n\n")}, nil
	}); err != nil {
		return err
	}
	complete := tracer.WrapHandler(trace, logger.WrapHandler(appLog, func(ctx context.Context, call streamCall) (struct{}, error) {
		return struct{}{}, app.ResolveStream(ctx, call.event, call.writer)
	}), tracer.HandlerOptions{DisableCaptureResponse: true})
	lambda.Start(httpapi.Streamify(func(ctx context.Context, event jsonv1.RawMessage, destination io.Writer) error {
		_, err := complete(ctx, streamCall{event: event, writer: destination})
		return err
	}))
	return nil
}
