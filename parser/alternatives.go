package parser

import (
	"context"
	"fmt"
)

type nullable[T any] struct{ schema Schema[T] }
type anySchema[T any] struct{ schema Schema[T] }

func (s anySchema[T]) Validate(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, false)
}
func (s anySchema[T]) ValidateSafe(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, true)
}
func (s anySchema[T]) validate(ctx context.Context, input any, safe bool) (any, []Issue, error) {
	if s.schema == nil {
		return nil, nil, fmt.Errorf("schema is required")
	}
	return validateSchema(ctx, input, s.schema, safe)
}
func (s nullable[T]) Validate(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, false)
}
func (s nullable[T]) ValidateSafe(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, true)
}
func (s nullable[T]) validate(ctx context.Context, input any, safe bool) (any, []Issue, error) {
	if s.schema == nil {
		return nil, nil, fmt.Errorf("nullable schema is required")
	}
	if input == nil {
		return nil, nil, nil
	}
	return validateSchema(ctx, input, s.schema, safe)
}

// Union tries branches in order and preserves nested failures when none succeeds.
// A sole branch with only continuable check failures retains its own diagnostics.
func Union(schemas ...Schema[any]) Schema[any] { return union{append([]Schema[any](nil), schemas...)} }

type union struct{ schemas []Schema[any] }

func (s union) Validate(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, false)
}
func (s union) ValidateSafe(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, true)
}
func (s union) validate(ctx context.Context, input any, safe bool) (any, []Issue, error) {
	if len(s.schemas) == 0 {
		return nil, nil, fmt.Errorf("union requires at least one schema")
	}
	for _, schema := range s.schemas {
		if schema == nil {
			return nil, nil, fmt.Errorf("union schema is required")
		}
	}
	if len(s.schemas) == 1 {
		return validateSchema(ctx, input, s.schemas[0], safe)
	}
	branches := make([][]Issue, 0, len(s.schemas))
	eligible := -1
	eligibleCount := 0
	var candidate any
	for i, schema := range s.schemas {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		value, issues, err := validateSchema(ctx, input, schema, safe)
		if err != nil {
			return nil, nil, err
		}
		if issues == nil {
			return value, nil, nil
		}
		branches = append(branches, Prefix(issues))
		if continuable(issues) {
			eligible = i
			eligibleCount++
			candidate = value
		}
	}
	if eligibleCount == 1 {
		return candidate, branches[eligible], nil
	}
	return nil, []Issue{{Code: "invalid_union", Message: "Invalid input", Errors: branches}}, nil
}

type refined[T any] struct {
	schema    Schema[T]
	predicate func(T) bool
	message   string
}

func (s refined[T]) Validate(ctx context.Context, input any) (T, []Issue, error) {
	return s.validate(ctx, input, false)
}
func (s refined[T]) ValidateSafe(ctx context.Context, input any) (T, []Issue, error) {
	return s.validate(ctx, input, true)
}
func (s refined[T]) validate(ctx context.Context, input any, safe bool) (T, []Issue, error) {
	var zero T
	if s.schema == nil || s.predicate == nil {
		return zero, nil, fmt.Errorf("refinement schema and predicate are required")
	}
	value, issues, err := validateSchema(ctx, input, s.schema, safe)
	if err != nil || !continuable(issues) {
		return value, issues, err
	}
	if err := ctx.Err(); err != nil {
		return zero, nil, err
	}
	if !s.predicate(value) {
		issues = append(Prefix(issues), Issue{Code: "custom", Message: s.message, Continuable: true})
	}
	return value, issues, nil
}
