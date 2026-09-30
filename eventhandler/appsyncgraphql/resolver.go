package appsyncgraphql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

type Resolver struct{ *Router }

func New(options Options) *Resolver { return &Resolver{NewRouter(options)} }

// Resolve accepts decoded JSON events or batches. Invalid input returns nil after
// a warning. Handler errors become response envelopes except the two framework
// exceptions, which propagate to Lambda. Events are passed through without copying.
func (r *Resolver) Resolve(ctx context.Context, input any) (any, error) {
	events, batch := input.([]any)
	if typed, ok := input.([]Event); ok {
		batch = true
		events = make([]any, len(typed))
		for i := range typed {
			events[i] = typed[i]
		}
	}
	var event any = input
	if batch {
		for _, value := range events {
			if !isEvent(value) {
				r.diagnostic(ctx, "warn", "Received a batch event that is not compatible with this resolver", nil)
				return nil, nil
			}
		}
		if len(events) == 0 {
			// The reference fails while constructing its error message for event[0].
			return nil, &TypeError{Message: "Cannot read properties of undefined (reading 'info')"}
		}
		event = events[0]
	} else if !isEvent(event) {
		r.diagnostic(ctx, "warn", "Received an event that is not compatible with this resolver", nil)
		return nil, nil
	}
	typeName, field := eventRoute(event)
	value, err := call(func() (any, error) {
		item, found := r.lookup(ctx, typeName, field, batch)
		if !found {
			prefix := "No resolver found for "
			if batch {
				prefix = "No batch resolver found for "
			}
			return nil, &ResolverNotFoundException{Message: prefix + typeName + "-" + field}
		}
		if batch {
			return r.executeBatch(ctx, events, item)
		}
		return item.handler(ctx, event.(map[string]any)["arguments"], event)
	})
	if err != nil {
		return r.handleError(ctx, field, err)
	}
	return value, nil
}

func (r *Resolver) executeBatch(ctx context.Context, events []any, item route) (any, error) {
	r.diagnostic(ctx, "debug", fmt.Sprintf("Aggregate flag aggregate=%t & graceful error handling flag throwOnError=%t", !item.options.Individual, item.options.ThrowOnError), nil)
	if !item.options.Individual {
		value, err := call(func() (any, error) { return item.handler(ctx, events, events) })
		if err != nil {
			return nil, err
		}
		v := reflect.ValueOf(value)
		if !v.IsValid() || (v.Kind() != reflect.Slice && v.Kind() != reflect.Array) || (v.Kind() == reflect.Slice && v.IsNil()) {
			return nil, &InvalidBatchResponseException{Message: "The response must be an array when using batch resolvers"}
		}
		// Materialize the top-level array so []byte cannot become a Base64 string
		// when the Lambda SDK serializes the aggregate response.
		result := make([]any, v.Len())
		for i := range result {
			result[i] = v.Index(i).Interface()
		}
		return result, nil
	}
	results := make([]any, 0, len(events))
	for i, event := range events {
		value, err := call(func() (any, error) { return item.handler(ctx, event.(map[string]any)["arguments"], event) })
		if err != nil {
			if item.options.ThrowOnError {
				return nil, err
			}
			r.diagnostic(ctx, "error", "", err)
			_, field := eventRoute(event)
			r.diagnostic(ctx, "debug", fmt.Sprintf("Failed to process event #%d from field '%s'", i+1, field), nil)
			value = nil
		}
		results = append(results, value)
	}
	return results, nil
}

func (r *Resolver) handleError(ctx context.Context, field string, err error) (any, error) {
	r.diagnostic(ctx, "error", "An error occurred in handler "+field, err)
	var missing *ResolverNotFoundException
	var invalid *InvalidBatchResponseException
	if errors.As(err, &missing) || errors.As(err, &invalid) {
		return nil, err
	}
	var unknown thrownValue
	if errors.As(err, &unknown) {
		return map[string]any{"error": "An unknown error occurred"}, nil
	}
	name := errorName(err)
	if handler := r.lookupException(ctx, name); handler != nil {
		r.diagnostic(ctx, "debug", "Calling exception handler for error: "+name, nil)
		value, failure := call(func() (any, error) { return handler(ctx, err) })
		if failure == nil {
			return value, nil
		}
		r.diagnostic(ctx, "error", "Exception handler for "+name+" threw an error", failure)
	}
	return map[string]any{"error": name + " - " + err.Error()}, nil
}
