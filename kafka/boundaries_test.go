package kafka

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"testing"
)

func TestNullAndMissingFieldsBypassCodecAndParser(t *testing.T) {
	codecCalls, parserCalls := 0, 0
	field := &FieldConfig{Type: Avro, Schema: "schema", Decoder: func(context.Context, string, any, any) (any, error) {
		codecCalls++
		return "decoded", nil
	}, Parser: func(_ context.Context, value any) (ParseResult, error) {
		parserCalls++
		return ParseResult{Value: value}, nil
	}}
	c := New(Config{Key: field, Value: field})
	for _, test := range []struct {
		name   string
		record map[string]any
		key    any
		value  any
		calls  int
	}{
		{"missing", map[string]any{}, Undefined{}, Undefined{}, 0},
		{"null", map[string]any{"key": nil, "value": nil}, nil, nil, 0},
		{"empty strings", map[string]any{"key": "", "value": ""}, Undefined{}, "decoded", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := codecCalls
			event, err := c.Deserialize(context.Background(), map[string]any{"records": map[string]any{"topic": []any{test.record}}})
			if err != nil || codecCalls != before {
				t.Fatalf("eager decode: %v, calls=%d", err, codecCalls)
			}
			key, keyErr := event.Records[0].Key(context.Background())
			value, valueErr := event.Records[0].Value(context.Background())
			if keyErr != nil || valueErr != nil || !reflect.DeepEqual(key, test.key) || !reflect.DeepEqual(value, test.value) {
				t.Fatalf("key=%v/%v, value=%v/%v", key, keyErr, value, valueErr)
			}
			if codecCalls-before != test.calls || parserCalls != codecCalls {
				t.Fatalf("unexpected callback counts: codec=%d, parser=%d", codecCalls, parserCalls)
			}
		})
	}
}

func TestParserRejectionAndErrorsRemainFieldLocal(t *testing.T) {
	cause := errors.New("application parser failure")
	for _, operational := range []bool{false, true} {
		calls := 0
		c := New(Config{Value: &FieldConfig{Type: JSON, Parser: func(context.Context, any) (ParseResult, error) {
			calls++
			if operational {
				return ParseResult{}, cause
			}
			return ParseResult{Value: "discarded", Issues: []any{}}, nil
		}}})
		event, err := c.Deserialize(context.Background(), map[string]any{"records": map[string]any{"topic": []any{
			map[string]any{"key": "a2V5", "value": "MQ==", "headers": false},
		}}})
		if err != nil || calls != 0 {
			t.Fatalf("parser ran eagerly: %v, %d", err, calls)
		}
		r := event.Records[0]
		for range 2 {
			value, err := r.Value(context.Background())
			var rejected *ParserError
			if value != nil || (operational && err != cause) || (!operational && (!errors.As(err, &rejected) || rejected.Issues == nil || len(rejected.Issues) != 0)) {
				t.Fatalf("parser error identity/category changed: %v, %v", value, err)
			}
		}
		key, err := r.Key(context.Background())
		if err != nil || key != "key" || calls != 2 {
			t.Fatalf("value failure affected key or cached error: %v, %v, %d", key, err, calls)
		}
		if _, err := r.Headers(context.Background()); err == nil {
			t.Fatal("invalid headers did not fail lazily")
		}
	}
}

func TestStrictJSONFailureRetainsDecodedTextAndDiagnostics(t *testing.T) {
	for _, raw := range []string{`{"a":1,"a":2}`, `{"a":`, `"\ud800"`} {
		count := 0
		ctx := context.WithValue(context.Background(), struct{}{}, "invocation")
		c := New(Config{Value: &FieldConfig{Type: JSON}, Diagnostic: func(got context.Context, _ string, err error) {
			if got != ctx || err == nil {
				t.Error("JSON diagnostic lost context or cause")
			}
			count++
		}})
		event, err := c.Deserialize(ctx, map[string]any{"records": map[string]any{"topic": []any{
			map[string]any{"value": base64.StdEncoding.EncodeToString([]byte(raw))},
		}}})
		if err != nil || count != 0 {
			t.Fatalf("JSON parsing was eager: %v, %d", err, count)
		}
		for range 2 {
			value, err := event.Records[0].Value(ctx)
			if err != nil || value != raw {
				t.Fatalf("failed JSON did not retain text: %v, %v", value, err)
			}
		}
		if count != 2 {
			t.Fatalf("diagnostics cached or omitted: %d", count)
		}
	}
}
