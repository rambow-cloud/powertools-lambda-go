package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStrictOverlapResourceFailure(t *testing.T) {
	const pattern = "^(a|aa)+$"
	_, err := Compile(context.Background(), map[string]any{
		"properties":        map[string]any{strings.Repeat("a", 16) + "!": true},
		"patternProperties": map[string]any{pattern: map[string]any{"type": "integer"}},
	}, Options{CompileOptions: CompileOptions{Regex: RegexOptions{MaxBacktrackingStackSize: 1}}})
	var compilation *SchemaCompilationError
	var operational *RegexError
	if !errors.As(err, &compilation) || !errors.As(err, &operational) || operational.Pattern != pattern {
		t.Fatalf("strict matching must retain resource failure: %v", err)
	}
}
