package tracer_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/tracer"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func ExampleNewOTelBackend() {
	// Production providers can export OTLP to a collector using awsxray.
	// This provider has no exporter, so the example performs no network calls.
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	t, err := tracer.New(tracer.WithServiceName("orders"),
		tracer.WithBackend(tracer.NewOTelBackend(provider)), tracer.WithLocalTracing(true))
	if err != nil {
		panic(err)
	}
	ctx, end := t.StartSpan(context.Background(), "accept-order")
	fmt.Println(t.TraceID(ctx) != "")
	end(nil)
	// Output: true
}
