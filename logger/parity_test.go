package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

type marshaledDocument map[string]any

func (marshaledDocument) MarshalJSON() ([]byte, error) { return []byte(`{"custom":true}`), nil }

func TestEmptyAttributeOwnershipAndGoValues(t *testing.T) {
	cleanEnv(t)
	var output bytes.Buffer
	var pointer *string
	var nilMap map[string]any
	var nilSlice []string
	type label string
	document := map[label]any{
		"empty": label(""), "pointer": pointer, "map": nilMap, "slice": nilSlice,
		"zero": 0, "false": false, "list": []any{}, "object": Fields{},
		"nested": Fields{"empty": "", "null": nil},
	}
	visited := map[string]int{}
	l := New(WithOutput(&output), WithFormatter(func(Fields) (any, error) {
		return document, nil
	}), WithReplacer(func(key string, value any) any {
		if key == "" {
			if _, ok := value.(map[label]any); !ok {
				t.Fatalf("cleanup changed formatter map type: %T", value)
			}
		}
		visited[key]++
		return value
	}), WithRecordOrder("false", "zero"))
	if err := l.Info("ignored"); err != nil {
		t.Fatal(err)
	}
	got := records(t, &output)[0]
	for _, key := range []string{"pointer", "map", "slice"} {
		if _, ok := got[key]; ok || visited[key] != 0 {
			t.Fatalf("empty top-level value reached output/replacer: %s", key)
		}
	}
	if visited["empty"] != 1 || got["zero"] != float64(0) || got["false"] != false || len(got) != 5 {
		t.Fatalf("incorrect shallow cleanup: %v; callbacks: %v", got, visited)
	}
	if !reflect.DeepEqual(got["nested"], map[string]any{"empty": "", "null": nil}) || !bytes.HasPrefix(output.Bytes(), []byte(`{"false":false,"zero":0`)) {
		t.Fatal(output.String())
	}
	if len(document) != 9 || document["empty"] != label("") || document["nested"].(Fields)["null"] != nil {
		t.Fatal("cleanup mutated formatter-owned data")
	}
	output.Reset()
	l = New(WithOutput(&output), WithFormatter(func(Fields) (any, error) { return marshaledDocument{}, nil }))
	if err := l.Info("custom marshaler"); err != nil || output.String() != "{\"custom\":true}\n" {
		t.Fatalf("custom map marshaler changed: %s; %v", output.String(), err)
	}
	var nilChannel chan int
	l = New(WithOutput(io.Discard))
	if err := l.Info("unsupported Go value", Fields{"channel": nilChannel}); err == nil {
		t.Fatal("unsupported Go value lost its serialization error")
	}
}

func TestChildTemporaryKeysInInvocation(t *testing.T) {
	cleanEnv(t)
	var output bytes.Buffer
	root := New(WithOutput(&output), WithPersistentKeys(Fields{"shared": "parent"}))
	var retained *Logger
	handler := WrapHandler(root, func(ctx context.Context, request int) (int, error) {
		parent := root.WithContext(ctx)
		parent.AppendKeys(Fields{"shared": "temporary", "request": request})
		child := parent.Child(WithPersistentKeys(Fields{"shared": "child"}))
		retained = child
		if child.PersistentKeys()["request"] != nil || child.PersistentKeys()["shared"] != "child" {
			t.Fatal("temporary keys entered child persistent store")
		}
		if err := child.Info("before"); err != nil {
			return 0, err
		}
		child.ResetKeys()
		if err := child.Info("after"); err != nil {
			return 0, err
		}
		return request, parent.Info("parent")
	})
	for request := 1; request <= 2; request++ {
		if result, err := handler(context.Background(), request); err != nil || result != request {
			t.Fatal(result, err)
		}
		if !errors.Is(retained.Info("late"), ErrInvocationClosed) {
			t.Fatal("child escaped invocation closure")
		}
	}
	got := records(t, &output)
	if len(got) != 6 {
		t.Fatal(got)
	}
	for index := 0; index < 6; index += 3 {
		if got[index]["shared"] != "temporary" || got[index+1]["shared"] != "child" || got[index+1]["request"] != nil || got[index+2]["shared"] != "temporary" {
			t.Fatal(got)
		}
	}
}

func paritySpanContext(traceHex, spanHex string, sampled bool) context.Context {
	id, _ := trace.TraceIDFromHex(traceHex)
	span, _ := trace.SpanIDFromHex(spanHex)
	var flags trace.TraceFlags
	if sampled {
		flags = trace.FlagsSampled
	}
	return trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: id, SpanID: span, TraceFlags: flags}))
}

