package tracer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setup(t *testing.T, opts ...Option) (*Tracer, *tracetest.InMemoryExporter) {
	t.Helper()
	for _, k := range []string{"POWERTOOLS_TRACE_ENABLED", "POWERTOOLS_DEV", "POWERTOOLS_TRACER_CAPTURE_RESPONSE", "POWERTOOLS_TRACER_CAPTURE_ERROR", "POWERTOOLS_TRACER_CAPTURE_HTTPS_REQUESTS", "_X_AMZN_TRACE_ID"} {
		t.Setenv(k, "")
	}
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	options := []Option{WithBackend(NewOTelBackend(provider)), WithLocalTracing(true), WithServiceName("orders")}
	options = append(options, opts...)
	tracer, err := New(options...)
	if err != nil {
		t.Fatal(err)
	}
	return tracer, exporter
}
func attrs(s tracetest.SpanStub) map[string]any {
	out := map[string]any{}
	for _, a := range s.Attributes {
		out[string(a.Key)] = a.Value.AsInterface()
	}
	return out
}
func TestHandlerParentMetadataAndNestedSpan(t *testing.T) {
	tr, exporter := setup(t)
	h := WrapHandler(tr, func(ctx context.Context, n int) (int, error) {
		return Capture(ctx, tr, "work", func(ctx context.Context) (int, error) {
			if err := tr.PutAnnotation(ctx, "order", n); err != nil {
				return 0, err
			}
			return n + 1, nil
		})
	}, HandlerOptions{Name: "handler"})
	ctx := context.WithValue(context.Background(), "x-amzn-trace-id", "Root=1-12345678-123456789012345678901234;Parent=1234567890123456;Sampled=1")
	if got, err := h(ctx, 1); got != 2 || err != nil {
		t.Fatal(got, err)
	}
	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatal(spans)
	}
	child, parent := spans[0], spans[1]
	if parent.Name != "## handler" || child.Name != "### work" || child.Parent.SpanID() != parent.SpanContext.SpanID() || parent.Parent.SpanID().String() != "1234567890123456" || parent.SpanContext.TraceID().String() != "12345678123456789012345678901234" {
		t.Fatal(spans)
	}
	if attrs(parent)["powertools.metadata.orders.handler response"] != "2" || attrs(parent)["Service"] != "orders" || attrs(child)["order"] != int64(1) {
		t.Fatal(attrs(parent), attrs(child))
	}
	exporter.Reset()
	ctx = context.WithValue(ctx, "x-amzn-trace-id", "Root=1-12345678-123456789012345678901234;Parent=1234567890123456;Sampled=0")
	_, _ = h(ctx, 2)
	if len(exporter.GetSpans()) != 0 {
		t.Fatal("parent sampling was ignored")
	}
}
func TestErrorRedactionAndPanic(t *testing.T) {
	tr, exporter := setup(t, WithCaptureError(false), WithCaptureResponse(false))
	business := errors.New("secret")
	h := WrapHandler(tr, func(context.Context, int) (int, error) { return 42, business })
	got, err := h(context.Background(), 0)
	if got != 42 || err != business {
		t.Fatal(got, err)
	}
	s := exporter.GetSpans()[0]
	if s.Status.Code != codes.Error || s.Status.Description != "operation failed" || len(s.Events) != 0 {
		t.Fatal(s)
	}
	exporter.Reset()
	token := &struct{}{}
	func() {
		defer func() {
			if recover() != token {
				t.Error("panic changed")
			}
		}()
		_, _ = WrapHandler(tr, func(context.Context, int) (int, error) { panic(token) })(context.Background(), 0)
	}()
	if len(exporter.GetSpans()) != 1 || exporter.GetSpans()[0].EndTime.IsZero() {
		t.Fatal("panic span was not closed")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestHTTPAndAWSInstrumentation(t *testing.T) {
	tr, exporter := setup(t)
	ctx, end := tr.StartSpan(context.Background(), "parent")
	defer end(nil)
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Traceparent") == "" || r.Header.Get("X-Amzn-Trace-Id") == "" {
			t.Error("missing propagation headers")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"TableNames":[]}`)), Request: r}, nil
	})
	original := &http.Client{Transport: transport}
	client := tr.HTTPClient(original)
	request, _ := http.NewRequestWithContext(ctx, "GET", "https://example.invalid", nil)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if request.Header.Get("Traceparent") != "" {
		t.Fatal("request mutated")
	}
	awsClient := dynamodb.New(dynamodb.Options{Region: "us-east-1", Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	}), HTTPClient: &http.Client{Transport: transport}}, func(o *dynamodb.Options) { tr.InstrumentAWS(&o.APIOptions) })
	if _, err := awsClient.ListTables(ctx, &dynamodb.ListTablesInput{}); err != nil {
		t.Fatal(err)
	}
	if len(exporter.GetSpans()) < 2 {
		t.Fatal("client spans missing")
	}
}

type failingFlush struct {
	*OTelBackend
	deadline bool
}

func (b *failingFlush) ForceFlush(ctx context.Context) error {
	_, b.deadline = ctx.Deadline()
	<-ctx.Done()
	return ctx.Err()
}
func TestFlushDeadlineDoesNotReplaceResult(t *testing.T) {
	base, _ := setup(t)
	b := &failingFlush{OTelBackend: base.backend.(*OTelBackend)}
	var reported error
	tr, err := New(WithLocalTracing(true), WithBackend(b), WithFlushTimeout(time.Millisecond), WithErrorHandler(func(err error) { reported = err }))
	if err != nil {
		t.Fatal(err)
	}
	business := errors.New("business")
	got, err := WrapHandler(tr, func(context.Context, int) (int, error) { return 7, business })(context.Background(), 0)
	if got != 7 || err != business || !b.deadline || !errors.Is(reported, context.DeadlineExceeded) {
		t.Fatal(got, err, reported)
	}
}
func TestDisabledAndProviderOwnership(t *testing.T) {
	tr, exporter := setup(t)
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, end := tr.StartSpan(context.Background(), "still alive")
	end(nil)
	end(errors.New("duplicate"))
	if len(exporter.GetSpans()) != 1 {
		t.Fatal("provider closed or end duplicated")
	}
	t.Setenv("POWERTOOLS_TRACE_ENABLED", "false")
	disabled, err := New(WithLocalTracing(true))
	if err != nil || disabled.Enabled() || disabled.Backend() != nil {
		t.Fatal(disabled, err)
	}
}
