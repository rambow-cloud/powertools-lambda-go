package tracer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

func TestHTTPContextPrecedenceAndExternalTraceQueries(t *testing.T) {
	tr, _ := setup(t)
	headers := http.Header{}
	headers.Set("traceparent", "00-65abcdef123456789012345678901234-1234567890123456-00")
	headers.Set("X-Amzn-Trace-Id", "Root=1-65abcdef-999999999999999999999999;Parent=9999999999999999;Sampled=1")
	headers.Set("baggage", "tenant=orders")
	parent, cancel := context.WithTimeout(context.Background(), time.Minute)
	ctx := ExtractHTTPContext(parent, headers)
	if tr.TraceID(ctx) != "65abcdef123456789012345678901234" || tr.XRayTraceID(ctx) != "1-65abcdef-123456789012345678901234" || tr.IsTraceSampled(ctx) || baggage.FromContext(ctx).Member("tenant").Value() != "orders" {
		t.Fatal("W3C context or sampling lost")
	}
	if ExtractHTTPContext(ctx, headers) != ctx {
		t.Fatal("valid existing context replaced")
	}
	cancel()
	if ctx.Err() != context.Canceled {
		t.Fatal("extraction lost cancellation")
	}
	headers.Set("traceparent", "invalid")
	ctx = ExtractHTTPContext(context.Background(), headers)
	if tr.TraceID(ctx) != "65abcdef999999999999999999999999" || !tr.IsTraceSampled(ctx) {
		t.Fatal("valid X-Ray propagation fallback lost")
	}
	headers.Set("X-Amzn-Trace-Id", "invalid")
	if trace.SpanContextFromContext(ExtractHTTPContext(context.Background(), headers)).IsValid() {
		t.Fatal("malformed headers produced a parent")
	}
	ctx = context.WithValue(context.Background(), "x-amzn-trace-id", "Root=1-65abcdef-123456789012345678901234;Parent=1234567890123456;Sampled=1")
	if tr.XRayTraceID(ctx) != "1-65abcdef-123456789012345678901234" || !tr.IsTraceSampled(ctx) {
		t.Fatal("Lambda runtime fallback lost")
	}
}

func TestDefaultOTelSamplingAndXRayIDs(t *testing.T) {
	for _, scenario := range []struct {
		name, sampler, ratio, parent string
		want                         bool
	}{
		{"default root", "", "", "", true},
		{"off root", "always_off", "", "", false},
		{"zero ratio", "traceidratio", "0", "", false},
		{"full ratio", "traceidratio", "1", "", true},
		{"sampled parent", "parentbased_always_off", "", "01", true},
		{"unsampled parent", "parentbased_always_on", "", "00", false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER", scenario.sampler)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", scenario.ratio)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
			defer server.Close()
			t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", server.URL+"/v1/traces")
			backend, err := newDefaultOTelBackend(context.Background(), "orders")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := backend.Shutdown(ctx); err != nil {
					t.Error(err)
				}
			}()
			ctx := context.Background()
			if scenario.parent != "" {
				h := http.Header{}
				h.Set("traceparent", "00-65abcdef123456789012345678901234-1234567890123456-"+scenario.parent)
				ctx = ExtractHTTPContext(ctx, h)
			}
			_, span, err := backend.Start(ctx, "sampling", Internal)
			if err != nil {
				t.Fatal(err)
			}
			defer span.End()
			if span.Sampled() != scenario.want {
				t.Fatalf("sampled=%v, want=%v", span.Sampled(), scenario.want)
			}
			if scenario.parent == "" {
				epoch, err := strconv.ParseInt(span.TraceID()[:8], 16, 64)
				if err != nil || time.Since(time.Unix(epoch, 0)) < 0 || time.Since(time.Unix(epoch, 0)) > 5*time.Second {
					t.Fatalf("root ID is not X-Ray compatible: %s", span.TraceID())
				}
			}
		})
	}
}
