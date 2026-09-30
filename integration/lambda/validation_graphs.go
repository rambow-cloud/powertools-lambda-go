package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

var validationGraphSchemas = sync.OnceValues(func() (map[string]*validation.Schema, error) {
	schemas, err := compileValidationSchemas(map[string]string{
		"ignored": `{"$schema":"http://json-schema.org/draft-07/schema#","if":{"$ref":"https://example.test/missing"},"then":true,"else":true}`,
		"active":  `{"if":{"$schema":"https://example.test/annotation","type":"number"},"then":true,"else":true,"properties":{"value":{"$ref":"#/if"}}}`,
		"dialect": `{"$schema":"http://json-schema.org/schema","type":"string"}`,
	})
	if err != nil {
		return nil, err
	}
	schemas["external"], err = validation.Compile(context.Background(), json.RawMessage(`{"$ref":"https://example.test/nested/number"}`), validation.Options{CompileOptions: validation.CompileOptions{
		ExternalSchemas: []any{json.RawMessage(`{"$id":"https://example.test/document","$defs":{"a/b~c":{"$id":"nested/","$defs":{"value":{"$id":"number","$schema":"https://example.test/annotation","type":"number"}}}}}`)},
	}})
	if err != nil {
		return nil, err
	}
	schemas["ordered"], err = validation.Compile(context.Background(), json.RawMessage(`{"$ref":"https://example.test/shared"}`), validation.Options{CompileOptions: validation.CompileOptions{
		ExternalSchemas: []any{
			json.RawMessage(`{"$id":"https://example.test/first","$defs":{"value":{"$id":"shared","type":"number"}}}`),
			json.RawMessage(`{"$id":"https://example.test/second","$defs":{"value":{"$id":"shared","type":"string"}}}`),
		},
	}})
	return schemas, err
})

func validationGraphProbe(ctx context.Context) (map[string]any, error) {
	schemas, err := validationGraphSchemas()
	if err != nil {
		return nil, err
	}
	result, err := collectValidationIssues(ctx, schemas, map[string]any{
		"active": map[string]any{"value": "bad"}, "external": "bad",
	})
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"ignored", "dialect", "ordered"} {
		result[name], err = schemas[name].Validate(ctx, "value")
		if err != nil {
			return nil, err
		}
	}
	_, err = validation.Compile(ctx, json.RawMessage(`{"if":{"$ref":"https://example.test/missing"},"then":false,"else":true}`), validation.Options{})
	var compilation *validation.SchemaCompilationError
	result["active_missing_rejected"] = errors.As(err, &compilation)
	return result, nil
}
