package validation

import (
	"context"
	jsonv1 "encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestDraft201909Reference(t *testing.T) {
	testNativeDraftReference(t, "testdata/draft-2019-09-v2.35.0.json", Draft201909Compiler{}, 58, map[string]bool{"contains-evaluated-valid": true, "contains-evaluated-invalid": true, "fractional-multiple-native-exact": true})
}

func TestDraft201909TupleSnapshotsAndIsolation(t *testing.T) {
	child := map[string]any{"$id": "tuple", "type": "array", "items": []any{map[string]any{"type": "integer"}}, "additionalItems": false}
	library := map[string]any{"$id": "https://example.test/library/", "$defs": map[string]any{"value": child}}
	compiled, err := Compile(context.Background(), map[string]any{"$ref": "https://example.test/library/tuple"}, Options{Compiler: Draft201909Compiler{}, CompileOptions: CompileOptions{ExternalSchemas: []any{library}}})
	if err != nil {
		t.Fatal(errors.Unwrap(err))
	}
	if child["$id"] != "tuple" {
		t.Fatal("caller resource identity changed")
	}
	child["additionalItems"] = true
	library["$defs"] = nil
	payload := []any{1, "extra"}
	_, err = compiled.Validate(context.Background(), payload)
	var first *SchemaValidationError
	if !errors.As(err, &first) || len(first.Issues) != 1 {
		t.Fatal(err)
	}
	first.Issues[0].Params["absoluteKeywordLocation"] = "changed"
	var wg sync.WaitGroup
	for index := range 32 {
		wg.Go(func() {
			if index%2 == 0 {
				if _, err := compiled.Validate(context.Background(), []any{1}); err != nil {
					t.Error(err)
				}
				return
			}
			_, err := compiled.Validate(context.Background(), payload)
			var failure *SchemaValidationError
			if !errors.As(err, &failure) || len(failure.Issues) != 1 || failure.Issues[0].Keyword != "additionalItems" || failure.Issues[0].Params["absoluteKeywordLocation"] == "changed" {
				t.Errorf("tuple/diagnostic reuse changed: %v", err)
			}
		})
	}
	wg.Wait()
	if !reflect.DeepEqual(payload, []any{1, "extra"}) {
		t.Fatal("payload changed", payload)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compiled.Validate(ctx, []any{1}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, source := range []any{map[string]any{"$schema": draft202012URI}, map[string]any{"prefixItems": []any{true}}, map[string]any{"$dynamicRef": "#node"}, map[string]any{"$ref": "https://example.test/not-registered"}} {
		if _, err := Compile(context.Background(), source, Options{Compiler: Draft201909Compiler{}}); err == nil {
			t.Fatal("wrong dialect/vocabulary or implicit loader accepted")
		}
	}
	if _, err := Compile(context.Background(), map[string]any{"$schema": draft201909URI}, Options{}); err == nil {
		t.Fatal("default Draft 7 selected 2019-09")
	}
}

func TestDraft201909NestedTupleVocabulary(t *testing.T) {
	for _, source := range []jsonv1.RawMessage{
		jsonv1.RawMessage(`{"items":[{"typo":true}]}`),
		jsonv1.RawMessage(`{"items":[{"$schema":"https://json-schema.org/draft/2020-12/schema"}]}`),
		jsonv1.RawMessage(`{"items":[{"$id":"https://example.test/same"},{"$id":"https://example.test/same"}]}`),
	} {
		if _, err := Compile(context.Background(), source, Options{Compiler: Draft201909Compiler{}}); err == nil {
			t.Fatal("tuple schema position escaped checks")
		}
	}
}
