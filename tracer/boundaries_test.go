package tracer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestCaptureConfigurationBoundaries(t *testing.T) {
	for _, kind := range []string{"response", "error", "http"} {
		for _, control := range []string{"default", "option-off", "env-off", "env-veto", "env-does-not-enable"} {
			t.Run(kind+"-"+control, func(t *testing.T) {
				options := []Option{}
				enabled := control == "default"
				if control == "option-off" || control == "env-veto" || control == "env-does-not-enable" {
					optionEnabled := control == "env-veto"
					switch kind {
					case "response":
						options = append(options, WithCaptureResponse(optionEnabled))
					case "error":
						options = append(options, WithCaptureError(optionEnabled))
					case "http":
						options = append(options, WithCaptureHTTP(optionEnabled))
					}
				}
				tr, exporter := setup(t, options...)
				if control == "env-off" || control == "env-veto" || control == "env-does-not-enable" {
					key := map[string]string{"response": "POWERTOOLS_TRACER_CAPTURE_RESPONSE", "error": "POWERTOOLS_TRACER_CAPTURE_ERROR", "http": "POWERTOOLS_TRACER_CAPTURE_HTTPS_REQUESTS"}[kind]
					value := "FALSE"
					if control == "env-does-not-enable" {
						value = "true"
					}
					t.Setenv(key, value)
					var err error
					tr, err = New(append([]Option{WithLocalTracing(true), WithBackend(tr.backend), WithServiceName("orders")}, options...)...)
					if err != nil {
						t.Fatal(err)
					}
				}
				switch kind {
				case "response":
					for _, wrapperOff := range []bool{false, true} {
						exporter.Reset()
						got, err := WrapHandler(tr, func(context.Context, int) (int, error) { return 7, nil }, HandlerOptions{Name: "audit", DisableCaptureResponse: wrapperOff})(context.Background(), 0)
						spans := exporter.GetSpans()
						if err != nil || got != 7 || len(spans) != 1 {
							t.Fatalf("result=%d spans=%d error=%v", got, len(spans), err)
						}
						_, captured := attrs(spans[0])["powertools.metadata.orders.audit response"]
						if captured != (enabled && !wrapperOff) {
							t.Fatalf("captured=%v", captured)
						}
					}
				case "error":
					failure := errors.New("business failure")
					got, err := WrapHandler(tr, func(context.Context, int) (int, error) { return 7, failure })(context.Background(), 0)
					spans := exporter.GetSpans()
					if got != 7 || err != failure || len(spans) != 1 || spans[0].Status.Code != codes.Error {
						t.Fatalf("result=%d error=%v spans=%v", got, err, spans)
					}
					if (len(spans[0].Events) > 0) != enabled {
						t.Fatal("error capture policy ignored")
					}
					if !enabled && spans[0].Status.Description != "operation failed" {
						t.Fatal("error description was not suppressed")
					}
				case "http":
					underlying := roundTrip(func(r *http.Request) (*http.Response, error) { return responseForBoundary(r), nil })
					original := &http.Client{Transport: underlying, Timeout: time.Second}
					copied := tr.HTTPClient(original)
					if copied == original || copied.Timeout != original.Timeout {
						t.Fatal("HTTP client was not copied")
					}
					request, _ := http.NewRequestWithContext(context.Background(), "GET", "https://example.invalid", nil)
					response, err := copied.Do(request)
					if err != nil {
						t.Fatal(err)
					}
					_ = response.Body.Close()
					if (len(exporter.GetSpans()) == 1) != enabled {
						t.Fatal("HTTP capture policy ignored")
					}
					if request.Header.Get("Traceparent") != "" {
						t.Fatal("caller request mutated")
					}
				}
			})
		}
	}
}

func responseForBoundary(r *http.Request) *http.Response {
	return &http.Response{StatusCode: 204, Header: http.Header{}, Body: http.NoBody, Request: r}
}

type boundaryProvider struct {
	trace.TracerProvider
	flush     func(context.Context) error
	shutdowns atomic.Int32
}

func (p *boundaryProvider) ForceFlush(ctx context.Context) error { return p.flush(ctx) }
func (p *boundaryProvider) Shutdown(context.Context) error       { p.shutdowns.Add(1); return nil }

type boundaryContextKey struct{}

