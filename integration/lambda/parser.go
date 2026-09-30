package main

import (
	"context"
	"errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
)

var orderInputSchema = parser.Typed[order](parser.Object(
	parser.Field{Name: "id", Schema: parser.String()},
	parser.Field{Name: "amount", Schema: parser.Refine(parser.Number(), func(value any) bool { return value.(float64) >= 0 }, "amount must be non-negative")},
))

type parserResult struct {
	Orders      []order        `json:"orders"`
	SafeIssues  []parser.Issue `json:"safe_issues"`
	FirstIssues []parser.Issue `json:"first_issues"`
	Detail      order          `json:"detail"`
	Streams     map[string]any `json:"streams"`
	HTTP        map[string]any `json:"http"`
	Services    map[string]any `json:"services"`
	Identity    map[string]any `json:"identity"`
	Errors      map[string]any `json:"errors"`
}

func parserProbe(ctx context.Context) (parserResult, error) {
	var result parserResult
	record := events.SQSMessage{MessageId: "1", ReceiptHandle: "receipt", Body: `{"id":"a","amount":1}`, Attributes: map[string]string{"ApproximateReceiveCount": "1", "ApproximateFirstReceiveTimestamp": "1", "SenderId": "sender", "SentTimestamp": "1"}, MessageAttributes: map[string]events.SQSMessageAttribute{}, Md5OfBody: "hash", EventSource: "aws:sqs", EventSourceARN: "arn:queue", AWSRegion: "ap-east-1"}
	schema := envelopes.SQS(parser.JSONStringified(orderInputSchema))
	var err error
	result.Orders, err = parser.Parse(ctx, events.SQSEvent{Records: []events.SQSMessage{record}}, schema)
	if err != nil {
		return result, err
	}
	first, second := record, record
	first.Body = `{"id":1,"amount":-1}`
	second.Body = `{"id":"b","amount":-1}`
	invalid := events.SQSEvent{Records: []events.SQSMessage{first, second}}
	safe, err := parser.SafeParse(ctx, invalid, schema)
	if err != nil {
		return result, err
	}
	if safe.Success || safe.Error == nil {
		return result, errors.New("invalid SQS records passed safe parsing")
	}
	result.SafeIssues = safe.Error.Issues
	_, err = parser.Parse(ctx, invalid, schema)
	var failure *parser.ParseError
	if !errors.As(err, &failure) {
		return result, errors.New("invalid SQS records passed ordinary parsing")
	}
	result.FirstIssues = failure.Issues
	event := map[string]any{"version": "0", "id": "event", "source": "orders", "account": "account", "time": "2026-09-15T00:00:00Z", "region": "ap-east-1", "resources": []any{}, "detail-type": "order", "detail": map[string]any{"id": "detail", "amount": 2}}
	result.Detail, err = parser.Parse(ctx, event, envelopes.EventBridge(orderInputSchema))
	if err != nil {
		return result, err
	}
	result.Streams, err = parserStreamsProbe(ctx, record)
	if err != nil {
		return result, err
	}
	result.HTTP, err = parserHTTPProbe(ctx)
	if err != nil {
		return result, err
	}
	result.Services, err = parserServicesProbe(ctx)
	if err != nil {
		return result, err
	}
	result.Identity, err = parserIdentityProbe(ctx)
	if err != nil {
		return result, err
	}
	result.Errors, err = parserErrorsProbe(ctx, record)
	return result, err
}
