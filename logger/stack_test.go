package logger

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"go.opentelemetry.io/otel/trace"
)

type previewError struct{ code int }

func (e *previewError) Error() string { return "payment declined" }

func stackOrigin(err error) error { return WithStack(err) }

func errorDocument(t *testing.T, err error, options ...Option) Fields {
	t.Helper()
	var out bytes.Buffer
	l := New(append([]Option{WithOutput(&out)}, options...)...)
	if logErr := l.Error("failed", err); logErr != nil {
		t.Fatal(logErr)
	}
	return Fields(records(t, &out)[0]["error"].(map[string]any))
}

func TestWithStackIdentityAndSerialization(t *testing.T) {
	cleanEnv(t)
	if WithStack(nil) != nil {
		t.Fatal("nil changed")
	}
	base := &previewError{42}
	attached := stackOrigin(base)
	var typed *previewError
	if attached.Error() != base.Error() || !errors.Is(attached, base) || !errors.As(attached, &typed) || typed != base || errors.Unwrap(attached) != base {
		t.Fatal("error identity changed")
	}
	wrapped := fmt.Errorf("checkout: %w", attached)
	if WithStack(attached) != attached || WithStack(wrapped) != wrapped {
		t.Fatal("existing single-chain stack replaced")
	}
	doc := errorDocument(t, attached)
	if doc["name"] != "*logger.previewError" || doc["cause"] != nil || doc["message"] != base.Error() || doc["stack_source"] != "attached" || !strings.Contains(doc["stack"].(string), ".stackOrigin\n") {
		t.Fatal(doc)
	}
	if !strings.Contains(doc["location"].(string), "stack_test.go:") || !strings.Contains(doc["stack"].(string), "\t"+doc["location"].(string)) {
		t.Fatal("location does not match first frame", doc)
	}
	doc = errorDocument(t, wrapped)
	cause := doc["cause"].(map[string]any)
	if doc["name"] != "*fmt.wrapError" || cause["name"] != "*logger.previewError" || cause["cause"] != nil || cause["stack"] != doc["stack"] {
		t.Fatal("transparent wrapper or %w lost", doc)
	}
}

func TestJoinedStacksAndLazyFallback(t *testing.T) {
	cleanEnv(t)
	a := stackOrigin(errors.New("a"))
	b := errors.New("b")
	join := errors.Join(a, b)
	doc := errorDocument(t, join)
	causes := doc["cause"].([]any)
	left, right := causes[0].(map[string]any), causes[1].(map[string]any)
	if doc["stack_source"] != "logging" || left["stack_source"] != "attached" || right["stack"] != doc["stack"] || doc["location"] == left["location"] {
		t.Fatal("join branches conflated", doc)
	}
	aggregation := WithStack(join)
	if !errors.Is(aggregation, b) || errorDocument(t, aggregation)["stack_source"] != "attached" {
		t.Fatal("explicit aggregation stack missing")
	}
	// Directly inspect lazy state rather than adding instrumentation to runtime.Callers.
	n := normalizer{active: map[visit]bool{}, stacks: true}
	if _, err := n.normalize("", Fields{"plain": 1, "nil": nil}, 0); err != nil || n.fallback != nil {
		t.Fatal("no-error document captured a stack", err)
	}
	if _, err := n.normalize("", a, 0); err != nil || n.fallback != nil {
		t.Fatal("attached error captured a fallback", err)
	}
	if _, err := n.normalize("", []any{b, b}, 0); err != nil || n.fallback == nil {
		t.Fatal("fallback absent", err)
	}
	first := n.fallback
	if _, err := n.normalize("", b, 0); err != nil || n.fallback != first {
		t.Fatal("fallback recaptured", err)
	}
}

func recursiveStack(depth int) error {
	if depth == 0 {
		return WithStack(errors.New("deep"))
	}
	err := recursiveStack(depth - 1)
	runtime.KeepAlive(depth)
	return err
}

