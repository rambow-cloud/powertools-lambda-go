package parser

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"strings"
)

// Pipe validates transformed output and preserves safe-mode envelope aggregation.
func Pipe[A, B any](first Schema[A], next Schema[B]) Schema[B] { return pipeline[A, B]{first, next} }

type pipeline[A, B any] struct {
	first Schema[A]
	next  Schema[B]
}

func (s pipeline[A, B]) Validate(ctx context.Context, input any) (B, []Issue, error) {
	return s.validate(ctx, input, false)
}
func (s pipeline[A, B]) ValidateSafe(ctx context.Context, input any) (B, []Issue, error) {
	return s.validate(ctx, input, true)
}
func (s pipeline[A, B]) validate(ctx context.Context, input any, safe bool) (B, []Issue, error) {
	var zero B
	if s.first == nil || s.next == nil {
		return zero, nil, fmt.Errorf("pipe schemas are required")
	}
	value, issues, err := validateSchema(ctx, input, s.first, safe)
	if err != nil || issues != nil {
		return zero, issues, err
	}
	return validateSchema(ctx, value, s.next, safe)
}
func validateSchema[T any](ctx context.Context, input any, schema Schema[T], safe bool) (T, []Issue, error) {
	var zero T
	if schema == nil {
		return zero, nil, fmt.Errorf("schema is required")
	}
	if err := ctx.Err(); err != nil {
		return zero, nil, err
	}
	if extended, ok := schema.(SafeSchema[T]); safe && ok {
		value, issues, err := extended.ValidateSafe(ctx, input)
		return childResult(value, issues, err)
	}
	value, issues, err := schema.Validate(ctx, input)
	return childResult(value, issues, err)
}

func childResult[T any](value T, issues []Issue, err error) (T, []Issue, error) {
	var failure *ParseError
	if errors.As(err, &failure) {
		issues = Prefix(failure.Issues)
		if issues == nil {
			issues = []Issue{}
		}
		// An envelope ParseError has no validated output for later refinements.
		for i := range issues {
			issues[i].Continuable = false
		}
		err = nil
	}
	return value, issues, err
}

// Enum validates a string against a fixed set of values.
func Enum(values ...string) Schema[any] {
	allowed := append([]string(nil), values...)
	encoded := make([]string, len(allowed))
	for i, value := range allowed {
		data, _ := json.Marshal(value)
		encoded[i] = string(data)
	}
	message := "Invalid option: expected one of " + strings.Join(encoded, "|")
	if len(allowed) == 1 {
		message = "Invalid input: expected " + encoded[0]
	}
	return SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) {
		for _, value := range allowed {
			if input == value {
				return value, nil, nil
			}
		}
		return nil, []Issue{{Code: "invalid_value", Message: message}}, nil
	})
}
