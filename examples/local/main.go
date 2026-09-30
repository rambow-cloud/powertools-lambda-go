package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func main() {
	ctx := context.Background()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(ctx)
	tr, err := tracer.New(tracer.WithLocalTracing(true), tracer.WithBackend(tracer.NewOTelBackend(provider)), tracer.WithServiceName("demo"))
	if err != nil {
		log.Fatal(err)
	}
	l := logger.New(logger.WithServiceName("demo"))
	handler := tracer.WrapHandler(tr, logger.WrapHandler(l, func(ctx context.Context, name string) (string, error) {
		_ = l.WithContext(ctx).Info("Hello", logger.Fields{"name": name})
		return "Hello, " + name, nil
	}))
	if _, err := handler(ctx, "Go"); err != nil {
		log.Fatal(err)
	}
	for _, span := range exporter.GetSpans() {
		fmt.Printf("Span: %s, trace: %s\n", span.Name, span.SpanContext.TraceID())
	}
}
