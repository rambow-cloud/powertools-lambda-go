package validationdialects_test

import (
	"context"
	"encoding/json/jsontext"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

func TestDraft202012ExactNumberBoundaries(t *testing.T) {
	testDialectNumbers(t, validation.Draft202012Compiler{})
}

func TestDraft201909ExactNumberBoundaries(t *testing.T) {
	testDialectNumbers(t, validation.Draft201909Compiler{})
}

func testDialectNumbers(t *testing.T, compiler validation.Compiler) {
	t.Helper()
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
			schema, err := validation.Compile(context.Background(), jsontext.Value(item.schema), validation.Options{Compiler: compiler})
			if err != nil {
				t.Fatal(err)
			}
			_, err = schema.Validate(context.Background(), jsontext.Value(item.input))
			if item.valid != (err == nil) {
				t.Fatal(item.input, err)
			}
		})
	}
}
