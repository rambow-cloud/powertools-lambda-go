package tracer_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httptracer "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer"
	powertracer "github.com/rambow-cloud/powertools-lambda-go/tracer"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestReference(t *testing.T) {
	data, err := os.ReadFile("testdata/tracer-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name, Action string
			Event        json.RawMessage
			Status       int
			Spans        []struct {
				Name   string
				HTTP   map[string]any
				Closed bool
			}
			Responses []struct {
				Name  string
				Value any
			}
			Errors, Annotations []string
			Restored            bool
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 128 {
		t.Fatalf("cases: %d", len(fixture.Cases))
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			tr, recorder := newTracer(t, powertracer.WithEnabled(item.Action != "disabled"), powertracer.WithCaptureResponse(item.Action != "config-off"))
			app := httpapi.New(httpapi.Options{})
			app.Use(httptracer.New(tr, httptracer.Options{DisableCaptureResponse: item.Action == "capture-off"}))
			if item.Action == "compressed" {
				threshold := float64(0)
				app.Use(httpapi.Compress(httpapi.CompressionOptions{Threshold: &threshold}))
			}
			if err := app.Get("/items/:id", func(*httpapi.RequestContext) (any, error) {
				switch item.Action {
				case "http-error":
					return nil, httpapi.NewHTTPError(400, "bad input")
				case "error":
					return nil, errors.New("business failure")
				}
				contentType, body := "application/json", `{"ok":true}`
				switch item.Action {
				case "text":
					contentType, body = "text/plain", "hello"
				case "charset":
					contentType = "application/json; charset=utf-8"
				case "invalid-json":
					body = "{"
				}
				headers := http.Header{"Content-Type": []string{contentType}}
				if length := map[string]string{"length": "12", "length-prefix": "+12trailing", "length-negative": "-2", "length-invalid": "invalid"}[item.Action]; length != "" {
					headers.Set("Content-Length", length)
				}
				status := 200
				if item.Action == "server" {
					status = 503
				}
				return httpapi.Response{StatusCode: status, Headers: headers, Body: body}, nil
			}); err != nil {
				t.Fatal(err)
			}
			response, err := app.Resolve(context.Background(), item.Event)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != item.Status {
				t.Fatalf("status: %d, want %d", response.StatusCode, item.Status)
			}
			spans := recorder.Ended()
			if len(spans) != len(item.Spans) {
				t.Fatalf("spans: %d, want %d", len(spans), len(item.Spans))
			}
			if len(spans) == 0 {
				return
			}
			span, expected := spans[0], item.Spans[0]
			if span.Name() != expected.Name || !expected.Closed || !item.Restored || span.SpanKind() != trace.SpanKindInternal {
				t.Fatal("span lifecycle mismatch")
			}
			attrs := attributes(span)
			var recorded map[string]any
			if err := json.Unmarshal([]byte(attrs["powertools.metadata.orders.http"].(string)), &recorded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(recorded, expected.HTTP) {
				t.Fatalf("HTTP data: got %#v, want %#v", recorded, expected.HTTP)
			}
			if attrs["Service"] != "orders" {
				t.Fatal("service annotation missing")
			}
			if _, ok := attrs["ColdStart"].(bool); !ok {
				t.Fatal("cold-start annotation missing")
			}
			responseKey := "powertools.metadata.orders." + strings.NewReplacer("%", "%25", ".", "%2E").Replace(span.Name()+" response")
			if len(item.Responses) == 0 {
				if _, ok := attrs[responseKey]; ok {
					t.Fatal("unexpected response capture")
				}
			} else {
				var captured any
				if err := json.Unmarshal([]byte(attrs[responseKey].(string)), &captured); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(captured, item.Responses[0].Value) {
					t.Fatalf("response metadata: %#v", captured)
				}
			}
			if len(item.Errors) > 0 && (span.Status().Code != codes.Error || len(span.Events()) != 1) {
				t.Fatal("error capture missing")
			}
			if attrs["http.request.method"] != "GET" || attrs["http.response.status_code"] != int64(expected.HTTP["response"].(map[string]any)["status"].(float64)) {
				t.Fatal("OTel HTTP attributes missing")
			}
		})
	}
}

func newTracer(t *testing.T, options ...powertracer.Option) (*powertracer.Tracer, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	options = append([]powertracer.Option{powertracer.WithLocalTracing(true), powertracer.WithServiceName("orders"), powertracer.WithBackend(powertracer.NewOTelBackend(provider))}, options...)
	tr, err := powertracer.New(options...)
	if err != nil {
		t.Fatal(err)
	}
	return tr, recorder
}

func attributes(span sdktrace.ReadOnlySpan) map[string]any {
	result := map[string]any{}
	for _, attribute := range span.Attributes() {
		result[string(attribute.Key)] = attribute.Value.AsInterface()
	}
	return result
}
