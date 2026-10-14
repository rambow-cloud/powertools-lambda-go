package bedrock

import (
	"context"
	json "encoding/json/v2"
	"strings"
	"testing"
)

func TestNativeInvalidStringsFailBeforeBodyPublication(t *testing.T) {
	invalid := string([]byte{0xff})
	badKey, badValue := &Parameters{}, &Parameters{}
	badKey.Set(invalid, "value")
	badValue.Set("key", invalid)
	for _, test := range []struct {
		name  string
		value any
	}{
		{"string", invalid},
		{"map key", map[string]any{invalid: "value"}},
		{"map value", map[string]any{"key": invalid}},
		{"array value", []any{invalid}},
		{"parameter key", badKey},
		{"parameter value", badValue},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := stringify(test.value)
			if err == nil || encoded != nil {
				t.Errorf("invalid native string produced partial JSON: %q, %v", encoded, err)
			}
			ctx := context.WithValue(context.Background(), struct{}{}, "request")
			var diagnosed error
			app := New(Options{Diagnostic: func(got context.Context, level, _ string, err error) {
				if level == "error" {
					diagnosed = err
					if got != ctx {
						t.Error("diagnostic lost invocation context")
					}
				}
			}})
			app.Tool(func(context.Context, *Parameters, Event) (any, error) { return test.value, nil }, Configuration{Name: "tool"})
			response, err := app.Resolve(ctx, testEvent())
			if err != nil || diagnosed == nil {
				t.Fatalf("invalid string escaped the serialization-error boundary: response=%v, err=%v, diagnostic=%v", response, err, diagnosed)
			}
			if !strings.HasPrefix(body(response).(string), "Unable to complete tool execution due to ") {
				t.Fatalf("partial body published: %v", body(response))
			}
			if _, err := json.Marshal(response); err != nil {
				t.Fatalf("failure envelope is not valid JSON: %v", err)
			}
		})
	}
}

func TestNativeBodiesRetainJSONV2Contracts(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		want  string
	}{
		{"unicode", map[string]any{"é": "😀\u2028<"}, `{"é":"😀` + "\u2028" + `<"}`},
		{"binary", []byte{1, 2}, `"AQI="`},
		{"struct", struct {
			Value int `json:"value"`
		}{3}, `{"value":3}`},
		{"nested nil containers", []any{map[string]int(nil), []int(nil)}, `[null,null]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := stringify(test.value)
			if err != nil || string(encoded) != test.want {
				t.Fatalf("native body=%s, %v; want=%s", encoded, err, test.want)
			}
		})
	}
}
