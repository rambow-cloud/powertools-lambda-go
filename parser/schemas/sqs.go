package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var SqsMsgAttributeDataTypeSchema = parser.Union(parser.Literal("String"), parser.Literal("Number"), parser.Literal("Binary"), parser.String())
var SqsMsgAttributeSchema = parser.Object(
	parser.Field{Name: "stringValue", Schema: parser.Nullable(parser.String()), Optional: true},
	parser.Field{Name: "binaryValue", Schema: parser.Nullable(parser.String()), Optional: true},
	parser.Field{Name: "stringListValues", Schema: parser.Any(parser.Array(parser.String())), Optional: true},
	parser.Field{Name: "binaryListValues", Schema: parser.Any(parser.Array(parser.String())), Optional: true},
	parser.Field{Name: "dataType", Schema: SqsMsgAttributeDataTypeSchema},
)
var SqsAttributesSchema = parser.Object(
	parser.Field{Name: "ApproximateReceiveCount", Schema: parser.String()},
	parser.Field{Name: "ApproximateFirstReceiveTimestamp", Schema: parser.String()},
	parser.Field{Name: "MessageDeduplicationId", Schema: parser.String(), Optional: true},
	parser.Field{Name: "MessageGroupId", Schema: parser.String(), Optional: true},
	parser.Field{Name: "SenderId", Schema: parser.String()},
	parser.Field{Name: "SentTimestamp", Schema: parser.String()},
	parser.Field{Name: "SequenceNumber", Schema: parser.String(), Optional: true},
	parser.Field{Name: "AWSTraceHeader", Schema: parser.String(), Optional: true},
	parser.Field{Name: "DeadLetterQueueSourceArn", Schema: parser.String(), Optional: true},
)
var SqsRecordSchema = parser.Object(
	parser.Field{Name: "messageId", Schema: parser.String()},
	parser.Field{Name: "receiptHandle", Schema: parser.String()},
	parser.Field{Name: "body", Schema: parser.String()},
	parser.Field{Name: "attributes", Schema: SqsAttributesSchema},
	parser.Field{Name: "messageAttributes", Schema: parser.Dictionary(SqsMsgAttributeSchema)},
	parser.Field{Name: "md5OfBody", Schema: parser.String()},
	parser.Field{Name: "md5OfMessageAttributes", Schema: parser.Nullable(parser.String()), Optional: true},
	parser.Field{Name: "eventSource", Schema: parser.Literal("aws:sqs")},
	parser.Field{Name: "eventSourceARN", Schema: parser.String()},
	parser.Field{Name: "awsRegion", Schema: parser.String()},
)
var SqsSchema = parser.Object(parser.Field{Name: "Records", Schema: parser.Any(parser.Array(SqsRecordSchema, 1))})