func TestStackTruncationOptionsAndHooks(t *testing.T) {
	cleanEnv(t)
	doc := errorDocument(t, recursiveStack(80))
	if doc["stack_truncated"] != true || strings.Count(doc["stack"].(string), "\n\t") != 64 {
		t.Fatal("frame cap failed", doc)
	}
	plain := errors.New("plain")
	doc = errorDocument(t, plain)
	if doc["stack_source"] != "logging" || doc["stack_truncated"] != nil || !strings.Contains(doc["stack"].(string), ".errorDocument\n") {
		t.Fatal(doc)
	}
	doc = errorDocument(t, WithStack(plain), WithStackTrace(false))
	if len(doc) != 3 || doc["location"] != "" || doc["name"] != "*errors.errorString" {
		t.Fatal("opt out changed legacy schema", doc)
	}
	var out bytes.Buffer
	parent := New(WithOutput(&out), WithStackTrace(false))
	child := parent.Child(WithStackTrace(true), WithReplacer(func(key string, value any) any {
		if key == "stack" || key == "location" {
			return "[REDACTED]"
		}
		return value
	}), WithFormatter(func(fields Fields) (any, error) {
		if _, ok := fields["error"].(error); !ok {
			t.Fatal("formatter no longer receives raw errors")
		}
		return Fields{"nested": []any{fields["error"], Fields{"another": plain}}}, nil
	}))
	if err := child.Error("child", plain); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "\n") != 1 || strings.Count(out.String(), `"stack":"[REDACTED]"`) != 2 || strings.Contains(out.String(), "stack_test.go") {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := parent.Error("parent", plain); err != nil || strings.Contains(out.String(), `"stack"`) {
		t.Fatal(err, out.String())
	}
}

type unobservedError struct{}

func (unobservedError) Error() string { panic("filtered error evaluated") }
func (unobservedError) Unwrap() error { panic("filtered chain evaluated") }

func TestFilteredErrorAndExactCaller(t *testing.T) {
	cleanEnv(t)
	var out bytes.Buffer
	l := New(WithOutput(&out))
	if err := l.Debug("filtered", unobservedError{}); err != nil || out.Len() != 0 {
		t.Fatal(err, out.String())
	}
	_, file, line, _ := runtime.Caller(0)
	err := WithStack(errors.New("source"))
	stack, _ := attachedStack(err)
	if stack.location != fmt.Sprintf("%s:%d", file, line+1) {
		t.Fatal("attached caller off by one", stack.location)
	}
	_, file, line, _ = runtime.Caller(0)
	_ = l.Error("logging site", errors.New("plain"))
	doc := records(t, &out)[0]["error"].(map[string]any)
	if doc["location"] != fmt.Sprintf("%s:%d", file, line+1) {
		t.Fatal("logging caller off by one", doc)
	}
}

func TestDeepNestedErrorStillFindsApplicationFrame(t *testing.T) {
	cleanEnv(t)
	var value any = errors.New("nested")
	for i := 0; i < 90; i++ {
		value = Fields{"child": value}
	}
	var out bytes.Buffer
	l := New(WithOutput(&out))
	if err := l.Info("nested", Fields{"payload": value}); err != nil {
		t.Fatal(err)
	}
	nested := records(t, &out)[0]["payload"].(map[string]any)
	for i := 0; i < 89; i++ {
		nested = nested["child"].(map[string]any)
	}
	doc := nested["child"].(map[string]any)
	if !strings.Contains(doc["stack"].(string), ".TestDeepNestedErrorStillFindsApplicationFrame\n") {
		t.Fatal(doc)
	}
}

type firstPanicWriter struct {
	bytes.Buffer
	calls int
}

func (w *firstPanicWriter) Write(data []byte) (int, error) {
	w.calls++
	if w.calls == 1 {
		panic("buffer sink")
	}
	return w.Buffer.Write(data)
}

