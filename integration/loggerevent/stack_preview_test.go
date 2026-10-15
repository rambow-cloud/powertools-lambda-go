package loggerevent_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

// TestStackPreviewSDKAndTracer exercises actual SDK decoding, the OTel exporter,
// both Logger wrappers and Tracer's panic cleanup, without an AWS endpoint.
func TestStackPreviewSDKAndTracer(t *testing.T) {
	for _, key := range []string{"POWERTOOLS_DEV", "POWERTOOLS_TRACE_ENABLED", "POWERTOOLS_LOGGER_LOG_EVENT", "POWERTOOLS_LOGGER_SAMPLE_RATE", "AWS_LAMBDA_LOG_LEVEL", "LOG_LEVEL", "_X_AMZN_TRACE_ID", "OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_TRACES_HEADERS"} {
		t.Setenv(key, "")
	}
	var exports atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" {
			t.Errorf("unexpected exporter path %s", r.URL.Path)
		}
		exports.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", server.URL+"/v1/traces")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE", "true")
	t.Setenv("OTEL_TRACES_SAMPLER", "always_on")
	tr, err := tracer.New(tracer.WithLocalTracing(true), tracer.WithErrorHandler(func(err error) { t.Error(err) }))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tr.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	var out bytes.Buffer
	log := logger.New(logger.WithServiceName("preview"), logger.WithOutput(&out), logger.WithLevel(logger.InfoLevel))
	token := errors.New("unhandled payment error")
	var late *logger.Logger
	business := func(ctx context.Context, event event) (string, error) {
		late = log.WithContext(ctx)
		late.SetCorrelationID(event.Name)
		if err := late.Error("attached", logger.WithStack(errors.New("origin"))); err != nil {
			t.Fatal(err)
		}
		if err := late.Error("fallback", errors.New("logging")); err != nil {
			t.Fatal(err)
		}
		panic(token)
	}
	pipeline := tracer.WrapHandler(tr, logger.WrapRawHandler(log, logger.WrapHandler(log.Child(), business)))
	ctx := lambdacontext.NewContext(context.Background(), &lambdacontext.LambdaContext{AwsRequestID: "preview-request"})
	func() {
		defer func() {
			if got := recover(); got != token {
				t.Fatalf("panic changed: %v", got)
			}
		}()
		_, _ = lambda.NewHandler(pipeline).Invoke(ctx, []byte(`{"name":"order-1","unknown":true}`))
	}()
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || exports.Load() == 0 {
		t.Fatalf("records=%d exports=%d: %s", len(lines), exports.Load(), out.String())
	}
	var traceID any
	for i, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			traceID = record["trace_id"]
		}
		detail := record["error"].(map[string]any)
		if traceID == nil || record["trace_id"] != traceID || record["span_id"] == nil || record["function_request_id"] != "preview-request" || record["correlation_id"] != "order-1" || detail["stack_source"] != []string{"attached", "logging", "panic"}[i] {
			t.Fatal(record)
		}
	}
	if !errors.Is(late.Info("late"), logger.ErrInvocationClosed) {
		t.Fatal("Logger scope escaped Tracer cleanup")
	}
}
