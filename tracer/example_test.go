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
	backend := tracer.NewOTelBackend(provider)
	// Direct backend operations use the supplied provider independently of
	// the environment flags that control a Tracer instance.
	_, span, err := backend.Start(context.Background(), "accept-order", tracer.Internal)
	if err != nil {
		panic(err)
	}
	fmt.Println(span.TraceID() != "")
	span.End()
	// Output: true
}
