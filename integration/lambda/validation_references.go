package main

import (
	"context"
	jsonv1 "encoding/json"
	"errors"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

var validationReferenceSchemas = sync.OnceValues(func() (map[string]*validation.Schema, error) {
	schemas := map[string]string{
		"type_order":             `{"type":"object","minimum":5,"minLength":4,"minProperties":2}`,
		"nullable":               `{"type":["string"],"nullable":true}`,
		"nested_id":              `{"$id":"https://example.test/root","$defs":{"value":{"$id":"value","type":"string"}},"properties":{"zebra":{"$ref":"value"},"alpha":{"$ref":"https://example.test/value"}}}`,
		"recursive":              `{"$defs":{"node":{"type":"object","properties":{"child":{"$ref":"#/$defs/node"}},"required":["child"]}},"$ref":"#/$defs/node"}`,
		"strict_unicode":         `{"properties":{"😀":{"type":"string"}},"patternProperties":{"^.$":{"type":"integer"}}}`,
		"strict_property_escape": `{"properties":{"a":{"type":"string"}},"patternProperties":{"^\\p{Letter}+$":{"type":"integer"}}}`,
		"keyword_size":           `{"$defs":{"value":{"type":"string","minLength":1.5,"maxLength":-1}},"$ref":"#/$defs/value"}`,
		"keyword_empty":          `{"$defs":{"value":{"allOf":[],"anyOf":[],"oneOf":[]}},"$ref":"#/$defs/value"}`,
		"keyword_required":       `{"$defs":{"value":{"required":[1,{"a":1}]}},"$ref":"#/$defs/value"}`,
		"keyword_decimal":        `{"multipleOf":0.1}`,
		"keyword_zero":           `{"$defs":{"value":{"multipleOf":0}},"$ref":"#/$defs/value"}`,
		"keyword_exponent":       `{"multipleOf":1}`,
		"keyword_array":          `{"$defs":{"value":{"minItems":1.5,"maxItems":-1}},"$ref":"#/$defs/value"}`,
		"shape_dependency":       `{"$defs":{"value":{"dependencies":{"value":[true]}}},"$ref":"#/$defs/value"}`,
	}
	return compileValidationSchemas(schemas)
})

func validationReferenceProbe(ctx context.Context) (map[string]any, error) {
	schemas, err := validationReferenceSchemas()
	if err != nil {
		return nil, err
	}
	payloads := map[string]any{
		"type_order": "bad", "nullable": false,
		"nested_id":              map[string]any{"alpha": false, "zebra": false},
		"recursive":              map[string]any{"child": map[string]any{"child": false}},
		"strict_unicode":         map[string]any{"😀": "bad"},
		"strict_property_escape": map[string]any{"a": "bad"},
		"keyword_size":           "a",
		"keyword_empty":          nil,
		"keyword_required":       map[string]any{},
		"keyword_decimal":        0.3,
		"keyword_zero":           1,
		"keyword_exponent":       1e21,
		"keyword_array":          []any{1},
		"shape_dependency":       map[string]any{"value": 1},
	}
	result, err := collectValidationIssues(ctx, schemas, payloads)
	if err != nil {
		return nil, err
	}
	_, err = validation.Compile(ctx, jsonv1.RawMessage(`{"properties":{"😀":true},"patternProperties":{"^..$":{"type":"integer"}}}`), validation.Options{})
	var compilation *validation.SchemaCompilationError
	result["strict_utf16_rejected"] = errors.As(err, &compilation)
	result["setup"], err = validationSetupProbe(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}
