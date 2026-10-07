package main

import (
	"context"
	jsonv1 "encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

type validationPair struct{ input, output *validation.Schema }

func compileValidationSchemas(sources map[string]string) (map[string]*validation.Schema, error) {
	compiled := map[string]*validation.Schema{}
	for name, source := range sources {
		schema, err := validation.Compile(context.Background(), jsonv1.RawMessage(source), validation.Options{})
		if err != nil {
			return nil, err
		}
		compiled[name] = schema
	}
	return compiled, nil
}

func collectValidationIssues(ctx context.Context, schemas map[string]*validation.Schema, payloads map[string]any) (map[string]any, error) {
	result := map[string]any{}
	for name, payload := range payloads {
		_, err := schemas[name].Validate(ctx, payload)
		var failure *validation.SchemaValidationError
		if !errors.As(err, &failure) {
			return nil, fmt.Errorf("missing %s validation diagnostics: %v", name, err)
		}
		result[name] = failure.Issues
	}
	return result, nil
}

var validationSchemas = sync.OnceValues(func() (validationPair, error) {
	input, err := validation.Compile(context.Background(), map[string]any{
		"type": "object", "required": []string{"id", "amount"},
		"properties": map[string]any{"id": map[string]any{"type": "string", "format": "order"}, "amount": map[string]any{"$ref": "https://example.test/amount"}},
	}, validation.Options{Envelope: "body", CompileOptions: validation.CompileOptions{
		Formats: map[string]func(any) error{"order": func(value any) error {
			if strings.HasPrefix(value.(string), "order-") {
				return nil
			}
			return fmt.Errorf("order prefix required")
		}},
		ExternalRefs: map[string]any{"https://example.test/amount": map[string]any{"type": "integer", "minimum": 1}},
	}})
	if err != nil {
		return validationPair{}, err
	}
	output, err := validation.Compile(context.Background(), map[string]any{"type": "string", "const": "ok"}, validation.Options{Envelope: "not_applied_to_output"})
	return validationPair{input, output}, err
})

func validationProbe(ctx context.Context) (map[string]any, error) {
	pair, err := validationSchemas()
	if err != nil {
		return nil, err
	}
	inbound, outbound := pair.input, pair.output
	result := map[string]any{}
	calls := 0
	handler := validation.WrapHandler[map[string]any](inbound, outbound, func(_ context.Context, input map[string]any) (string, error) {
		calls++
		input["id"] = "changed"
		return "ok", nil
	})
	input := map[string]any{"body": map[string]any{"id": "order-a", "amount": 2}}
	value, err := handler(ctx, input)
	if err != nil {
		return nil, err
	}
	result["value"] = value
	result["isolated"] = input["body"].(map[string]any)["id"] == "order-a"
	_, err = handler(ctx, map[string]any{"body": map[string]any{"id": "bad", "amount": 0}})
	var failure *validation.SchemaValidationError
	if !errors.As(err, &failure) {
		return nil, fmt.Errorf("invalid payload reached handler: %v", err)
	}
	result["inbound_error"] = failure.Error()
	result["issues"] = failure.Issues
	result["calls"] = calls
	invalidOutput := validation.WrapHandler[any](nil, outbound, func(context.Context, any) (string, error) { return "bad", nil })
	_, err = invalidOutput(ctx, nil)
	if !errors.As(err, &failure) {
		return nil, fmt.Errorf("invalid output accepted: %v", err)
	}
	result["outbound_error"] = failure.Error()
	business := errors.New("business")
	failed := validation.WrapHandler[any](nil, outbound, func(context.Context, any) (string, error) { return "partial", business })
	value, err = failed(ctx, nil)
	result["business_preserved"] = value == "partial" && err == business
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = inbound.Validate(cancelled, input)
	result["cancelled"] = errors.Is(err, context.Canceled)
	result["regex"], err = validationRegexProbe(ctx)
	if err != nil {
		return nil, err
	}
	result["applicators"], err = validationApplicatorProbe(ctx)
	if err != nil {
		return nil, err
	}
	result["references"], err = validationReferenceProbe(ctx)
	if err != nil {
		return nil, err
	}
	result["graphs"], err = validationGraphProbe(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}
