package datamasking

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestEraseMissingRulePolicy(t *testing.T) {
	for _, ignore := range []bool{false, true} {
		for _, mixed := range []bool{false, true} {
			for _, field := range []string{"missing", "items[*].missing"} {
				t.Run(fmt.Sprintf("ignore=%t/mixed=%t/%s", ignore, mixed, field), func(t *testing.T) {
					input := map[string]any{"secret": "visible", "items": []any{map[string]any{"present": "value"}}}
					warnings := []string{}
					m := New(Config{IgnoreMissing: ignore, Warn: func(_ context.Context, s string) { warnings = append(warnings, s) }})
					o := EraseOptions{Rules: []FieldRule{{Field: field}}}
					if mixed {
						o.Fields = []string{"secret"}
					}
					v, e := m.Erase(context.Background(), input, o)
					message := "Field not found: '" + field + "'"
					if ignore {
						want := map[string]any{"secret": "visible", "items": []any{map[string]any{"present": "value"}}}
						if mixed {
							want["secret"] = DefaultMask
						}
						if e != nil || !reflect.DeepEqual(v, want) || !reflect.DeepEqual(warnings, []string{message}) {
							t.Fatalf("ignored missing rule: %v/%v warnings=%v", v, e, warnings)
						}
					} else {
						var missing *Error
						if !errors.As(e, &missing) || missing.Name != "DataMaskingFieldNotFoundError" || missing.Message != message || v != nil || len(warnings) != 0 {
							t.Fatalf("strict missing rule: %v/%v warnings=%v", v, e, warnings)
						}
					}
					if input["secret"] != "visible" {
						t.Fatal("caller input changed")
					}
				})
			}
		}
	}
}

func TestEraseRuleOrderOverlapAndMissingWarnings(t *testing.T) {
	warnings := []string{}
	first := "one"
	m := New(Config{IgnoreMissing: true, Warn: func(_ context.Context, s string) { warnings = append(warnings, s) }})
	in := map[string]any{"secret": "original", "other": "value"}
	v, e := m.Erase(context.Background(), in, EraseOptions{Rules: []FieldRule{
		{Field: "secret", Rule: Rule{CustomMask: &first}},
		{Field: "ruleMissing"},
		{Field: "secret", Rule: Rule{Replace: func(s string) (string, error) { return s + "-two", nil }}},
	}, Fields: []string{"secret", "other", "fieldMissing"}})
	if e != nil || !reflect.DeepEqual(v, map[string]any{"secret": "one-two", "other": DefaultMask}) || !reflect.DeepEqual(warnings, []string{"Field not found: 'ruleMissing'", "Field not found: 'fieldMissing'"}) {
		t.Fatalf("ordering: %v/%v warnings=%v", v, e, warnings)
	}
	if !reflect.DeepEqual(in, map[string]any{"secret": "original", "other": "value"}) {
		t.Fatal("input mutated")
	}
}

func TestEraseMissingRuleAfterMaskPreservesInput(t *testing.T) {
	in := map[string]any{"secret": "visible"}
	v, e := New(Config{}).Erase(context.Background(), in, EraseOptions{Rules: []FieldRule{{Field: "secret"}, {Field: "missing"}}})
	var missing *Error
	if v != nil || !errors.As(e, &missing) || missing.Name != "DataMaskingFieldNotFoundError" || !reflect.DeepEqual(in, map[string]any{"secret": "visible"}) {
		t.Fatalf("partial failure changed input: result=%v error=%v input=%v", v, e, in)
	}
}
