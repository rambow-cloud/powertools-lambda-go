package bedrock

import (
	"context"
	"strings"
	"testing"
)

func TestToolResponseAncestorIdentity(t *testing.T) {
	cyclicMap := map[string]any{"value": "kept"}
	cyclicMap["self"] = cyclicMap
	cyclicSlice := []any{"kept", nil}
	cyclicSlice[1] = cyclicSlice
	cyclicParameters := &Parameters{}
	cyclicParameters.Set("self", cyclicParameters)
	sharedMap := map[string]any{"value": "kept"}
	sharedSlice := []any{"kept"}
	sharedParameters := &Parameters{}
	sharedParameters.Set("value", "kept")
	nilParameters := &Parameters{}
	nilParameters.Set("map", map[string]any(nil))
	nilParameters.Set("slice", []any(nil))
	nilParameters.Set("pointer", (*Parameters)(nil))
	for _, test := range []struct {
		name  string
		value any
		want  string
		cycle bool
	}{
		{"map cycle", cyclicMap, "", true},
		{"slice cycle", cyclicSlice, "", true},
		{"parameter pointer cycle", cyclicParameters, "", true},
		{"shared maps", []any{sharedMap, sharedMap}, `[{"value":"kept"},{"value":"kept"}]`, false},
		{"shared slices", []any{sharedSlice, sharedSlice}, `[["kept"],["kept"]]`, false},
		{"shared parameter pointers", []any{sharedParameters, sharedParameters}, `[{"value":"kept"},{"value":"kept"}]`, false},
		{"overlapping slices", []any{sharedSlice, sharedSlice[:0]}, `[["kept"],[]]`, false},
		{"nested nil containers", nilParameters, `{"map":null,"slice":null,"pointer":null}`, false},
		// Nil tool results retain the resolver's empty-response contract.
		{"nil map", map[string]any(nil), "", false},
		{"nil slice", []any(nil), "", false},
		{"nil pointer", (*Parameters)(nil), "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := New(quiet())
			app.Tool(func(context.Context, *Parameters, Event) (any, error) {
				return test.value, nil
			}, Configuration{Name: "tool"})
			response, err := app.Resolve(context.Background(), testEvent())
			if err != nil {
				t.Fatal(err)
			}
			got := body(response).(string)
			if test.cycle {
				if !strings.Contains(got, "TypeError - Converting circular structure to JSON") {
					t.Fatalf("cycle response = %q", got)
				}
			} else if got != test.want {
				t.Fatalf("response = %q, want %q", got, test.want)
			}
		})
	}
	if sharedMap["value"] != "kept" || sharedSlice[0] != "kept" || sharedParameters.Get("value") != "kept" {
		t.Fatal("response encoding mutated the caller's containers")
	}
}
