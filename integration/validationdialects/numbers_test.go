package validationdialects_test

import (
	"context"
	jsonv1 "encoding/json"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

func TestDraft202012ExactNumberBoundaries(t *testing.T) {
	for _, item := range []struct {
		name, schema, input string
		valid               bool
	}{
		{"large-lower-bound", `{"type":"integer","minimum":9007199254740993}`, `9007199254740992`, false},
		{"large-equal-bound", `{"type":"integer","minimum":9007199254740993}`, `9007199254740993`, true},
		{"large-enum", `{"enum":[9007199254740993]}`, `9007199254740993`, true},
		{"large-enum-mismatch", `{"enum":[9007199254740993]}`, `9007199254740992`, false},
		{"integer-exponent", `{"type":"integer","multipleOf":1000}`, `1e3`, true},
		{"negative-zero", `{"type":"integer","const":0}`, `-0`, true},
		{"exact-fraction", `{"type":"number","multipleOf":0.1}`, `1.2`, true},
	} {
		t.Run(item.name, func(t *testing.T) {
			schema, err := validation.Compile(context.Background(), jsonv1.RawMessage(item.schema), validation.Options{Compiler: validation.Draft202012Compiler{}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = schema.Validate(context.Background(), jsonv1.RawMessage(item.input))
			if item.valid != (err == nil) {
				t.Fatal(item.input, err)
			}
		})
	}
}
