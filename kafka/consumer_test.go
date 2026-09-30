package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestLazyIsolationAndContext(t *testing.T) {
	type contextKey struct{}
	var calls atomic.Int32
	consumer := New(Config{Value: &FieldConfig{Type: JSON, Parser: func(ctx context.Context, value any) (ParseResult, error) {
		calls.Add(1)
		input := value.(map[string]any)
		if input["count"] != float64(1) {
			return ParseResult{}, errors.New("decoded values leaked across reads")
		}
		input["count"] = ctx.Value(contextKey{})
		return ParseResult{Value: input}, nil
	}}})
	const count = 64
	var ready sync.WaitGroup
	ready.Add(count)
	start := make(chan struct{})
	failures := make(chan error, count)
	for i := range count {
		go func() {
			ctx := context.WithValue(context.Background(), contextKey{}, i)
			event, err := consumer.Deserialize(ctx, json.RawMessage(`{"records":{"x":[{"value":"eyJjb3VudCI6MX0=","headers":[]}]}}`))
			ready.Done()
			<-start
			if err != nil {
				failures <- err
				return
			}
			for range 2 {
				value, err := event.Records[0].Value(ctx)
				if err != nil {
					failures <- err
					return
				}
				if value.(map[string]any)["count"] != i {
					failures <- fmt.Errorf("context leaked: %v", value)
					return
				}
			}
			failures <- nil
		}()
	}
	ready.Wait()
	if calls.Load() != 0 {
		t.Fatal("payload decoded before access")
	}
	close(start)
	for range count {
		if err := <-failures; err != nil {
			t.Error(err)
		}
	}
	if calls.Load() != 2*count {
		t.Fatalf("getter was cached: %d", calls.Load())
	}
}

func TestWrapperPreservesIdentityAndCancellation(t *testing.T) {
	sentinel := errors.New("handler failed")
	ctx := context.WithValue(context.Background(), struct{}{}, 42)
	handler := WrapHandler(New(Config{}), func(actual context.Context, event *ConsumerRecords) (string, error) {
		if actual != ctx {
			t.Fatal("context replaced")
		}
		return "result", sentinel
	})
	result, err := handler(ctx, json.RawMessage(`{"records":{"x":[{"value":"!"}]}}`))
	if result != "result" || err != sentinel {
		t.Fatal(result, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = handler(canceled, json.RawMessage(`{"records":{}}`))
	if err != context.Canceled {
		t.Fatal(err)
	}
	event, err := New(Config{}).Deserialize(ctx, json.RawMessage(`{"records":{"x":[{"key":"","value":null,"headers":null}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, getter := range []func(context.Context) (any, error){event.Records[0].Key, event.Records[0].Value} {
		if _, err := getter(canceled); err != context.Canceled {
			t.Fatal(err)
		}
	}
	if _, err := event.Records[0].Headers(canceled); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestCapturedFieldsAndRepeatedDiagnostics(t *testing.T) {
	headers := []any{map[string]any{"h": []any{float64(65)}}}
	raw := map[string]any{"key": "eA==", "value": "ew==", "headers": headers}
	input := map[string]any{"records": map[string]any{"x": []any{raw}}}
	var logs int
	c := New(Config{Value: &FieldConfig{Type: JSON}, Diagnostic: func(context.Context, string, error) { logs++ }})
	event, err := c.Deserialize(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	r := event.Records[0]
	raw["key"], r.OriginalValue, r.OriginalHeaders = "eQ==", "changed", nil
	headers[0].(map[string]any)["h"].([]any)[0] = float64(66)
	key, err := r.Key(context.Background())
	if key != "x" || err != nil {
		t.Fatal(key, err)
	}
	for range 2 {
		if value, err := r.Value(context.Background()); value != "{" || err != nil {
			t.Fatal(value, err)
		}
	}
	decoded, err := r.Headers(context.Background())
	if err != nil || decoded[0]["h"] != "B" {
		t.Fatal(decoded, err)
	}
	if logs != 2 {
		t.Fatal("diagnostic was cached", logs)
	}
	if _, ok := input["records"].(map[string]any); !ok {
		t.Fatal("input event mutated")
	}
}

func TestCodecBoundaryAndErrorIdentity(t *testing.T) {
	sentinel := errors.New("codec failed")
	metadata := map[string]any{"schemaId": "17"}
	ctx := context.Background()
	config := Config{Value: &FieldConfig{Type: Protobuf, Schema: "schema", Decoder: func(actual context.Context, value string, schema, info any) (any, error) {
		if actual != ctx || value != "eA==" || schema != "schema" || info.(map[string]any)["schemaId"] != "17" {
			t.Fatal("codec arguments changed")
		}
		return nil, sentinel
	}}}
	event, err := New(config).Deserialize(ctx, map[string]any{"records": map[string]any{"x": []any{map[string]any{"value": "eA==", "valueSchemaMetadata": metadata}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := event.Records[0].Value(ctx); err != sentinel {
		t.Fatal(err)
	}
}

func TestConsumerErrorHierarchy(t *testing.T) {
	cause := errors.New("original failure")
	variants := []error{
		&ConsumerError{Message: "base", Cause: cause},
		&DeserializationError{ConsumerError{Message: "decode", Cause: cause}},
		&MissingSchemaError{ConsumerError{Message: "schema", Cause: cause}},
		&ParserError{ConsumerError: ConsumerError{Message: "parse", Cause: cause}, Issues: []any{}},
	}
	for _, variant := range variants {
		var base *ConsumerError
		if !errors.As(variant, &base) || base.Message != variant.Error() || !errors.Is(variant, cause) {
			t.Fatalf("lost base or cause: %T", variant)
		}
	}
	var base *ConsumerError
	if errors.As(&TypeError{Message: "invalid"}, &base) {
		t.Fatal("native type errors are not consumer errors")
	}
}
