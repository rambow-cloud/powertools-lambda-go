package logger

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestOTelBufferingCorrelationAndCleanup(t *testing.T) {
	for _, flags := range []trace.TraceFlags{0, trace.FlagsSampled} {
		t.Run(flags.String(), func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("_X_AMZN_TRACE_ID", "")
			id, _ := trace.TraceIDFromHex("65abcdef123456789012345678901234")
			spanID, _ := trace.SpanIDFromHex("1234567890123456")
			ctx := trace.ContextWithRemoteSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: id, SpanID: spanID, TraceFlags: flags}))
			var output bytes.Buffer
			l := New(WithOutput(&output), WithBuffer(BufferOptions{}))
			business := errors.New("business failure")
			var late *Logger
			handler := WrapHandler(l, func(ctx context.Context, fail bool) (int, error) {
				late = l.WithContext(ctx)
				late.SetCorrelationID("request")
				if late.GetCorrelationID() != "request" || late.GetLevelName() != "INFO" {
					t.Fatal("invocation getters lost state")
				}
				if err := late.Debug("buffered OTel diagnostic"); err != nil {
					t.Fatal(err)
				}
				if output.Len() != 0 {
					t.Fatal("debug record emitted before flush")
				}
				if fail {
					return 7, business
				}
				return 7, nil
			}, HandlerOptions{FlushBufferOnError: true})
			if result, err := handler(ctx, false); result != 7 || err != nil || output.Len() != 0 {
				t.Fatal("successful invocation retained or emitted its buffer")
			}
			if result, err := handler(ctx, true); result != 7 || err != business {
				t.Fatal("handler result changed")
			}
			items := records(t, &output)
			if len(items) != 2 || items[0]["message"] != "buffered OTel diagnostic" || items[0]["trace_id"] != id.String() || items[0]["span_id"] != spanID.String() || items[0]["xray_trace_id"] != "1-65abcdef-123456789012345678901234" {
				t.Fatal(items)
			}
			if !errors.Is(late.Info("late"), ErrInvocationClosed) || l.GetCorrelationID() != nil {
				t.Fatal("invocation state escaped cleanup")
			}
			output.Reset()
			t.Setenv("_X_AMZN_TRACE_ID", "Root=stale-runtime-trace")
			if err := l.WithContext(ctx).Info("prefer OTel"); err != nil {
				t.Fatal(err)
			}
			if records(t, &output)[0]["xray_trace_id"] != "1-65abcdef-123456789012345678901234" {
				t.Fatal("runtime header overrode active OTel context")
			}
		})
	}
}

func TestLoggerConfigurationGetters(t *testing.T) {
	cleanEnv(t)
	t.Setenv("POWERTOOLS_LOGGER_LOG_EVENT", "true")
	l := New(WithPersistentKeys(Fields{"correlation_id": "persistent"}))
	if !l.GetLogEvent() || l.GetCorrelationID() != nil {
		t.Fatal("getter semantics differ from reference")
	}
	l.SetLevel(WarnLevel)
	if l.GetLevelName() != "WARN" {
		t.Fatal(l.GetLevelName())
	}
}
