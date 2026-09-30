package envelopes

import (
	"context"
	"fmt"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

type objectEnvelope[T any] struct {
	outer        *parser.ObjectSchema
	payload      parser.Schema[T]
	field, label string
}

func (s objectEnvelope[T]) Validate(ctx context.Context, input any) (T, []parser.Issue, error) {
	return s.validate(ctx, input, false)
}
func (s objectEnvelope[T]) ValidateSafe(ctx context.Context, input any) (T, []parser.Issue, error) {
	return s.validate(ctx, input, true)
}
func (s objectEnvelope[T]) validate(ctx context.Context, input any, safe bool) (T, []parser.Issue, error) {
	var zero T
	if s.payload == nil {
		return zero, nil, fmt.Errorf("envelope payload schema is required")
	}
	wrapped := parser.SchemaFunc[any](func(ctx context.Context, input any) (any, []parser.Issue, error) {
		if extended, ok := s.payload.(parser.SafeSchema[T]); safe && ok {
			return extended.ValidateSafe(ctx, input)
		}
		return s.payload.Validate(ctx, input)
	})
	parsed, issues, err := s.outer.Extend(parser.Field{Name: s.field, Schema: wrapped}).Validate(ctx, input)
	if err != nil {
		return zero, nil, err
	}
	if issues != nil {
		return zero, nil, &parser.ParseError{Message: s.label, Issues: issues}
	}
	value, exists := parsed.(map[string]any)[s.field]
	if !exists || value == nil {
		return zero, nil, nil
	}
	return value.(T), nil, nil
}

// APIGateway validates a REST event with the application's body schema.
func APIGateway[T any](payload parser.Schema[T]) parser.Schema[T] {
	return objectEnvelope[T]{outer: schemas.APIGatewayProxyEventSchema, payload: payload, field: "body", label: "Failed to parse API Gateway body"}
}

// APIGatewayV2 validates an HTTP API v2 event with the application's body schema.
func APIGatewayV2[T any](payload parser.Schema[T]) parser.Schema[T] {
	return objectEnvelope[T]{outer: schemas.APIGatewayProxyEventV2Schema, payload: payload, field: "body", label: "Failed to parse API Gateway HTTP body"}
}

// LambdaFunctionURL uses the HTTP API v2 shape without implicit Base64 decoding.
func LambdaFunctionURL[T any](payload parser.Schema[T]) parser.Schema[T] {
	return objectEnvelope[T]{outer: schemas.LambdaFunctionUrlSchema, payload: payload, field: "body", label: "Failed to parse Lambda function URL body"}
}

// VpcLattice validates the v1 event's snake_case metadata and body.
func VpcLattice[T any](payload parser.Schema[T]) parser.Schema[T] {
	return objectEnvelope[T]{outer: schemas.VpcLatticeSchema, payload: payload, field: "body", label: "Failed to parse VPC Lattice body"}
}

// VpcLatticeV2 validates the v2 event's camelCase metadata and body.
func VpcLatticeV2[T any](payload parser.Schema[T]) parser.Schema[T] {
	return objectEnvelope[T]{outer: schemas.VpcLatticeV2Schema, payload: payload, field: "body", label: "Failed to parse VPC Lattice v2 body"}
}
