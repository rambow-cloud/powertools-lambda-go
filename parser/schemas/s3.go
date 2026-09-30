package schemas

import (
	"context"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

var s3Identity = parser.Object(field("principalId", parser.String()))
var s3SourceAddress = parser.Union(ipAddress(4), parser.Literal("s3.amazonaws.com"))
var s3Message = parser.Object(field("s3SchemaVersion", parser.String()), field("configurationId", parser.String()),
	field("object", parser.Object(field("key", parser.String()), optional("size", parser.Number()), optional("urlDecodedKey", parser.String()), optional("eTag", parser.String()), optional("sequencer", parser.String()), optional("versionId", parser.String()))),
	field("bucket", parser.Object(field("name", parser.String()), field("ownerIdentity", s3Identity), field("arn", parser.String()))),
)
var s3Record = parser.Object(field("eventVersion", parser.String()), field("eventSource", parser.Literal("aws:s3")), field("awsRegion", parser.String()), field("eventTime", isoDateTime), field("eventName", parser.String()), field("userIdentity", s3Identity),
	field("requestParameters", parser.Object(field("sourceIPAddress", s3SourceAddress))),
	field("responseElements", parser.Object(field("x-amz-request-id", parser.String()), field("x-amz-id-2", parser.String()))), field("s3", s3Message),
	optional("glacierEventData", parser.Object(field("restoreEventData", parser.Object(field("lifecycleRestorationExpiryTime", parser.String()), field("lifecycleRestoreStorageClass", parser.String()))))),
)
var nonnegativeNumber = parser.Pipe(parser.Number(), parser.SchemaFunc[any](func(_ context.Context, input any) (any, []parser.Issue, error) {
	if input.(float64) < 0 {
		return input, []parser.Issue{{Code: "too_small", Message: "Too small: expected number to be >=0", Continuable: true}}, nil
	}
	return input, nil, nil
}))
var s3EventBridgeDetail = parser.Object(field("version", parser.String()), field("bucket", parser.Object(field("name", parser.String()))),
	field("object", parser.Object(field("key", parser.String()), optional("size", nonnegativeNumber), optional("etag", parser.String()), optional("version-id", parser.String()), optional("sequencer", parser.String()))),
	field("request-id", parser.String()), field("requester", parser.String()), optional("source-ip-address", ipAddress(4)), optional("reason", parser.String()), optional("deletion-type", parser.String()), optional("restore-expiry-time", parser.String()), optional("source-storage-class", parser.String()), optional("destination-storage-class", parser.String()), optional("destination-access-tier", parser.String()),
)
var S3EventNotificationEventBridgeSchema = EventBridgeSchema.Extend(field("detail", s3EventBridgeDetail))
var S3Schema = parser.Object(field("Records", array(s3Record, 1)))
var S3SqsEventNotificationSchema = parser.Object(field("Records", array(SqsRecordSchema.Extend(field("body", parser.JSONStringified[any](S3Schema))), 1)))
var s3SessionContext = parser.Object(
	field("sessionIssuer", parser.Object(field("type", parser.String()), optional("userName", parser.String()), field("principalId", parser.String()), field("arn", parser.String()), field("accountId", parser.String()))),
	field("attributes", parser.Object(field("creationDate", parser.String()), field("mfaAuthenticated", parser.Transform(parser.Union(parser.Boolean(), parser.Literal("true"), parser.Literal("false")), func(_ context.Context, value any) (any, error) { return value == true || value == "true", nil })))),
)
var S3ObjectLambdaEventSchema = parser.Object(field("xAmzRequestId", parser.String()),
	field("getObjectContext", parser.Object(field("inputS3Url", parser.String()), field("outputRoute", parser.String()), field("outputToken", parser.String()))),
	field("configuration", parser.Object(field("accessPointArn", parser.String()), field("supportingAccessPointArn", parser.String()), field("payload", parser.Union(parser.String(), parser.Object())))),
	field("userRequest", parser.Object(field("url", parser.String()), field("headers", parser.Dictionary(parser.String())))),
	field("userIdentity", parser.Object(field("type", parser.String()), field("accountId", parser.String()), field("accessKeyId", parser.String()), optional("userName", parser.String()), field("principalId", parser.String()), field("arn", parser.String()), optional("sessionContext", s3SessionContext))), field("protocolVersion", parser.String()),
)
