package commons_test

import (
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

func TestDeepMergeAncestorIdentity(t *testing.T) {
	type namedMap map[string]any
	named := namedMap{"value": 1}
	named["self"] = named
	items := []any{1, nil}
	items[1] = items
	mutual := map[string]any{"value": 2}
	mutual["child"] = map[string]any{"parent": mutual, "value": 3}
	for _, test := range []struct {
		name  string
		value any
		want  any
	}{
		{"named map cycle", named, map[string]any{"value": 1}},
		{"slice cycle", items, []any{1}},
		{"mutual map cycle", mutual, map[string]any{"value": 2, "child": map[string]any{"value": 3}}},
		{"nil map", namedMap(nil), map[string]any{}},
		{"nil slice", []any(nil), []any{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := commons.DeepMerge(nil, map[string]any{"payload": test.value})
			if !reflect.DeepEqual(got["payload"], test.want) {
				t.Fatalf("payload = %#v, want %#v", got["payload"], test.want)
			}
		})
	}

	shared := map[string]any{"value": 4}
	source := map[string]any{"first": shared, "second": shared, "equal": map[string]any{"value": 4}}
	got := commons.DeepMerge(nil, source)
	if !reflect.DeepEqual(got, source) {
		t.Fatal("shared or equal descendants were treated as ancestors")
	}
	got["first"].(map[string]any)["value"] = 9
	if shared["value"] != 4 || got["second"].(map[string]any)["value"] != 4 {
		t.Fatal("merged descendants alias the source or their siblings")
	}
	if got := commons.DeepMerge(source, source); !reflect.DeepEqual(got, source) {
		t.Fatal("merging the target into itself changed its contents")
	}
}