func TestInvocationFlushBoundaries(t *testing.T) {
	for _, mode := range []string{"success", "business-error", "panic", "cancelled", "expired-deadline"} {
		t.Run(mode, func(t *testing.T) {
			tr, exporter := setup(t)
			original := tr.backend.(*OTelBackend).provider
			before := time.Now()
			deadline := before.Add(time.Minute)
			if mode == "expired-deadline" {
				deadline = before.Add(-time.Second)
			}
			parent, cancel := context.WithDeadline(context.WithValue(context.Background(), boundaryContextKey{}, "invocation"), deadline)
			defer cancel()
			failure := errors.New("business failure")
			flushFailure := errors.New("flush failure")
			provider := &boundaryProvider{TracerProvider: original}
			flushes := 0
			provider.flush = func(ctx context.Context) error {
				flushes++
				limit, ok := ctx.Deadline()
				if !ok || limit.After(deadline) || limit.After(time.Now().Add(2*time.Second)) || ctx.Value(boundaryContextKey{}) != "invocation" {
					t.Fatalf("unbounded or lost flush context: deadline=%v", limit)
				}
				if len(exporter.GetSpans()) != 1 {
					t.Fatal("flush preceded span completion")
				}
				if mode == "expired-deadline" {
					if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
						t.Fatal("expired deadline extended")
					}
					return ctx.Err()
				}
				if ctx.Err() != nil {
					t.Fatal("caller cancellation prevented bounded flush")
				}
				return flushFailure
			}
			reports := []error{}
			var err error
			tr, err = New(WithLocalTracing(true), WithBackend(NewOTelBackend(provider)), WithErrorHandler(func(err error) { reports = append(reports, err) }))
			if err != nil {
				t.Fatal(err)
			}
			var result int
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				result, err = WrapHandler(tr, func(context.Context, int) (int, error) {
					switch mode {
					case "panic":
						panic(failure)
					case "business-error":
						return 7, failure
					case "cancelled":
						cancel()
					}
					return 7, nil
				})(parent, 0)
			}()
			if flushes != 1 || len(reports) != 1 || provider.shutdowns.Load() != 0 {
				t.Fatalf("flushes=%d reports=%v shutdowns=%d", flushes, reports, provider.shutdowns.Load())
			}
			if mode == "expired-deadline" {
				if !errors.Is(reports[0], context.DeadlineExceeded) {
					t.Fatal(reports)
				}
			} else if reports[0] != flushFailure {
				t.Fatal(reports)
			}
			switch mode {
			case "panic":
				if recovered != failure {
					t.Fatal("panic identity changed")
				}
			case "business-error":
				if result != 7 || err != failure {
					t.Fatal(result, err)
				}
			default:
				if result != 7 || err != nil || recovered != nil {
					t.Fatal(result, err, recovered)
				}
			}
			if err := tr.Shutdown(context.Background()); err != nil || provider.shutdowns.Load() != 0 {
				t.Fatal("injected provider shut down")
			}
			// Ownership is an internal backend decision, not an application-provider veto.
			owned := &OTelBackend{provider: provider, owned: true}
			if err := owned.Shutdown(context.Background()); err != nil || provider.shutdowns.Load() != 1 {
				t.Fatal("owned shutdown was not delegated")
			}
		})
	}
}

func TestClosedSpansAndConcurrentInvocationIsolation(t *testing.T) {
	tr, exporter := setup(t)
	var contexts [32]context.Context
	var wait sync.WaitGroup
	for i := range contexts {
		wait.Go(func() {
			handler := WrapHandler(tr, func(ctx context.Context, n int) (int, error) {
				contexts[n] = ctx
				if err := tr.PutAnnotation(ctx, "caller", n); err != nil {
					return 0, err
				}
				return n, nil
			}, HandlerOptions{Name: fmt.Sprintf("audit-%d", i)})
			result, err := handler(context.Background(), i)
			if err != nil || result != i {
				t.Error(result, err)
			}
		})
	}
	wait.Wait()
	spans := exporter.GetSpans()
	if len(spans) != len(contexts) {
		t.Fatal("invocation spans lost")
	}
	for i, ctx := range contexts {
		if trace.SpanFromContext(ctx).IsRecording() {
			t.Fatal("handler span still open")
		}
		if tr.TraceID(ctx) == "" {
			t.Fatal("closed context lost correlation")
		}
		// SDK spans ignore late attributes; they retain identity without reopening.
		if err := tr.PutMetadata(ctx, "late", "ignored"); err != nil {
			t.Fatal(err)
		}
		if err := tr.PutAnnotation(ctx, "late", i); err != nil {
			t.Fatal(err)
		}
	}
	if len(exporter.GetSpans()) != 32 {
		t.Fatal("late writes exported another span")
	}
	names := map[string]bool{}
	for _, span := range exporter.GetSpans() {
		caller := attrs(span)["caller"].(int64)
		if span.Name != fmt.Sprintf("## audit-%d", caller) || attrs(span)["late"] != nil || attrs(span)["powertools.metadata.orders.late"] != nil || names[span.SpanContext.SpanID().String()] {
			t.Fatal("cross-invocation or late attribute contamination")
		}
		names[span.SpanContext.SpanID().String()] = true
	}
}