func TestOTelBufferTraceIdentity(t *testing.T) {
	for _, sampled := range []bool{false, true} {
		t.Run(fmt.Sprint(sampled), func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("_X_AMZN_TRACE_ID", "Root=stale-runtime-trace")
			var output bytes.Buffer
			root := New(WithOutput(&output), WithBuffer(BufferOptions{}))
			const firstTrace = "12345678123456789012345678901234"
			first := root.WithContext(paritySpanContext(firstTrace, "1234567890123456", sampled))
			sameTrace := root.WithContext(paritySpanContext(firstTrace, "8765432109876543", sampled))
			other := root.WithContext(paritySpanContext("87654321123456789012345678901234", "1234567890123456", sampled))
			if err := first.Debug("first span"); err != nil {
				t.Fatal(err)
			}
			other.ClearBuffer()
			if err := other.FlushBuffer(); err != nil || output.Len() != 0 {
				t.Fatalf("wrong OTel trace emitted records: %s; %v", &output, err)
			}
			if err := sameTrace.Debug("second span"); err != nil {
				t.Fatal(err)
			}
			if err := sameTrace.FlushBuffer(); err != nil {
				t.Fatal(err)
			}
			got := records(t, &output)
			if len(got) != 2 || got[0]["span_id"] != "1234567890123456" || got[1]["span_id"] != "8765432109876543" {
				t.Fatal(got)
			}
			output.Reset()
			if err := first.Debug("old trace"); err != nil {
				t.Fatal(err)
			}
			if err := other.Debug("new trace"); err != nil {
				t.Fatal(err)
			}
			if err := other.FlushBuffer(); err != nil {
				t.Fatal(err)
			}
			got = records(t, &output)
			if len(got) != 1 || got[0]["message"] != "new trace" {
				t.Fatal(got)
			}
		})
	}
}

func TestConcurrentBufferedInvocationsAndChildren(t *testing.T) {
	cleanEnv(t)
	var output bytes.Buffer
	root := New(WithOutput(&output), WithBuffer(BufferOptions{}))
	var retained []*Logger
	var retainedMu sync.Mutex
	business := errors.New("business failure")
	handler := WrapHandler(root, func(ctx context.Context, request int) (int, error) {
		parent := root.WithContext(ctx)
		parent.AppendKeys(Fields{"request": request})
		child := parent.Child(WithPersistentKeys(Fields{"component": "child"}))
		retainedMu.Lock()
		retained = append(retained, child)
		retainedMu.Unlock()
		if err := parent.Debug("parent detail"); err != nil {
			return 0, err
		}
		if err := child.Debug("child detail"); err != nil {
			return 0, err
		}
		if request%2 == 0 {
			return request, business
		}
		return request, nil
	}, HandlerOptions{FlushBufferOnError: true})
	var wg sync.WaitGroup
	for request := 0; request < 40; request++ {
		wg.Go(func() {
			ctx := paritySpanContext(fmt.Sprintf("%032x", request+1), "1234567890123456", request%3 == 0)
			result, err := handler(ctx, request)
			if result != request || (request%2 == 0 && err != business) || (request%2 != 0 && err != nil) {
				t.Errorf("request %d: result=%d error=%v", request, result, err)
			}
		})
	}
	wg.Wait()
	got := records(t, &output)
	if len(got) != 80 {
		t.Fatalf("got %d records, want 80", len(got))
	}
	counts := map[int]int{}
	for _, record := range got {
		request := int(record["request"].(float64))
		if request%2 != 0 || record["trace_id"] != fmt.Sprintf("%032x", request+1) {
			t.Fatalf("invocation buffer leaked: %v", record)
		}
		counts[request]++
	}
	for request := 0; request < 40; request += 2 {
		if counts[request] != 4 {
			t.Fatalf("request %d emitted %d records", request, counts[request])
		}
	}
	for _, child := range retained {
		if !errors.Is(child.FlushBuffer(), ErrInvocationClosed) || !errors.Is(child.Debug("late"), ErrInvocationClosed) {
			t.Fatal("closed child accepted a buffer operation")
		}
	}
}

type parityFailWriter struct {
	err   error
	calls int
}

func (w *parityFailWriter) Write([]byte) (int, error) { w.calls++; return 0, w.err }

func TestBufferOutputErrorsPreserveHandlerResult(t *testing.T) {
	cleanEnv(t)
	ctx := paritySpanContext("12345678123456789012345678901234", "1234567890123456", false)
	want := errors.New("sink failed")
	writer := &parityFailWriter{err: want}
	root := New(WithOutput(writer), WithBuffer(BufferOptions{MaxBytes: 10}))
	if err := root.WithContext(ctx).Debug("oversize"); !errors.Is(err, want) || writer.calls != 2 {
		t.Fatalf("overflow writes: calls=%d error=%v", writer.calls, err)
	}
	var diagnostics []error
	root = New(WithOutput(writer), WithBuffer(BufferOptions{}), WithErrorHandler(func(err error) { diagnostics = append(diagnostics, err) }))
	business := errors.New("business failed")
	handler := WrapHandler(root, func(ctx context.Context, value int) (int, error) {
		if err := root.WithContext(ctx).Debug("detail"); err != nil {
			t.Fatal(err)
		}
		return value, business
	}, HandlerOptions{FlushBufferOnError: true})
	if result, err := handler(ctx, 7); result != 7 || err != business || len(diagnostics) != 2 {
		t.Fatalf("result=%d error=%v diagnostics=%v", result, err, diagnostics)
	}
	for _, err := range diagnostics {
		if !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
}
