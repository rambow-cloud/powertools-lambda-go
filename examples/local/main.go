package main

import (
	"context"
	"os"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func main() {
	appLog := logger.New(logger.WithServiceName("demo"))
	if err := run(appLog); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func run(appLog *logger.Logger) error {
	ctx := context.Background()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(ctx)
	tr, err := tracer.New(tracer.WithLocalTracing(true), tracer.WithBackend(tracer.NewOTelBackend(provider)), tracer.WithServiceName("demo"))
	if err != nil {
		return err
	}
	handler := tracer.WrapHandler(tr, logger.WrapHandler(appLog, func(ctx context.Context, name string) (string, error) {
		if err := appLog.WithContext(ctx).Info("Hello", logger.Fields{"name": name}); err != nil {
			return "", err
		}
		return "Hello, " + name, nil
	}))
	if _, err := handler(ctx, "Go"); err != nil {
		return err
	}
	for _, span := range exporter.GetSpans() {
		if err := appLog.Info("Span recorded", logger.Fields{"span": span.Name, "trace_id": span.SpanContext.TraceID().String()}); err != nil {
			return err
		}
	}
	return nil
}