func TestStartFailureAndMetadataFailurePreserveResults(t *testing.T) {
	tr, exporter := setup(t)
	reports := []error{}
	var err error
	tr, err = New(WithLocalTracing(true), WithBackend(tr.backend), WithErrorHandler(func(err error) { reports = append(reports, err) }))
	if err != nil {
		t.Fatal(err)
	}
	value := make(chan int)
	result, err := WrapHandler(tr, func(context.Context, int) (chan int, error) { return value, nil })(context.Background(), 0)
	if result != value || err != nil || len(reports) != 1 || len(exporter.GetSpans()) != 1 {
		t.Fatal("metadata serialization replaced business result")
	}
	reports = nil
	tr, err = New(WithLocalTracing(true), WithBackend(NewOTelBackend(nil)), WithErrorHandler(func(err error) { reports = append(reports, err) }))
	if err != nil {
		t.Fatal(err)
	}
	got, err := WrapHandler(tr, func(context.Context, int) (int, error) { return 7, nil })(context.Background(), 0)
	if got != 7 || err != nil || len(reports) != 1 {
		t.Fatal("backend start failure replaced result")
	}
}

func TestOTelSiblingSpanSurvivesCompletion(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter), sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	t.Setenv("POWERTOOLS_TRACE_ENABLED", "")
	t.Setenv("POWERTOOLS_DEV", "")
	first, _ := New(WithLocalTracing(true), WithBackend(NewOTelBackend(provider)))
	second, _ := New(WithLocalTracing(true), WithBackend(NewOTelBackend(provider)))
	one, endOne := first.StartSpan(context.Background(), "one")
	two, endTwo := second.StartSpan(context.Background(), "two")
	if err := first.PutAnnotation(two, "wrong-owner", true); err != nil {
		t.Fatal(err)
	}
	endOne(nil)
	endOne(errors.New("duplicate completion"))
	if err := first.Flush(one); err != nil {
		t.Fatal(err)
	}
	if err := first.Shutdown(one); err != nil {
		t.Fatal(err)
	}
	if !trace.SpanFromContext(two).IsRecording() {
		t.Fatal("sibling span ended")
	}
	if err := second.PutMetadata(two, "active", true); err != nil {
		t.Fatal(err)
	}
	endTwo(nil)
	spans := exporter.GetSpans()
	if len(spans) != 2 || attrs(spans[1])["wrong-owner"] != nil {
		t.Fatal("ownership or completion isolation failed")
	}
}

func TestSDKTracingGateAndHTTPControlIndependence(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			tr, exporter := setup(t, WithEnabled(enabled), WithCaptureHTTP(false))
			calls := 0
			transport := roundTrip(func(request *http.Request) (*http.Response, error) {
				calls++
				markers := strings.Count(request.Header.Get("User-Agent"), "PT/tracer/")
				want := 0
				if enabled {
					want = 1
				}
				if markers != want {
					t.Errorf("SDK identity markers=%d want=%d", markers, want)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Request: request, Body: io.NopCloser(strings.NewReader(`{"TableNames":[]}`))}, nil
			})
			client := dynamodb.New(dynamodb.Options{Region: "ap-east-1", BaseEndpoint: aws.String("https://dynamodb.local"), RetryMaxAttempts: 1, HTTPClient: &http.Client{Transport: transport}, Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
			})}, func(o *dynamodb.Options) { tr.InstrumentAWS(&o.APIOptions) })
			if _, err := client.ListTables(context.Background(), &dynamodb.ListTablesInput{}); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || (len(exporter.GetSpans()) > 0) != enabled {
				t.Fatalf("calls=%d spans=%d", calls, len(exporter.GetSpans()))
			}
		})
	}
}

func TestTracingEnablementVetoBoundaries(t *testing.T) {
	for _, mode := range []string{"option-off", "environment-off", "development", "local-runtime"} {
		t.Run(mode, func(t *testing.T) {
			base, exporter := setup(t)
			options := []Option{WithBackend(base.backend), WithLocalTracing(true)}
			switch mode {
			case "option-off":
				options = append(options, WithEnabled(false))
			case "environment-off":
				t.Setenv("POWERTOOLS_TRACE_ENABLED", "FALSE")
			case "development":
				t.Setenv("POWERTOOLS_DEV", "true")
			case "local-runtime":
				options = append(options, WithLocalTracing(false))
				t.Setenv("AWS_EXECUTION_ENV", "")
				t.Setenv("AWS_LAMBDA_RUNTIME_API", "")
			}
			tr, err := New(options...)
			if err != nil {
				t.Fatal(err)
			}
			if tr.Enabled() {
				t.Fatal("enablement veto ignored")
			}
			result, err := WrapHandler(tr, func(context.Context, int) (int, error) { return 7, nil })(context.Background(), 0)
			if result != 7 || err != nil || len(exporter.GetSpans()) != 0 {
				t.Fatal("disabled tracer changed result or created spans")
			}
		})
	}
}
