package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

type fixtureError struct{ name, message string }

func (e *fixtureError) Error() string     { return e.message }
func (e *fixtureError) ErrorName() string { return e.name }

func normalize(value any) any {
	switch v := value.(type) {
	case Undefined:
		return map[string]any{"$": "undefined"}
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
	case map[string]any:
		out := map[string]any{}
		for k, item := range v {
			out[k] = normalize(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalize(item)
		}
		return out
	}
	return value
}

func errorValue(err error) any {
	if err == nil {
		return nil
	}
	name := "Error"
	var named interface{ ErrorName() string }
	if errors.As(err, &named) {
		name = named.ErrorName()
	}
	out := map[string]any{"name": name, "message": err.Error()}
	var parser *ParserError
	if errors.As(err, &parser) {
		out["cause"] = parser.Issues
	}
	return out
}

func TestTypeScriptReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Version string
		Cases   []struct {
			Name, Event string
			Config      map[string]struct {
				Type   SchemaType
				Schema any
				Parser string
			}
			Access   []string
			Expected any
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != "2.35.0" || len(corpus.Cases) < 165 {
		t.Fatal("unexpected reference corpus")
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			logs, calls, records := []any{}, []any{}, []any{}
			config := Config{Diagnostic: func(_ context.Context, message string, _ error) { logs = append(logs, message) }}
			for field, input := range tc.Config {
				item := &FieldConfig{Type: input.Type, Schema: input.Schema}
				// These cases exercise missing-schema timing, not binary codec behavior.
				if input.Type == Avro || input.Type == Protobuf {
					item.Decoder = func(context.Context, string, any, any) (any, error) {
						t.Fatal("missing schema invoked decoder")
						return nil, nil
					}
				}
				if input.Parser != "" {
					item.Parser = func(_ context.Context, value any) (ParseResult, error) {
						calls = append(calls, map[string]any{"field": field, "value": normalize(value)})
						switch input.Parser {
						case "fail":
							return ParseResult{Issues: []any{map[string]any{"message": "invalid field"}}}, nil
						case "emptyIssues":
							return ParseResult{Issues: []any{}}, nil
						case "throw":
							return ParseResult{}, &fixtureError{"RangeError", "parser failed"}
						case "transform":
							return ParseResult{Value: map[string]any{"parsed": value}}, nil
						default:
							return ParseResult{Value: value}, nil
						}
					}
				}
				if field == "key" {
					config.Key = item
				} else {
					config.Value = item
				}
			}
			output, err := New(config).Deserialize(context.Background(), json.RawMessage(tc.Event))
			if tc.Name == "topic-order" {
				// The pinned JavaScript fixture repeats a topic member. JSON v2
				// rejects it instead of applying the reference's last-value rule.
				if err == nil || !strings.Contains(err.Error(), "duplicate object member name") || output != nil || len(calls) != 0 {
					t.Fatalf("duplicate topic was accepted: %v / %v", output, err)
				}
				return
			}
			var fields any
			if err == nil {
				fields = output.Fields
				for _, record := range output.Records {
					metadata := map[string]any{}
					for key, value := range record.Fields {
						metadata[key] = normalize(value)
					}
					metadata["originalKey"], metadata["originalValue"], metadata["originalHeaders"] = normalize(record.OriginalKey), normalize(record.OriginalValue), normalize(record.OriginalHeaders)
					metadata["keySchemaMetadata"], metadata["valueSchemaMetadata"] = normalize(record.KeySchemaMetadata), normalize(record.ValueSchemaMetadata)
					reads := []any{}
					for _, field := range tc.Access {
						var value any
						var err error
						switch field {
						case "key":
							value, err = record.Key(context.Background())
						case "value":
							value, err = record.Value(context.Background())
						case "headers":
							value, err = record.Headers(context.Background())
						}
						if err != nil {
							value = nil
						}
						reads = append(reads, map[string]any{"field": field, "value": normalize(value), "error": errorValue(err)})
					}
					records = append(records, map[string]any{"metadata": metadata, "reads": reads})
				}
			}
			actual := map[string]any{"entered": err == nil, "fields": fields, "records": records, "calls": calls, "logs": logs, "error": errorValue(err)}
			encoded, marshalErr := json.Marshal(actual)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			var comparable any
			if err := json.Unmarshal(encoded, &comparable); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(comparable, tc.Expected) {
				want, _ := json.Marshal(tc.Expected)
				t.Fatalf("actual: %s\nexpected: %s", encoded, want)
			}
		})
	}
	t.Logf("Verified %d actual TypeScript scenarios", len(corpus.Cases))
}
