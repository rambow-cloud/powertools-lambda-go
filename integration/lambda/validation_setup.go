package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

var validationSetupSchemas = sync.OnceValues(func() (map[string]*validation.Schema, error) {
	return compileValidationSchemas(map[string]string{
		"unused":            `{"definitions":{"unknown":{"typo":true},"pattern":{"pattern":"(?i)a"}},"type":"string"}`,
		"defs":              `{"$defs":{"value":{"minLength":-1}},"$ref":"#/$defs/value"}`,
		"pattern":           `{"patternProperties":{"[":true}}`,
		"shape_scalar":      `{"$defs":{"value":1},"$ref":"#/$defs/value"}`,
		"shape_ignored":     `{"if":{"typo":true},"then":true,"else":{"deprecated":true}}`,
		"shape_unused":      `{"$defs":{"value":{"required":true}},"type":"string"}`,
		"shape_annotations": `{"deprecated":true,"contentSchema":{"type":"integer"}}`,
	})
})

func validationSetupProbe(ctx context.Context) (map[string]any, error) {
	schemas, err := validationSetupSchemas()
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	for name, payload := range map[string]any{
		"unused": "value", "defs": "value", "pattern": map[string]any{"a": 1},
		"shape_scalar": "value", "shape_ignored": "value", "shape_unused": "value", "shape_annotations": "value",
	} {
		value, err := schemas[name].Validate(ctx, payload)
		if err != nil {
			return nil, err
		}
		result[name] = value
	}
	for name, raw := range map[string]string{
		"referenced_rejected": `{"definitions":{"value":{"typo":true}},"$ref":"#/definitions/value"}`,
		"structure_rejected":  `{"definitions":{"value":{"minLength":-1}}}`,
		"pattern_rejected":    `{"patternProperties":{"[":true},"additionalProperties":false}`,
		"shape_rejected":      `{"$defs":{"value":{"required":true}},"$ref":"#/$defs/value"}`,
		"branch_rejected":     `{"if":true,"then":true,"else":{"format":"unknown"}}`,
	} {
		_, err := validation.Compile(ctx, json.RawMessage(raw), validation.Options{})
		var compilation *validation.SchemaCompilationError
		result[name] = errors.As(err, &compilation)
	}
	return result, nil
}
