package validation

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"reflect"
	"testing"
)

func TestOneOfPassingSchemasJSONValue(t *testing.T) {
	for _, test := range []struct {
		name     string
		branches []any
		want     any
	}{
		{"none", []any{map[string]any{"type": "string"}, map[string]any{"type": "number"}}, nil},
		{"multiple", []any{true, true}, []any{float64(0), float64(1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Validate(context.Background(), false, map[string]any{"oneOf": test.branches}, Options{})
			var failure *SchemaValidationError
			if !errors.As(err, &failure) {
				t.Fatal(err)
			}
			for _, issue := range failure.Issues {
				if issue.Keyword != "oneOf" {
					continue
				}
				data, err := json.Marshal(issue.Params)
				if err != nil {
					t.Fatal(err)
				}
				var params map[string]any
				if err := json.Unmarshal(data, &params); err != nil {
					t.Fatal(err)
				}
				got, present := params["passingSchemas"]
				if !present || !reflect.DeepEqual(got, test.want) {
					t.Fatalf("passingSchemas = %#v, want %#v", got, test.want)
				}
				return
			}
			t.Fatal("missing oneOf diagnostic")
		})
	}
}

func TestEmptyPropertyNameRemainsInJSONDiagnostic(t *testing.T) {
	_, err := Validate(context.Background(), map[string]any{"": true}, map[string]any{
		"propertyNames": map[string]any{"pattern": "^[a-z]+$"},
	}, Options{})
	var failure *SchemaValidationError
	if !errors.As(err, &failure) {
		t.Fatal(err)
	}
	for _, issue := range failure.Issues {
		if issue.Keyword != "pattern" {
			continue
		}
		data, err := json.Marshal(issue)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		if name, present := value["propertyName"]; !present || name != "" {
			t.Fatalf("empty property name missing: %s", data)
		}
		return
	}
	t.Fatal("missing propertyNames pattern diagnostic")
}
