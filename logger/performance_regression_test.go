package logger

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestOrderedEncodingPreservesFormatterInput(t *testing.T) {
	cleanEnv(t)
	for _, plainMap := range []bool{false, true} {
		fields := Fields{"z": 3, "message": "formatted", "empty": "", "nil": nil, "a\"\n": "中\n文", "nested": Fields{"empty": "", "nil": nil}}
		var input any = fields
		if plainMap {
			input = map[string]any(fields)
		}
		var out bytes.Buffer
		rootCalls := 0
		log := New(WithOutput(&out), WithRecordOrder("z", "missing", "z", "message", "a\"\n"),
			WithFormatter(func(Fields) (any, error) { return input, nil }),
			WithReplacer(func(key string, value any) any {
				if key == "" {
					rootCalls++
					if reflect.TypeOf(value) != reflect.TypeOf(input) {
						t.Fatalf("root type changed from %T to %T", input, value)
					}
				}
				return value
			}))
		for range 2 {
			if err := log.Info("original"); err != nil {
				t.Fatal(err)
			}
		}
		want := "{\"z\":3,\"message\":\"formatted\",\"a\\\"\\n\":\"中\\n文\",\"nested\":{"
		lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
		if len(lines) != 2 || !strings.HasPrefix(lines[0], want) || !strings.HasPrefix(lines[1], want) || rootCalls != 2 {
			t.Fatalf("order, framing or replacer changed: calls=%d output=%s", rootCalls, &out)
		}
		for _, record := range records(t, &out) {
			nested := record["nested"].(map[string]any)
			if len(record) != 4 || nested["empty"] != "" || len(nested) != 2 {
				t.Fatalf("omission rules changed: %v", record)
			}
		}
		if len(fields) != 6 || fields["empty"] != "" || fields["nil"] != nil || fields["message"] != "formatted" {
			t.Fatal("encoder consumed formatter-owned fields")
		}
	}
}

type partialDocumentMarshaler struct{ calls int }

var partialDocumentError = errors.New("failed after writing a large JSON value")

func (m *partialDocumentMarshaler) MarshalJSONTo(enc *jsontext.Encoder) error {
	m.calls++
	if err := enc.WriteToken(jsontext.String(strings.Repeat("x", 8192))); err != nil {
		return err
	}
	return partialDocumentError
}

func TestOrderedEncoderFailureRemainsAtomic(t *testing.T) {
	cleanEnv(t)
	var out bytes.Buffer
	value := &partialDocumentMarshaler{}
	log := New(WithOutput(&out), WithRecordOrder("before", "failure", "after"))
	err := log.Info("failure", Fields{"before": strings.Repeat("y", 8192), "failure": value, "after": "unwritten"})
	var semantic *json.SemanticError
	if !errors.Is(err, partialDocumentError) || !errors.As(err, &semantic) || value.calls != 1 || out.Len() != 0 {
		t.Fatalf("partial output or changed failure: writes=%d bytes=%d error=%v", value.calls, out.Len(), err)
	}
	if err := log.Info("recovered"); err != nil || len(records(t, &out)) != 1 {
		t.Fatalf("failed encoder poisoned subsequent writes: %v", err)
	}
}

type traceLookupContext struct {
	context.Context
	header  string
	lookups int
}

func (ctx *traceLookupContext) Value(key any) any {
	if text, ok := key.(string); ok && text == "x-amzn-trace-id" {
		ctx.lookups++
		return ctx.header
	}
	return ctx.Context.Value(key)
}

func TestUnbufferedTraceLookupAndFiltering(t *testing.T) {
	cleanEnv(t)
	ctx := &traceLookupContext{Context: context.Background(), header: "Root=1-11111111-111111111111111111111111;Sampled=0"}
	var out bytes.Buffer
	log := New(WithOutput(&out)).WithContext(ctx)
	fields := Fields{"value": 1}
	allocs := testing.AllocsPerRun(100, func() {
		if err := log.Debug("filtered", fields); err != nil {
			panic(err)
		}
	})
	if ctx.lookups != 0 || out.Len() != 0 || allocs != 0 {
		t.Fatalf("filtered log enriched: lookups=%d bytes=%d allocations=%v", ctx.lookups, out.Len(), allocs)
	}
	for _, header := range []string{"Root=1-11111111-111111111111111111111111;Sampled=0", "Root=1-22222222-222222222222222222222222;Sampled=0"} {
		ctx.header = header
		if err := log.Info("visible"); err != nil {
			t.Fatal(err)
		}
	}
	got := records(t, &out)
	if ctx.lookups != 2 || got[0]["xray_trace_id"] == got[1]["xray_trace_id"] {
		t.Fatalf("trace was repeated or cached across records: lookups=%d records=%v", ctx.lookups, got)
	}
	span := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{1},
	})
	if err := log.WithContext(trace.ContextWithSpanContext(ctx, span)).Info("otel"); err != nil || ctx.lookups != 2 {
		t.Fatalf("active OTel context looked up the unused fallback: lookups=%d error=%v", ctx.lookups, err)
	}
	log.current.mu.Lock()
	log.current.closed = true
	log.current.mu.Unlock()
	if err := log.Debug("closed"); !errors.Is(err, ErrInvocationClosed) {
		t.Fatalf("filtering bypassed invocation closure: %v", err)
	}
}
