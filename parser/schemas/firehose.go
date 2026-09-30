package schemas

import (
	"context"
	"encoding/json"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

var kinesisRecordMetadata = parser.Object(field("shardId", parser.String()), field("partitionKey", parser.String()), field("approximateArrivalTimestamp", positiveNumber), field("sequenceNumber", parser.String()), field("subsequenceNumber", parser.Number()))
var firehoseRecordBase = parser.Object(field("recordId", parser.String()), field("approximateArrivalTimestamp", positiveNumber), nullish("kinesisRecordMetadata", kinesisRecordMetadata))
var firehoseBase = parser.Object(field("invocationId", parser.String()), field("deliveryStreamArn", parser.String()), field("region", parser.String()), optional("sourceKinesisStreamArn", parser.String()))
var KinesisFirehoseRecordSchema = firehoseRecordBase.Extend(field("data", base64Text))

var firehoseSqsData = parser.Pipe(parser.String(), parser.SchemaFunc[any](func(ctx context.Context, input any) (any, []parser.Issue, error) {
	var value any
	err := json.Unmarshal(commons.DecodeBase64Buffer(input.(string)), &value)
	if err == nil {
		parsed, issues, validationErr := SqsRecordSchema.Validate(ctx, value)
		if validationErr != nil {
			return nil, nil, validationErr
		}
		if issues == nil {
			return parsed, nil, nil
		}
	}
	return nil, []parser.Issue{{Code: "custom", Message: "Failed to parse SQS record"}}, nil
}))
var KinesisFirehoseSqsRecordSchema = firehoseRecordBase.Extend(field("data", firehoseSqsData))
var KinesisFirehoseSchema = firehoseBase.Extend(field("records", array(KinesisFirehoseRecordSchema, 1)))
var KinesisFirehoseSqsSchema = firehoseBase.Extend(field("records", array(KinesisFirehoseSqsRecordSchema, 1)))
