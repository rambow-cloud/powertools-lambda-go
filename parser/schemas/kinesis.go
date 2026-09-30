package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var KinesisDataStreamRecordPayload = parser.Object(
	field("kinesisSchemaVersion", parser.String()), field("partitionKey", parser.String()), field("sequenceNumber", parser.String()),
	field("approximateArrivalTimestamp", parser.Number()), field("data", parser.Base64Encoded(parser.Unknown())),
)
var KinesisDataStreamRecord = parser.Object(
	field("eventSource", parser.Literal("aws:kinesis")), field("eventVersion", parser.String()), field("eventID", parser.String()),
	field("eventName", parser.Literal("aws:kinesis:record")), field("awsRegion", parser.String()), field("invokeIdentityArn", parser.String()),
	field("eventSourceARN", parser.String()), field("kinesis", KinesisDataStreamRecordPayload),
)
var KinesisDynamoDBStreamSchema = parser.Object(field("Records", array(KinesisDataStreamRecord.Extend(
	field("kinesis", KinesisDataStreamRecordPayload.Extend(field("data", parser.Pipe[any, any](parser.Base64Encoded(parser.Unknown()), DynamoDBStreamToKinesisRecord)))),
))))
var KinesisDataStreamSchema = parser.Object(append([]parser.Field{field("Records", array(KinesisDataStreamRecord, 1))}, windowFields(parser.Unknown())...)...)
