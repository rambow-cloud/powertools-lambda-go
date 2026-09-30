// Package validation validates Lambda inputs and outputs against JSON Schema.
package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
)

// Validator returns validation diagnostics separately from operational errors.
// Implementations reused across invocations must support concurrent calls.
type Validator interface {
	Validate(context.Context, any) ([]Issue, error)
}
type ValidatorFunc func(context.Context, any) ([]Issue, error)

func (f ValidatorFunc) Validate(ctx context.Context, input any) ([]Issue, error) {
	return f(ctx, input)
}

type Compiler interface {
	Compile(context.Context, any, CompileOptions) (Validator, error)
}
type CompileOptions struct {
	Regex   RegexOptions
	Formats map[string]func(any) error
	// NumberFormats applies callbacks to JSON numbers rather than strings.
	NumberFormats map[string]func(any) error
	// ExternalRefs registers URL-keyed documents in lexical URL order.
	ExternalRefs map[string]any
	// ExternalSchemas registers documents by $id in slice order, after ExternalRefs.
	// Use it when nested resource identities depend on registration order.
	ExternalSchemas []any
}
type Options struct {
	CompileOptions
	Envelope     string
	QueryOptions []jmespath.Option
	Compiler     Compiler
}

// Schema retains a reusable validator and optional compiled extraction expression.
type Schema struct {
	validator Validator
	envelope  *jmespath.Expression
	// The reference middleware treats a literal false schema as an absent option.
	wrapperSkip bool
}

func Compile(ctx context.Context, schema any, options Options) (*Schema, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	engine := options.Compiler
	if engine == nil {
		engine = jsonCompiler{}
	}
	validator, err := engine.Compile(ctx, schema, options.CompileOptions)
	if err != nil {
		return nil, &SchemaCompilationError{Err: err}
	}
	if validator == nil {
		return nil, &SchemaCompilationError{Err: fmt.Errorf("compiler returned a nil validator")}
	}
	result := &Schema{validator: validator}
	if value, ok := schema.(bool); ok {
		result.wrapperSkip = !value
	}
	if expression := strings.TrimSpace(options.Envelope); expression != "" {
		result.envelope, err = jmespath.Compile(expression, options.QueryOptions...)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Validate compiles for this call, then extracts and validates the payload.
// Use Compile once for warm-invocation reuse.
func Validate(ctx context.Context, payload, schema any, options Options) (any, error) {
	compiled, err := Compile(ctx, schema, options)
	if err != nil {
		return nil, err
	}
	return compiled.Validate(ctx, payload)
}
func (s *Schema) Validate(ctx context.Context, payload any) (any, error) {
	return s.validate(ctx, payload, true)
}
func (s *Schema) validate(ctx context.Context, payload any, extract bool) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.validator == nil {
		return nil, fmt.Errorf("compiled schema is required")
	}
	value := payload
	if extract && s.envelope != nil {
		var err error
		value, err = s.envelope.Search(payload)
		if err != nil {
			return nil, err
		}
	}
	issues, err := s.validator.Validate(ctx, value)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if issues != nil {
		return nil, &SchemaValidationError{Message: "Schema validation failed", Issues: issues}
	}
	return value, nil
}

func jsonValue(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var result any
	if err = decoder.Decode(&result); err != nil {
		return nil, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("payload must contain one JSON value")
	}
	return result, nil
}

func typed[T any](value any) (T, error) {
	if result, ok := value.(T); ok {
		return result, nil
	}
	var result T
	encoded, err := json.Marshal(value)
	if err == nil {
		err = json.Unmarshal(encoded, &result)
	}
	return result, err
}
