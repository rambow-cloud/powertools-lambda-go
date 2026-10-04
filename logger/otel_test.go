package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestHandlerStartedTraceBufferCleanup(t *testing.T) {
	for _, sampled := range []bool{false, true} {
		for _, initialTrace := range []string{"none", "runtime", "otel"} {
			for _, outcome := range []string{"success", "error", "panic", "no-flush"} {
				t.Run(fmt.Sprintf("sampled=%v/%s/%s", sampled, initialTrace, outcome), func(t *testing.T) {
					cleanEnv(t)
					ctx := context.Background()
					if initialTrace == "runtime" {
						t.Setenv("_X_AMZN_TRACE_ID", "Root=1-87654321-123456789012345678901234")
					} else if initialTrace == "otel" {
						ctx = paritySpanContext("87654321123456789012345678901234", "8765432109876543", sampled)
					}
					var output bytes.Buffer
					root := New(WithOutput(&output), WithBuffer(BufferOptions{}))
					business := errors.New("business failure")
					panicValue := &struct{}{}
					var parent, child *Logger
					handler := WrapHandler(root, func(ctx context.Context, value int) (int, error) {
						parent = root.WithContext(ctx)
						child = parent.Child()
						// Attach a span after both loggers have joined the invocation scope.
						span := trace.SpanContextFromContext(paritySpanContext("12345678123456789012345678901234", "1234567890123456", sampled))
						traced := trace.ContextWithSpanContext(ctx, span)
						parent = parent.WithContext(traced)
						child = child.WithContext(traced)
						for _, l := range []*Logger{parent, child} {
							if err := l.Debug("handler trace detail"); err != nil {
								t.Fatal(err)
							}
						}
						if output.Len() != 0 {
							t.Fatal("buffered diagnostics emitted before cleanup")
						}
						if outcome == "panic" {
							panic(panicValue)
						}
						if outcome != "success" {
							return value, business
						}
						return value, nil
					}, HandlerOptions{FlushBufferOnError: outcome != "no-flush"})
					func() {
						defer func() {
							failure := recover()
							if (outcome == "panic" && failure != panicValue) || (outcome != "panic" && failure != nil) {
								t.Fatalf("panic changed: %v", failure)
							}
						}()
						result, err := handler(ctx, 7)
						if result != 7 || (outcome == "success" && err != nil) || (outcome != "success" && err != business) {
							t.Fatalf("handler result changed: result=%d error=%v", result, err)
						}
					}()
					if outcome == "error" || outcome == "panic" {
						items := records(t, &output)
						if len(items) != 4 {
							t.Fatalf("got %d records, want 4: %v", len(items), items)
						}
						for index := 0; index < len(items); index += 2 {
							if items[index]["message"] != "handler trace detail" || items[index]["trace_id"] != "12345678123456789012345678901234" || items[index+1]["level"] != "ERROR" {
								t.Fatalf("buffered diagnostic changed or reordered: %v", items)
							}
						}
					} else if output.Len() != 0 {
						t.Fatalf("cleanup flushed without opt-in failure: %s", &output)
					}
					for _, l := range []*Logger{parent, child} {
						if !errors.Is(l.FlushBuffer(), ErrInvocationClosed) || !errors.Is(l.Debug("late"), ErrInvocationClosed) {
							t.Fatal("invocation logger escaped closure")
						}
					}
				})
			}
		}
	}
}

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
