package validation

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func TestKeywordConstraintIsolation(t *testing.T) {
	schema, err := Compile(context.Background(), json.RawMessage(`{"$defs":{"value":{"minProperties":1.5,"maxProperties":-1,"required":[{"a":1}]}},"$ref":"#/$defs/value"}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 32 {
		workers.Go(func() {
			for range 2 {
				_, err := schema.Validate(context.Background(), map[string]any{})
				var failure *SchemaValidationError
				if !errors.As(err, &failure) || len(failure.Issues) != 3 {
					t.Errorf("unexpected validation: %v", err)
					return
				}
				for i, name := range []string{"maxProperties", "minProperties", "required"} {
					if failure.Issues[i].Keyword != name {
						t.Errorf("issue %d: %#v", i, failure.Issues[i])
					}
				}
				value := failure.Issues[2].Params["missingProperty"].(map[string]any)
				if value["a"] != json.Number("1") {
					t.Errorf("shared diagnostic value: %#v", value)
				}
				value["a"] = "changed by caller"
			}
		})
	}
	workers.Wait()
}
