package tracer

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestLambdaHTTPEnvelopeExtraction(t *testing.T) {
	parent := "00-65abcdef123456789012345678901234-1234567890123456-01"
	for _, event := range []any{
		map[string]any{"headers": map[string]string{"Traceparent": parent}},
		map[string]any{"headers": map[string]string{"traceparent": "invalid"}, "multiValueHeaders": map[string][]string{"traceparent": {parent}}},
		map[string]any{"request": map[string]any{"headers": map[string]string{"traceparent": parent}}},
		struct {
			Headers map[string]string `json:"headers"`
		}{map[string]string{"traceparent": parent}},
	} {
		ctx, cancel := context.WithCancel(context.Background())
		extracted := ExtractLambdaHTTPContext(ctx, event)
		if got := trace.SpanContextFromContext(extracted); !got.IsRemote() || got.TraceID().String() != "65abcdef123456789012345678901234" {
			t.Fatal(got)
		}
		if ExtractLambdaHTTPContext(extracted, nil) != extracted {
			t.Fatal("existing context replaced")
		}
		cancel()
		if extracted.Err() != context.Canceled {
			t.Fatal("cancellation lost")
		}
	}
	for _, event := range []any{nil, "not an event", map[string]any{"headers": 42}, make(chan int), map[string]any{"Records": []any{}}} {
		ctx := context.Background()
		if ExtractLambdaHTTPContext(ctx, event) != ctx {
			t.Fatal("invalid event changed context")
		}
	}
}
