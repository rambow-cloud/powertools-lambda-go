package jmespath

import (
	"errors"
	"testing"
)

func TestLiteralResultIsolationAndStandardGrammar(t *testing.T) {
	expression := MustCompile("`{\"items\":[1,2]}`")
	first, err := expression.Search(nil)
	if err != nil {
		t.Fatal(err)
	}
	first.(map[string]any)["items"].([]any)[0] = float64(99)
	second, err := expression.Search(nil)
	if err != nil || second.(map[string]any)["items"].([]any)[0] != float64(1) {
		t.Fatal("cached literal mutated", second, err)
	}
	for _, source := range []string{"`1` + `2`", "let $x = `1` in $x", "$"} {
		_, err := Compile(source)
		var failure *Error
		if !errors.As(err, &failure) || failure.Kind != SyntaxError {
			t.Fatal(source, err)
		}
	}
}
