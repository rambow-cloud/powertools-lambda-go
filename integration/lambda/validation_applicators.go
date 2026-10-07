package main

import (
	"context"
	jsonv1 "encoding/json"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

var validationApplicatorSchemas = sync.OnceValues(func() (map[string]*validation.Schema, error) {
	schemas := map[string]string{
		"nested": `{"properties":{"zebra":{"if":{"required":["card"]},"then":{"dependencies":{"card":["zip","name"]}},"propertyNames":{"pattern":"^[a-z]+$"}},"alpha":{"items":[{"type":"integer"}],"additionalItems":false}}}`,
		"one_of": `{"oneOf":[{"type":"string"},{"type":"number"},{"type":"integer"}]}`,
	}
	return compileValidationSchemas(schemas)
})

func validationApplicatorProbe(ctx context.Context) (map[string]any, error) {
	schemas, err := validationApplicatorSchemas()
	if err != nil {
		return nil, err
	}
	payloads := map[string]any{
		"nested": jsonv1.RawMessage(`{"alpha":[false,2],"zebra":{"card":true,"BAD/~":1}}`),
		"one_of": 6,
	}
	return collectValidationIssues(ctx, schemas, payloads)
}
