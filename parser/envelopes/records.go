package envelopes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

// recordEnvelope shares traversal and failure collection after schema-specific decoding.
type recordEnvelope[T any] struct {
	outer                    parser.Schema[any]
	payload                  parser.Schema[T]
	recordsPath, payloadPath []string
	outerError, recordLabel  [2]string
}

func (s recordEnvelope[T]) Validate(ctx context.Context, input any) ([]T, []parser.Issue, error) {
	return s.validate(ctx, input, false)
}
func (s recordEnvelope[T]) ValidateSafe(ctx context.Context, input any) ([]T, []parser.Issue, error) {
	return s.validate(ctx, input, true)
}
func pathValue(input any, path []string) any {
	for _, key := range path {
		input = input.(map[string]any)[key]
	}
	return input
}
func (s recordEnvelope[T]) validate(ctx context.Context, input any, safe bool) ([]T, []parser.Issue, error) {
	mode := 0
	if safe {
		mode = 1
	}
	if s.payload == nil {
		return nil, nil, fmt.Errorf("envelope payload schema is required")
	}
	value, issues, err := s.outer.Validate(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	if issues != nil {
		return nil, nil, &parser.ParseError{Message: s.outerError[mode], Issues: issues}
	}
	records := pathValue(value, s.recordsPath).([]any)
	result := make([]T, 0, len(records))
	var failures []parser.Issue
	indexes := []string{}
	for index, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		var parsed T
		var issues []parser.Issue
		var err error
		body := pathValue(record, s.payloadPath)
		if extended, ok := s.payload.(parser.SafeSchema[T]); safe && ok {
			parsed, issues, err = extended.ValidateSafe(ctx, body)
		} else {
			parsed, issues, err = s.payload.Validate(ctx, body)
		}
		if err != nil {
			return nil, nil, err
		}
		if issues != nil {
			indexes = append(indexes, strconv.Itoa(index))
			if failures == nil {
				failures = []parser.Issue{}
			}
			prefix := make([]any, 0, len(s.recordsPath)+1+len(s.payloadPath))
			for _, key := range s.recordsPath {
				prefix = append(prefix, key)
			}
			prefix = append(prefix, index)
			for _, key := range s.payloadPath {
				prefix = append(prefix, key)
			}
			failures = append(failures, parser.Prefix(issues, prefix...)...)
			if !safe {
				break
			}
		} else {
			result = append(result, parsed)
		}
	}
	if failures != nil {
		message := "Failed to parse " + s.recordLabel[mode] + " at index " + indexes[0]
		if len(indexes) > 1 {
			message = "Failed to parse " + s.recordLabel[mode] + "s at indexes " + strings.Join(indexes, ", ")
		}
		return nil, nil, &parser.ParseError{Message: message, Issues: failures}
	}
	return result, nil, nil
}

// SNS passes the notification's Message string to the payload schema unchanged.
func SNS[T any](payload parser.Schema[T]) parser.Schema[[]T] {
	return recordEnvelope[T]{outer: schemas.SnsSchema, payload: payload, recordsPath: []string{"Records"}, payloadPath: []string{"Sns", "Message"}, outerError: [2]string{"Failed to parse SNS envelope", "Failed to parse SNS envelope"}, recordLabel: [2]string{"SNS record", "SNS message"}}
}

// SNSSQS decodes the notification JSON, then validates its Message string.
func SNSSQS[T any](payload parser.Schema[T]) parser.Schema[[]T] {
	message := parser.Transform(parser.JSONStringified[any](schemas.SnsSqsNotificationSchema), func(_ context.Context, value any) (any, error) { return value.(map[string]any)["Message"], nil })
	return SQS(parser.Pipe(message, payload))
}

// Kinesis validates the decoded JSON/text data provided by the Kinesis schema.
func Kinesis[T any](payload parser.Schema[T]) parser.Schema[[]T] {
	return recordEnvelope[T]{outer: schemas.KinesisDataStreamSchema, payload: payload, recordsPath: []string{"Records"}, payloadPath: []string{"kinesis", "data"}, outerError: [2]string{"Failed to parse Kinesis Data Stream envelope", "Failed to parse Kinesis Data Stream envelope"}, recordLabel: [2]string{"Kinesis Data Stream record", "Kinesis Data Stream record"}}
}

// KinesisFirehose passes decoded text to the payload schema; JSON parsing is explicit.
func KinesisFirehose[T any](payload parser.Schema[T]) parser.Schema[[]T] {
	return recordEnvelope[T]{outer: schemas.KinesisFirehoseSchema, payload: payload, recordsPath: []string{"records"}, payloadPath: []string{"data"}, outerError: [2]string{"Failed to parse Kinesis Firehose envelope", "Failed to parse Kinesis Firehose envelope"}, recordLabel: [2]string{"Kinesis Firehose record", "Kinesis Firehose record"}}
}

// CloudWatch decompresses the log envelope and validates each message string.
func CloudWatch[T any](payload parser.Schema[T]) parser.Schema[[]T] {
	return recordEnvelope[T]{outer: schemas.CloudWatchLogsSchema, payload: payload, recordsPath: []string{"awslogs", "data", "logEvents"}, payloadPath: []string{"message"}, outerError: [2]string{"Failed to parse CloudWatch Log envelope", "Failed to parse CloudWatch Log envelope"}, recordLabel: [2]string{"CloudWatch log event", "CloudWatch Log message"}}
}
