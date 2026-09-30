package validation

import (
	"context"
	"errors"

	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
)

// WrapHandler validates a cloned inbound event and a successful outbound response.
// Only inbound validation applies an envelope; business errors and panics propagate.
func WrapHandler[I, T, R any](inbound, outbound *Schema, handler func(context.Context, T) (R, error)) func(context.Context, I) (R, error) {
	return func(ctx context.Context, input I) (R, error) {
		var zero R
		ctx = invocation.Ensure(ctx)
		var value any = input
		if inbound != nil && !inbound.wrapperSkip {
			var err error
			value, err = jsonValue(input)
			if err != nil {
				return zero, stageError("Inbound", err)
			}
			value, err = inbound.Validate(ctx, value)
			if err != nil {
				return zero, stageError("Inbound", err)
			}
		}
		parsed, err := typed[T](value)
		if err != nil {
			return zero, stageError("Inbound", err)
		}
		result, err := handler(ctx, parsed)
		if err != nil {
			return result, err
		}
		if outbound == nil || outbound.wrapperSkip {
			return result, nil
		}
		validated, err := outbound.validate(ctx, result, false)
		if err != nil {
			return zero, stageError("Outbound", err)
		}
		output, err := typed[R](validated)
		if err != nil {
			return zero, stageError("Outbound", err)
		}
		return output, nil
	}
}
func stageError(stage string, err error) error {
	failure := &SchemaValidationError{Message: stage + " schema validation failed", Err: err}
	var validation *SchemaValidationError
	if errors.As(err, &validation) {
		failure.Issues = validation.Issues
		failure.Err = validation.Err
	}
	return failure
}
