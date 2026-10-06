package envelopes

import (
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

// SQS validates record metadata and passes each body string to the payload schema.
func SQS[T any](payload parser.Schema[T]) parser.Schema[[]T] {
	return recordEnvelope[T]{outer: schemas.SqsSchema, payload: payload, recordsPath: []string{"Records"}, payloadPath: []string{"body"}, outerError: [2]string{"Failed to parse SQS Envelope", "Failed to parse SQS envelope"}, recordLabel: [2]string{"SQS Record", "SQS Record"}}
}

// EventBridge validates metadata together with the selected detail schema.
func EventBridge[T any](payload parser.Schema[T]) parser.Schema[T] {
	return objectEnvelope[T]{outer: schemas.EventBridgeSchema, payload: payload, field: "detail", label: "Failed to parse EventBridge envelope"}
}