func TestPanicTerminalRecordBypassesBuffer(t *testing.T) {
	cleanEnv(t)
	var out firstPanicWriter
	l := New(WithOutput(&out), WithBuffer(BufferOptions{BufferAt: ErrorLevel}))
	token := errors.New("original")
	h := WrapHandler(l, func(ctx context.Context, _ int) (int, error) {
		_ = l.WithContext(ctx).Debug("buffered")
		panic(token)
	})
	ctx := paritySpanContext("12345678123456789012345678901234", "1234567890123456", false)
	if recoverValue(func() { _, _ = h(ctx, 0) }) != token {
		t.Fatal("original lost")
	}
	items := records(t, &out.Buffer)
	if out.calls != 2 || len(items) != 1 || items[0]["message"] != "Unhandled panic" {
		t.Fatal(out.calls, items)
	}
}

func recoverValue(action func()) (value any) {
	defer func() { value = recover() }()
	action()
	return nil
}

func TestPreviewPanicValuesAndNestedWrappers(t *testing.T) {
	for _, value := range []any{"broken", 17, &previewError{9}, &struct{ ID int }{7}, nil} {
		for _, raw := range []bool{false, true} {
			t.Run(fmt.Sprintf("%T/raw=%v", value, raw), func(t *testing.T) {
				cleanEnv(t)
				var out bytes.Buffer
				l := New(WithOutput(&out))
				var late *Logger
				business := func(ctx context.Context, _ int) (int, error) {
					late = l.WithContext(ctx)
					late.AppendKeys(Fields{"order_id": "order-1"})
					panic(value)
				}
				inner := WrapHandler(l.Child(), business)
				outer := WrapHandler(l, inner, HandlerOptions{CorrelationID: func(any) any { return "corr-1" }})
				ctx := lambdacontext.NewContext(context.Background(), &lambdacontext.LambdaContext{AwsRequestID: "req-1"})
				got := recoverValue(func() {
					if raw {
						_, _ = lambda.NewHandler(WrapRawHandler(l, outer)).Invoke(ctx, []byte(`1`))
					} else {
						_, _ = outer(ctx, 1)
					}
				})
				if value != nil && got != value {
					t.Fatal("panic value replaced", got)
				}
				if value == nil {
					if _, ok := got.(*runtime.PanicNilError); !ok {
						t.Fatalf("panic(nil) not retained: %T", got)
					}
				}
				items := records(t, &out)
				if len(items) != 1 || items[0]["message"] != "Unhandled panic" || items[0]["function_request_id"] != "req-1" || items[0]["level"] != "ERROR" || items[0]["order_id"] != "order-1" {
					t.Fatal(items)
				}
				doc := items[0]["error"].(map[string]any)
				if doc["stack_source"] != "panic" || !strings.Contains(doc["location"].(string), "stack_test.go:") || strings.Count(out.String(), "\n") != 1 {
					t.Fatal(doc, out.String())
				}
				if !errors.Is(late.Info("late"), ErrInvocationClosed) {
					t.Fatal("scope not closed")
				}
			})
		}
	}
}

func TestPanicChainReusesPanicSnapshot(t *testing.T) {
	cleanEnv(t)
	var out bytes.Buffer
	l := New(WithOutput(&out))
	token := fmt.Errorf("outer: %w", errors.New("inner"))
	h := WrapHandler(l, func(context.Context, int) (int, error) { panic(token) })
	if recoverValue(func() { _, _ = h(context.Background(), 0) }) != token {
		t.Fatal("panic changed")
	}
	doc := records(t, &out)[0]["error"].(map[string]any)
	cause := doc["cause"].(map[string]any)
	if cause["stack_source"] != "panic" || cause["stack"] != doc["stack"] {
		t.Fatal(doc)
	}
}

type previewWriter struct{ panicOnWrite bool }

func (w previewWriter) Write([]byte) (int, error) {
	if w.panicOnWrite {
		panic("writer panic")
	}
	return 0, io.ErrClosedPipe
}

