// Package testfixture shares dependency-free conformance helpers across module tests.
package testfixture

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"reflect"
	"testing"
)

// Load reads the pinned JSON fixture owned by the calling package.
func Load(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

// Normalize maps native values to the shared cross-language fixture representation.
func Normalize(value any) any {
	switch v := value.(type) {
	case *big.Int:
		return map[string]any{"bigint": v.String()}
	case float64:
		if math.IsNaN(v) {
			return map[string]any{"number": "NaN"}
		}
		if math.IsInf(v, 1) {
			return map[string]any{"number": "Infinity"}
		}
		if math.IsInf(v, -1) {
			return map[string]any{"number": "-Infinity"}
		}
		return v
	case []byte:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = float64(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = Normalize(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = Normalize(item)
		}
		return out
	default:
		return v
	}
}

// AssertOutcome compares a reference result or expected failure.
func AssertOutcome(t *testing.T, test map[string]any, value any, err error) {
	t.Helper()
	if test["error"] != nil {
		if err == nil {
			t.Fatalf("expected error: %#v", test)
		}
		return
	}
	if err != nil || !reflect.DeepEqual(Normalize(value), test["value"]) {
		t.Fatalf("case=%#v value=%#v error=%v", test, Normalize(value), err)
	}
}
