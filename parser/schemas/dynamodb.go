package schemas

import (
	"context"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

var DynamoDBStreamChangeRecordBase = parser.Object(
	optional("ApproximateCreationDateTime", parser.Number()), field("Keys", parser.Dictionary(parser.Dictionary(parser.Unknown()))),
	optional("NewImage", parser.Dictionary(parser.Unknown())), optional("OldImage", parser.Dictionary(parser.Unknown())),
	field("SequenceNumber", parser.String()), field("SizeBytes", parser.Number()), field("StreamViewType", parser.Enum("NEW_IMAGE", "OLD_IMAGE", "NEW_AND_OLD_IMAGES", "KEYS_ONLY")),
)
var DynamoDBStreamToKinesisChangeRecord = DynamoDBStreamChangeRecordBase.Omit("SequenceNumber", "StreamViewType")
var unmarshallImages = parser.SchemaFunc[any](func(ctx context.Context, input any) (any, []parser.Issue, error) {
	result := input.(map[string]any)
	for _, name := range []string{"Keys", "NewImage", "OldImage"} {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		image, exists := result[name]
		if !exists {
			continue
		}
		decoded, err := commons.UnmarshallDynamoDB(image.(map[string]any))
		if err != nil {
			return nil, []parser.Issue{{Code: "custom", Message: "Could not unmarshall " + name + " in DynamoDB stream record", Path: []any{name}}}, nil
		}
		result[name] = decoded
	}
	return result, nil, nil
})
var DynamoDBStreamChangeRecord = parser.Pipe[any, any](DynamoDBStreamChangeRecordBase, unmarshallImages)
var UserIdentity = parser.Object(field("type", parser.Enum("Service")), field("principalId", parser.Literal("dynamodb.amazonaws.com")))
var DynamoDBStreamRecord = parser.Object(
	field("eventID", parser.String()), field("eventName", parser.Enum("INSERT", "MODIFY", "REMOVE")), field("eventVersion", parser.String()),
	field("eventSource", parser.Literal("aws:dynamodb")), field("awsRegion", parser.String()), field("eventSourceARN", parser.String()),
	field("dynamodb", DynamoDBStreamChangeRecord), optional("userIdentity", UserIdentity),
)
var DynamoDBStreamToKinesisRecord = DynamoDBStreamRecord.Extend(
	field("recordFormat", parser.Literal("application/json")), field("tableName", parser.String()), nullish("userIdentity", UserIdentity),
	field("dynamodb", parser.Pipe[any, any](DynamoDBStreamToKinesisChangeRecord, unmarshallImages)),
).Omit("eventVersion", "eventSourceARN")
var DynamoDBStreamSchema = parser.Object(append([]parser.Field{field("Records", array(DynamoDBStreamRecord, 1))}, windowFields(parser.String())...)...)