func TestPanicDiagnosticFailuresAndBufferPolicy(t *testing.T) {
	for _, mode := range []string{"write-error", "write-panic", "formatter-error", "formatter-panic", "replacer-panic", "callback-panic", "buffer-panic", "disabled", "stacks-disabled", "flush", "discard"} {
		t.Run(mode, func(t *testing.T) {
			cleanEnv(t)
			var out bytes.Buffer
			callbacks := 0
			opts := []Option{WithOutput(&out), WithBuffer(BufferOptions{DisableFlushOnError: mode == "discard"}), WithErrorHandler(func(error) {
				callbacks++
				if mode == "callback-panic" {
					panic("callback")
				}
			})}
			if mode == "write-error" || mode == "write-panic" || mode == "callback-panic" || mode == "buffer-panic" {
				opts = append(opts, WithOutput(previewWriter{mode == "write-panic" || mode == "buffer-panic"}))
			}
			if mode == "formatter-error" || mode == "formatter-panic" {
				opts = append(opts, WithFormatter(func(f Fields) (any, error) {
					if f["level"] == "ERROR" {
						if mode == "formatter-panic" {
							panic("formatter")
						}
						return nil, io.ErrUnexpectedEOF
					}
					return f, nil
				}))
			}
			if mode == "replacer-panic" {
				opts = append(opts, WithReplacer(func(k string, v any) any {
					if k == "stack" {
						panic("replacer")
					}
					return v
				}))
			}
			if mode == "disabled" {
				opts = append(opts, WithPanicLogging(false), WithStackTrace(false))
			}
			if mode == "stacks-disabled" {
				opts = append(opts, WithStackTrace(false))
			}
			l := New(opts...)
			var late *Logger
			token := &struct{}{}
			h := WrapHandler(l, func(ctx context.Context, _ int) (int, error) {
				late = l.WithContext(ctx)
				_ = late.Debug("buffered detail")
				panic(token)
			})
			ctx := paritySpanContext("12345678123456789012345678901234", "1234567890123456", false)
			if got := recoverValue(func() { _, _ = h(ctx, 0) }); got != token {
				t.Fatal("original panic lost", got)
			}
			if !errors.Is(late.FlushBuffer(), ErrInvocationClosed) {
				t.Fatal("scope escaped")
			}
			if mode == "formatter-error" && callbacks != 1 {
				t.Fatal("diagnostic error not reported", callbacks)
			}
			if mode == "disabled" && out.Len() != 0 {
				t.Fatal(out.String())
			}
			if mode == "flush" || mode == "discard" || mode == "stacks-disabled" {
				items := records(t, &out)
				want := 2
				if mode == "discard" {
					want = 1
				}
				if len(items) != want || items[len(items)-1]["message"] != "Unhandled panic" {
					t.Fatal(items)
				}
				if mode == "stacks-disabled" && items[len(items)-1]["error"].(map[string]any)["stack"] != nil {
					t.Fatal(items)
				}
			}
		})
	}
}

func TestPreviewConcurrentPanicAndOTel(t *testing.T) {
	cleanEnv(t)
	var out bytes.Buffer
	l := New(WithOutput(&out), WithBuffer(BufferOptions{}))
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Go(func() {
			h := WrapHandler(l, func(ctx context.Context, _ int) (int, error) {
				span := trace.SpanContextFromContext(paritySpanContext("12345678123456789012345678901234", "1234567890123456", false))
				request := l.WithContext(trace.ContextWithSpanContext(ctx, span))
				request.SetCorrelationID("corr")
				_ = request.Debug("detail", Fields{"nested": []any{WithStack(errors.New("nested")), errors.New("other")}})
				panic("failure")
			})
			_ = recoverValue(func() { _, _ = h(context.Background(), 0) })
		})
	}
	wg.Wait()
	items := records(t, &out)
	if len(items) != 48 {
		t.Fatal("duplicate or missing records", len(items))
	}
	for _, item := range items {
		if item["trace_id"] != "12345678123456789012345678901234" || item["correlation_id"] != "corr" {
			t.Fatal(item)
		}
	}
	// Returning an ordinary error does not opt into implicit application logging.
	out.Reset()
	business := errors.New("return only")
	h := WrapRawHandler(l, func(context.Context, jsonv1.RawMessage) (int, error) { return 0, business })
	if _, err := h(context.Background(), jsonv1.RawMessage(`{}`)); err != business || out.Len() != 0 {
		t.Fatal(err, out.String())
	}
}
