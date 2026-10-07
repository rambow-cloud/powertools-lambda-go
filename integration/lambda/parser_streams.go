package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	json "encoding/json/v2"
	"math/big"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
)

func parserStreamsProbe(ctx context.Context, record events.SQSMessage) (map[string]any, error) {
	body := `{"id":"stream","amount":1}`
	notification := map[string]any{"TopicArn": "arn:topic", "UnsubscribeUrl": "https://example.test/unsubscribe", "Type": "Notification", "Message": body, "MessageId": "1", "Timestamp": "2026-09-15T00:00:00Z"}
	raw, err := json.Marshal(notification)
	if err != nil {
		return nil, err
	}
	record.Body = string(raw)
	sns := map[string]any{"Records": []any{map[string]any{"EventSource": "aws:sns", "EventVersion": "1", "EventSubscriptionArn": "arn:subscription", "Sns": notification}}}
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	kinesis := map[string]any{"Records": []any{map[string]any{"eventSource": "aws:kinesis", "eventVersion": "1", "eventID": "1", "eventName": "aws:kinesis:record", "awsRegion": "ap-east-1", "invokeIdentityArn": "arn:role", "eventSourceARN": "arn:stream", "kinesis": map[string]any{"kinesisSchemaVersion": "1", "partitionKey": "key", "sequenceNumber": "90071992547409930001", "approximateArrivalTimestamp": 1, "data": encoded}}}}
	firehose := map[string]any{"invocationId": "1", "deliveryStreamArn": "arn:delivery", "region": "ap-east-1", "records": []any{map[string]any{"recordId": "1", "approximateArrivalTimestamp": 1, "data": encoded}}}
	logs := map[string]any{"messageType": "DATA_MESSAGE", "owner": "owner", "logGroup": "group", "logStream": "stream", "subscriptionFilters": []any{}, "logEvents": []any{map[string]any{"id": "1", "timestamp": 1, "message": body}}}
	raw, err = json.Marshal(logs)
	if err != nil {
		return nil, err
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err = writer.Write(raw); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	cloudwatch := map[string]any{"awslogs": map[string]any{"data": base64.StdEncoding.EncodeToString(compressed.Bytes())}}
	ddb := map[string]any{"Records": []any{map[string]any{"eventID": "1", "eventName": "INSERT", "eventVersion": "1", "eventSource": "aws:dynamodb", "awsRegion": "ap-east-1", "eventSourceARN": "arn:stream", "dynamodb": map[string]any{"Keys": map[string]any{"id": map[string]any{"S": "stream"}}, "NewImage": map[string]any{"id": map[string]any{"S": "stream"}, "amount": map[string]any{"N": "1"}}, "SequenceNumber": "1", "SizeBytes": 1, "StreamViewType": "NEW_IMAGE"}}}}
	cases := []struct {
		name   string
		input  any
		schema parser.Schema[any]
	}{
		{"sns", sns, parser.Any(envelopes.SNS(parser.JSONStringified(orderInputSchema)))},
		{"snssqs", events.SQSEvent{Records: []events.SQSMessage{record}}, parser.Any(envelopes.SNSSQS(parser.JSONStringified(orderInputSchema)))},
		{"kinesis", kinesis, parser.Any(envelopes.Kinesis(orderInputSchema))},
		{"firehose", firehose, parser.Any(envelopes.KinesisFirehose(parser.JSONStringified(orderInputSchema)))},
		{"cloudwatch", cloudwatch, parser.Any(envelopes.CloudWatch(parser.JSONStringified(orderInputSchema)))},
		{"dynamodb", ddb, parser.Any(envelopes.DynamoDBStream(orderInputSchema))},
	}
	result := map[string]any{}
	for _, item := range cases {
		value, err := parser.Parse(ctx, item.input, item.schema)
		if err != nil {
			return nil, err
		}
		result[item.name] = value
	}
	large, err := parser.Parse(ctx, map[string]any{"value": map[string]any{"N": "9007199254740993"}}, parser.DynamoDBMarshalled(parser.Unknown()))
	if err != nil {
		return nil, err
	}
	result["large_integer"] = large.(map[string]any)["value"].(*big.Int).String()
	return result, nil
}
