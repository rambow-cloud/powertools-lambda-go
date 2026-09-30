package avro

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/kafka"
)

func normalize(value any) any {
	switch v := value.(type) {
	case []byte:
		items := make([]any, len(v))
		for i, b := range v {
			items[i] = float64(b)
		}
		return map[string]any{"$": "bytes", "data": items}
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || (v == 0 && math.Signbit(v)) {
			tag := "NaN"
			if math.IsInf(v, 1) {
				tag = "Infinity"
			}
			if math.IsInf(v, -1) {
				tag = "-Infinity"
			}
			if v == 0 {
				tag = "-0"
			}
			return map[string]any{"$": tag}
		}
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalize(item)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = normalize(item)
		}
		return out
	}
	return value
}

func TestTypeScriptReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Version string
		Cases   []struct {
			Name, Schema, Data string
			Metadata, Value    any
			Error              *struct{ Name, Message string }
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != "2.35.0" || len(corpus.Cases) < 370 {
		t.Fatal("unexpected fixture corpus")
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			consumer := kafka.New(kafka.Config{Value: New(tc.Schema)})
			event, err := consumer.Deserialize(context.Background(), map[string]any{"records": map[string]any{"topic": []any{map[string]any{"value": tc.Data, "headers": []any{}, "valueSchemaMetadata": tc.Metadata}}}})
			if err != nil {
				t.Fatal("eager schema failure", err)
			}
			value, err := event.Records[0].Value(context.Background())
			if tc.Error != nil {
				var failure *kafka.DeserializationError
				if !errors.As(err, &failure) || failure.ErrorName() != tc.Error.Name || failure.Error() != tc.Error.Message {
					t.Fatalf("actual: %v\nexpected: %s: %s", err, tc.Error.Name, tc.Error.Message)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			actual := normalize(value)
			if !reflect.DeepEqual(actual, tc.Value) {
				got, _ := json.Marshal(actual)
				want, _ := json.Marshal(tc.Value)
				t.Fatalf("actual: %s\nexpected: %s", got, want)
			}
		})
	}
	t.Logf("Verified %d actual Avro scenarios", len(corpus.Cases))
}

func TestIsolationCancellationAndLazySchema(t *testing.T) {
	c := kafka.New(kafka.Config{Value: New(`{"type":"record","name":"Value","fields":[{"name":"items","type":{"type":"array","items":"int"}}]}`)})
	ctx := context.Background()
	event, err := c.Deserialize(ctx, map[string]any{"records": map[string]any{"x": []any{map[string]any{"value": "BAIEAA=="}}}})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			value, err := event.Records[0].Value(ctx)
			if err != nil {
				t.Error(err)
				return
			}
			items := value.(map[string]any)["items"].([]any)
			if !reflect.DeepEqual(items, []any{float64(1), float64(2)}) {
				t.Error(items)
			}
			items[0] = "mutated"
		})
	}
	wg.Wait()
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := event.Records[0].Value(canceled); err != context.Canceled {
		t.Fatal(err)
	}
	invalid := kafka.New(kafka.Config{Value: New(`{"type":"not-defined"}`)})
	event, err = invalid.Deserialize(ctx, map[string]any{"records": map[string]any{"x": []any{map[string]any{"value": nil}, map[string]any{"value": "AA=="}}}})
	if err != nil {
		t.Fatal(err)
	}
	if value, err := event.Records[0].Value(ctx); value != nil || err != nil {
		t.Fatal(value, err)
	}
	var failure *kafka.DeserializationError
	if _, err = event.Records[1].Value(ctx); !errors.As(err, &failure) {
		t.Fatal(err)
	}
}
