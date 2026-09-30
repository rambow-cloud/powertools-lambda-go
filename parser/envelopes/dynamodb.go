package envelopes

import (
	"context"
	"fmt"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

// Images retains missing images separately from present images with zero values.
type Images[T any] struct {
	NewImage *T `json:"NewImage,omitempty"`
	OldImage *T `json:"OldImage,omitempty"`
}

// DynamoDBStream preserves one old/new image pair per input record.
func DynamoDBStream[T any](payload parser.Schema[T]) parser.Schema[[]Images[T]] {
	return recordEnvelope[Images[T]]{outer: schemas.DynamoDBStreamSchema, payload: imageSchema[T]{payload}, recordsPath: []string{"Records"}, payloadPath: []string{"dynamodb"}, outerError: [2]string{"Failed to parse DynamoDB Stream envelope", "Failed to parse DynamoDB Stream envelope"}, recordLabel: [2]string{"DynamoDB record", "record"}}
}

type imageSchema[T any] struct{ payload parser.Schema[T] }

func (s imageSchema[T]) Validate(ctx context.Context, input any) (Images[T], []parser.Issue, error) {
	return s.validate(ctx, input, false)
}
func (s imageSchema[T]) ValidateSafe(ctx context.Context, input any) (Images[T], []parser.Issue, error) {
	return s.validate(ctx, input, true)
}
func (s imageSchema[T]) validate(ctx context.Context, input any, safe bool) (Images[T], []parser.Issue, error) {
	var result Images[T]
	var issues []parser.Issue
	if s.payload == nil {
		return result, nil, fmt.Errorf("DynamoDB image schema is required")
	}
	for _, name := range []string{"NewImage", "OldImage"} {
		value, exists := input.(map[string]any)[name]
		if !exists {
			continue
		}
		parsed, failures, err := s.payload.Validate(ctx, value)
		if err != nil {
			return result, nil, err
		}
		if failures != nil {
			if issues == nil {
				issues = []parser.Issue{}
			}
			issues = append(issues, parser.Prefix(failures, name)...)
			if !safe {
				break
			}
		} else if name == "NewImage" {
			result.NewImage = &parsed
		} else {
			result.OldImage = &parsed
		}
	}
	return result, issues, nil
}
