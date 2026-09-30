package batch

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
)

func NewSQS[R any](options Options) (*Processor[events.SQSMessage, R], error) {
	return New[events.SQSMessage, R](Source[events.SQSMessage]{Identifier: func(record events.SQSMessage) string { return record.MessageId }}, options)
}
func NewSQSFIFO[R any](options Options) (*Processor[events.SQSMessage, R], error) {
	return New[events.SQSMessage, R](Source[events.SQSMessage]{Identifier: func(record events.SQSMessage) string { return record.MessageId }, GroupID: func(record events.SQSMessage) string { return record.Attributes["MessageGroupId"] }}, options)
}
func NewKinesis[R any](options Options) (*Processor[events.KinesisEventRecord, R], error) {
	return New[events.KinesisEventRecord, R](Source[events.KinesisEventRecord]{Identifier: func(record events.KinesisEventRecord) string { return record.Kinesis.SequenceNumber }}, options)
}
func NewDynamoDB[R any](options Options) (*Processor[events.DynamoDBEventRecord, R], error) {
	return New[events.DynamoDBEventRecord, R](Source[events.DynamoDBEventRecord]{Identifier: func(record events.DynamoDBEventRecord) string { return record.Change.SequenceNumber }, OmitEmptyIdentifier: true}, options)
}

// RecordProcessor allows custom processors without subclassing or shared stores.
type RecordProcessor[T, R any] interface {
	Process(context.Context, []T, Handler[T, R]) (Report[T, R], error)
}

// WrapHandler adapts any typed event envelope and reuses shared invocation context.
func WrapHandler[E, T, R any](processor RecordProcessor[T, R], records func(E) []T, handler Handler[T, R]) func(context.Context, E) (Response, error) {
	return func(ctx context.Context, event E) (Response, error) {
		empty := Response{BatchItemFailures: []ItemFailure{}}
		if processor == nil || records == nil {
			return empty, &ConfigurationError{"processor and event extractor are required"}
		}
		items := records(event)
		if items == nil {
			return empty, &UnexpectedBatchTypeError{}
		}
		report, err := processor.Process(invocation.Ensure(ctx), items, handler)
		return report.Response, err
	}
}
func WrapSQS[R any](processor RecordProcessor[events.SQSMessage, R], handler Handler[events.SQSMessage, R]) func(context.Context, events.SQSEvent) (Response, error) {
	return WrapHandler(processor, func(event events.SQSEvent) []events.SQSMessage { return event.Records }, handler)
}
func WrapKinesis[R any](processor RecordProcessor[events.KinesisEventRecord, R], handler Handler[events.KinesisEventRecord, R]) func(context.Context, events.KinesisEvent) (Response, error) {
	return WrapHandler(processor, func(event events.KinesisEvent) []events.KinesisEventRecord { return event.Records }, handler)
}
func WrapDynamoDB[R any](processor RecordProcessor[events.DynamoDBEventRecord, R], handler Handler[events.DynamoDBEventRecord, R]) func(context.Context, events.DynamoDBEvent) (Response, error) {
	return WrapHandler(processor, func(event events.DynamoDBEvent) []events.DynamoDBEventRecord { return event.Records }, handler)
}

// WithParser composes record parsing and processing while keeping original record
// identifiers in the processor report. The parser owns schema/transform policy.
func WithParser[T, P, R any](parse func(context.Context, T) (P, error), handler Handler[P, R]) Handler[T, R] {
	return func(ctx context.Context, record T) (R, error) {
		var zero R
		if parse == nil || handler == nil {
			return zero, &ConfigurationError{"parser and parsed-record handler are required"}
		}
		parsed, err := parse(ctx, record)
		if err != nil {
			return zero, &ParsingError{Err: err}
		}
		return handler(ctx, parsed)
	}
}

type ParsingError struct{ Err error }

func (e *ParsingError) Error() string { return "record parsing failed: " + e.Err.Error() }
func (e *ParsingError) Unwrap() error { return e.Err }
