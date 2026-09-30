package validation

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestConcurrentReferencePresentation(t *testing.T) {
	schema := mustCompile(t, json.RawMessage(`{"$id":"https://example.test/root","$defs":{"value":{"$id":"value","type":"string","minLength":4}},"properties":{"zebra":{"$ref":"value"},"alpha":{"$ref":"https://example.test/value"}}}`), Options{})
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Go(func() {
			_, err := schema.Validate(context.Background(), map[string]any{"alpha": false, "zebra": false})
			var failure *SchemaValidationError
			if !errors.As(err, &failure) {
				t.Errorf("expected reference diagnostics: %v", err)
				return
			}
			var paths, instances []string
			for _, issue := range failure.Issues {
				paths = append(paths, issue.SchemaPath)
				instances = append(instances, issue.InstancePath)
			}
			if !reflect.DeepEqual(paths, []string{"value/type", "https://example.test/value/type"}) || !reflect.DeepEqual(instances, []string{"/zebra", "/alpha"}) {
				t.Errorf("reference scopes: %v %v", paths, instances)
			}
		})
	}
	workers.Wait()
}

func TestNullableTypeArraySnapshot(t *testing.T) {
	types := []any{"string"}
	schema := mustCompile(t, map[string]any{"type": types, "nullable": true}, Options{})
	if !reflect.DeepEqual(types, []any{"string"}) {
		t.Fatalf("compiler mutated caller type declaration: %v", types)
	}
	types[0] = "boolean"
	if _, err := schema.Validate(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	_, err := schema.Validate(context.Background(), false)
	var failure *SchemaValidationError
	if !errors.As(err, &failure) || len(failure.Issues) != 1 {
		t.Fatalf("nullable validation: %v", err)
	}
	if !reflect.DeepEqual(failure.Issues[0].Params["type"], []any{"string", "null"}) || failure.Issues[0].Message != "must be string,null" {
		t.Fatalf("nullable diagnostics: %#v", failure.Issues)
	}
}
