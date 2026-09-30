package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

const validationRegexPattern = `^(?<word>\p{Script=Greek}+)\k<word>$`

var validationRegexSchemas = sync.OnceValues(func() (map[string]*validation.Schema, error) {
	result := map[string]*validation.Schema{}
	for _, item := range []struct {
		name    string
		schema  any
		options validation.Options
	}{
		{"backreference", map[string]any{"pattern": validationRegexPattern}, validation.Options{}},
		{"properties", map[string]any{"type": "object", "patternProperties": map[string]any{`^\p{Letter}+$`: map[string]any{"type": "integer"}}}, validation.Options{}},
		{"limited", map[string]any{"pattern": "^(a|aa)+$"}, validation.Options{CompileOptions: validation.CompileOptions{Regex: validation.RegexOptions{MaxBacktrackingStackSize: 1}}}},
	} {
		schema, err := validation.Compile(context.Background(), item.schema, item.options)
		if err != nil {
			return nil, err
		}
		result[item.name] = schema
	}
	return result, nil
})

func validationRegexProbe(ctx context.Context) (map[string]any, error) {
	schemas, err := validationRegexSchemas()
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	result["matched"], err = schemas["backreference"].Validate(ctx, "αβαβ")
	if err != nil {
		return nil, err
	}
	_, err = schemas["backreference"].Validate(ctx, "αβ")
	var failure *validation.SchemaValidationError
	if !errors.As(err, &failure) || len(failure.Issues) != 1 {
		return nil, fmt.Errorf("missing regex validation error: %v", err)
	}
	result["pattern"] = failure.Issues[0].Params["pattern"]
	_, err = schemas["properties"].Validate(ctx, map[string]any{"α": "bad"})
	if !errors.As(err, &failure) || len(failure.Issues) != 1 {
		return nil, fmt.Errorf("missing property validation error: %v", err)
	}
	result["property_issue"] = failure.Issues[0]
	_, err = validation.Compile(ctx, map[string]any{"pattern": "(?i)a"}, validation.Options{})
	var compilation *validation.SchemaCompilationError
	result["invalid_syntax"] = errors.As(err, &compilation)
	_, err = schemas["limited"].Validate(ctx, strings.Repeat("a", 16)+"!")
	var operational *validation.RegexError
	result["resource_error"] = errors.As(err, &operational) && !errors.As(err, &failure)
	return result, nil
}
