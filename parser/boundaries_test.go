package parser_test

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

func TestEnvelopeBoundaryReference(t *testing.T) {
	runReference(t, "testdata/boundaries-v2.35.0.json", 84)
}

func TestNativeTypedBoundaryMapping(t *testing.T) {
	type payload struct {
		Name     string   `json:"name"`
		Optional *string  `json:"optional,omitempty"`
		Nullable *string  `json:"nullable"`
		Count    float64  `json:"count"`
		Items    []string `json:"items"`
	}
	schema := parser.Typed[payload](parser.Object(
		parser.Field{Name: "name", Schema: parser.String()},
		parser.Field{Name: "optional", Schema: parser.String(), Optional: true},
		parser.Field{Name: "nullable", Schema: parser.Nullable(parser.String())},
		parser.Field{Name: "count", Schema: parser.Number()},
		parser.Field{Name: "items", Schema: parser.Any(parser.Array(parser.String()))},
	))
	input := map[string]any{"name": "你好😀", "nullable": nil, "count": int64(9007199254740993), "items": []any{"a"}, "extra": true}
	got, err := parser.Parse(context.Background(), input, schema)
	if err != nil || got.Name != "你好😀" || got.Optional != nil || got.Nullable != nil || got.Count != 9007199254740992 || !reflect.DeepEqual(got.Items, []string{"a"}) {
		t.Fatalf("typed mapping %+v/%v", got, err)
	}
	got.Items[0] = "changed"
	if input["items"].([]any)[0] != "a" {
		t.Fatal("typed result aliases input")
	}
	for _, value := range []any{math.NaN(), math.Inf(1), math.Inf(-1)} {
		result, err := parser.SafeParse(context.Background(), value, parser.Number())
		if err != nil || result.Success || result.Error == nil || result.Error.Issues[0].Code != "invalid_type" {
			t.Fatalf("non-finite number %+v/%v", result, err)
		}
	}
	negative, err := parser.Parse(context.Background(), math.Copysign(0, -1), parser.Number())
	if err != nil || !math.Signbit(negative.(float64)) {
		t.Fatalf("negative zero %v/%v", negative, err)
	}
}

func TestBoundaryJSONAndOperationalErrors(t *testing.T) {
	schema := parser.JSONStringified(parser.Unknown())
	for _, input := range []string{`{"x":1,"x":2}`, "\"\xff\"", `"\ud800"`, `{} {}`} {
		result, err := parser.SafeParse(context.Background(), input, schema)
		if err != nil || result.Success || result.Error == nil || result.Error.Issues[0].Code != "custom" {
			t.Fatalf("strict JSON %q: %+v/%v", input, result, err)
		}
	}
	cause := errors.New("application transform failure")
	calls := 0
	typed := parser.Transform(parser.String(), func(context.Context, any) (any, error) { calls++; return nil, cause })
	if _, err := parser.SafeParse(context.Background(), "value", typed); !errors.Is(err, cause) || calls != 1 {
		t.Fatalf("cause/count %v/%d", err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := parser.Parse(ctx, "value", typed); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("pre-cancel %v/%d", err, calls)
	}
	encoded, err := json.Marshal(map[string]any{"name": "😀"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := parser.Parse(context.Background(), jsontext.Value(encoded), parser.Unknown())
	if err != nil || !reflect.DeepEqual(jsonValue(value), map[string]any{"name": "😀"}) {
		t.Fatalf("raw JSON interop %v/%v", value, err)
	}
}

func TestBoundaryPrefixOwnsRecursiveErrors(t *testing.T) {
	original := []parser.Issue{{Code: "invalid_union", Path: []any{"payload"}, Errors: [][]parser.Issue{{{Code: "invalid_type", Expected: "string", Path: []any{"value"}}}}}}
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Go(func() {
			copy := parser.Prefix(original, "root")
			copy[0].Path[0] = "changed"
			copy[0].Errors[0][0].Path[0] = "changed"
			copy[0].Errors[0][0].Expected = "changed"
		})
	}
	group.Wait()
	if !reflect.DeepEqual(original[0].Path, []any{"payload"}) || original[0].Errors[0][0].Path[0] != "value" || original[0].Errors[0][0].Expected != "string" {
		t.Fatal("recursive errors shared across callers")
	}
}
