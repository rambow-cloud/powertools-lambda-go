package main

import (
	"context"
	"os"

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
	appLog := logger.New(
		logger.WithServiceName("hello"),
		logger.WithErrorHandler(func(err error) {
			panic(err)
		}),
	)
	tr, err := tracer.New(
		tracer.WithServiceName("hello"),
		tracer.WithCaptureResponse(false),
		tracer.WithErrorHandler(func(err error) {
			if logErr := appLog.Error("Tracing failed", err); logErr != nil {
				panic(logErr)
			}
		}),
	)
	if err != nil {
		if logErr := appLog.Error("Tracer initialization failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
	handler := func(ctx context.Context, event Event) (Response, error) {
		requestLog := appLog.WithContext(ctx)
		if err := requestLog.Info("Handling request", logger.Fields{"name": event.Name}); err != nil {
			return Response{}, err
		}
		return tracer.Capture(ctx, tr, "greet", func(ctx context.Context) (Response, error) {
			_ = tr.PutAnnotation(ctx, "Operation", "greet")
			return Response{Message: "Hello, " + event.Name}, nil
		})
	}
	// Put the tracer outside the logger wrapper so logs receive the active span context.
	lambda.Start(tracer.WrapHandler(tr, logger.WrapHandler(appLog, handler)))
}
