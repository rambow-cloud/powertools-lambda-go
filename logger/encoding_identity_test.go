package logger

import (
	"bytes"
	"reflect"
	"testing"
)

func TestLogAncestorIdentity(t *testing.T) {
	cleanEnv(t)
	type namedMap map[string]any
	named := namedMap{"value": "kept"}
	named["self"] = named
	items := []any{"kept", nil}
	items[1] = items
	var pointer any
	pointer = &pointer
	shared := namedMap{"value": "kept"}
	for _, test := range []struct {
		name  string
		value any
		want  any
	}{
		{"named map cycle", named, map[string]any{"value": "kept", "self": "[Circular]"}},
		{"slice cycle", items, []any{"kept", "[Circular]"}},
		{"pointer cycle", pointer, "[Circular]"},
		{"shared maps", []any{shared, shared, namedMap{"value": "kept"}}, []any{map[string]any{"value": "kept"}, map[string]any{"value": "kept"}, map[string]any{"value": "kept"}}},
		{"nil map", namedMap(nil), nil},
		{"nil slice", []any(nil), nil},
		{"nil pointer", (*namedMap)(nil), nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			log := New(WithOutput(&output))
			if err := log.Info("identity", Fields{"payload": test.value}); err != nil {
				t.Fatal(err)
			}
			got := records(t, &output)[0]["payload"]
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("payload = %#v, want %#v", got, test.want)
			}
		})
	}
	if named["value"] != "kept" || reflect.ValueOf(named["self"]).Pointer() != reflect.ValueOf(named).Pointer() || items[0] != "kept" || shared["value"] != "kept" {
		t.Fatal("logging mutated the caller's containers")
	}
}
