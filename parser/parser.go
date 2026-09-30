// Package parser validates and transforms Lambda payloads using typed schemas.
package parser

import (
	"context"
	"errors"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
)

// Issue maps Standard Schema's message/path contract. Code and Expected retain
// useful built-in diagnostics without requiring a specific validation engine.
type Issue struct {
	Message  string    `json:"message"`
	Path     []any     `json:"path,omitempty"`
	Code     string    `json:"code,omitempty"`
	Expected string    `json:"expected,omitempty"`
	Errors   [][]Issue `json:"errors,omitempty"`
	// Continuable marks a check failure that preserves the parsed value's type.
	// It controls union/refinement evaluation and is not part of serialized errors.
	Continuable bool `json:"-"`
}

// Schema returns nil issues for success. A non-nil slice, including an empty
// slice, denotes validation failure. Unexpected errors propagate even in safe
// parsing, matching a thrown Standard Schema validator exception.
type Schema[T any] interface {
	Validate(context.Context, any) (T, []Issue, error)
}
type SchemaFunc[T any] func(context.Context, any) (T, []Issue, error)

func (f SchemaFunc[T]) Validate(ctx context.Context, input any) (T, []Issue, error) {
	return f(ctx, input)
}

// SafeSchema optionally aggregates additional envelope failures in safe mode.
type SafeSchema[T any] interface {
	ValidateSafe(context.Context, any) (T, []Issue, error)
}

type ParseError struct {
	Message string
	Issues  []Issue
	Err     error
}

func (e *ParseError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "Failed to parse schema"
}
func (e *ParseError) Unwrap() error { return e.Err }

// Result includes the original event only on validation failure. The event is
// retained by reference, as in the reference safeParse contract.
type Result[T any] struct {
	Success       bool
	Data          T
	Error         *ParseError
	OriginalEvent any
}

func Parse[T any](ctx context.Context, input any, schema Schema[T]) (T, error) {
	result, err := validate(ctx, input, schema, false)
	if err != nil {
		return result.Data, err
	}
	if !result.Success {
		return result.Data, result.Error
	}
	return result.Data, nil
}
func SafeParse[T any](ctx context.Context, input any, schema Schema[T]) (Result[T], error) {
	return validate(ctx, input, schema, true)
}

func validate[T any](ctx context.Context, input any, schema Schema[T], safe bool) (Result[T], error) {
	var result Result[T]
	if schema == nil {
		return result, fmt.Errorf("parser schema is required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	var issues []Issue
	var err error
	if extended, ok := schema.(SafeSchema[T]); safe && ok {
		result.Data, issues, err = extended.ValidateSafe(ctx, input)
	} else {
		result.Data, issues, err = schema.Validate(ctx, input)
	}
	if err != nil {
		var parsing *ParseError
		if !errors.As(err, &parsing) {
			return result, err
		}
		result.Error = parsing
	} else if issues != nil {
		result.Error = &ParseError{Message: "Failed to parse schema", Issues: issues}
	}
	if result.Error != nil {
		var zero T
		result.Data = zero
		result.OriginalEvent = input
		return result, nil
	}
	result.Success = true
	return result, nil
}

func Prefix(issues []Issue, path ...any) []Issue {
	if issues == nil {
		return nil
	}
	result := make([]Issue, len(issues))
	for i, issue := range issues {
		result[i] = issue
		result[i].Path = append(append([]any{}, path...), issue.Path...)
		if issue.Errors != nil {
			result[i].Errors = make([][]Issue, len(issue.Errors))
			for j, branch := range issue.Errors {
				result[i].Errors[j] = Prefix(branch)
			}
		}
	}
	return result
}

func continuable(issues []Issue) bool {
	if len(issues) == 0 {
		return issues == nil
	}
	for _, issue := range issues {
		if !issue.Continuable {
			return false
		}
	}
	return true
}

// WrapHandler parses before calling the business handler and preserves shared
// invocation identity. Context, operational errors and panics are not replaced.
func WrapHandler[I, T, R any](schema Schema[T], handler func(context.Context, T) (R, error)) func(context.Context, I) (R, error) {
	return func(ctx context.Context, event I) (R, error) {
		var zero R
		if handler == nil {
			return zero, fmt.Errorf("parser handler is required")
		}
		ctx = invocation.Ensure(ctx)
		parsed, err := Parse(ctx, event, schema)
		if err != nil {
			return zero, err
		}
		return handler(ctx, parsed)
	}
}
func WrapSafeHandler[I, T, R any](schema Schema[T], handler func(context.Context, Result[T]) (R, error)) func(context.Context, I) (R, error) {
	return func(ctx context.Context, event I) (R, error) {
		var zero R
		if handler == nil {
			return zero, fmt.Errorf("parser handler is required")
		}
		ctx = invocation.Ensure(ctx)
		result, err := SafeParse(ctx, event, schema)
		if err != nil {
			return zero, err
		}
		return handler(ctx, result)
	}
}
