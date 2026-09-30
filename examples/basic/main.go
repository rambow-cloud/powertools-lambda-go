package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

type Event struct {
	Name string `json:"name"`
}
type Response struct {
	Message string `json:"message"`
}

func main() {
	l := logger.New(logger.WithServiceName("hello"), logger.WithErrorHandler(func(err error) { log.Printf("logging failed: %v", err) }))
	tr, err := tracer.New(tracer.WithServiceName("hello"), tracer.WithCaptureResponse(false), tracer.WithErrorHandler(func(err error) { log.Printf("tracing failed: %v", err) }))
	if err != nil {
		log.Fatal(err)
	}
	handler := func(ctx context.Context, event Event) (Response, error) {
		bound := l.WithContext(ctx)
		_ = bound.Info("Handling request", logger.Fields{"name": event.Name})
		return tracer.Capture(ctx, tr, "greet", func(ctx context.Context) (Response, error) {
			_ = tr.PutAnnotation(ctx, "Operation", "greet")
			return Response{Message: "Hello, " + event.Name}, nil
		})
	}
	// Put the tracer outside the logger wrapper so logs receive the active span context.
	lambda.Start(tracer.WrapHandler(tr, logger.WrapHandler(l, handler)))
}
