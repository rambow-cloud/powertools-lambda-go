package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var snsMsgAttribute = parser.Object(field("Type", parser.String()), field("Value", parser.String()))

var SnsNotificationSchema = parser.Object(
	nullish("Subject", parser.String()), field("TopicArn", parser.String()), field("UnsubscribeUrl", absoluteURL),
	optional("UnsubscribeURL", absoluteURL), optional("SigningCertUrl", absoluteURL), optional("SigningCertURL", absoluteURL),
	field("Type", parser.Literal("Notification")), optional("MessageAttributes", parser.Dictionary(snsMsgAttribute)),
	field("Message", parser.String()), field("MessageId", parser.String()), optional("Signature", parser.String()),
	optional("SignatureVersion", parser.String()), field("Timestamp", isoDateTime),
)

var SnsSqsNotificationSchema = SnsNotificationSchema.Extend(optional("UnsubscribeURL", parser.String()), optional("SigningCertURL", absoluteURL)).Omit("UnsubscribeUrl", "SigningCertUrl")
var SnsRecordSchema = parser.Object(field("EventSource", parser.Literal("aws:sns")), field("EventVersion", parser.String()), field("EventSubscriptionArn", parser.String()), field("Sns", SnsNotificationSchema))
var SnsSchema = parser.Object(field("Records", array(SnsRecordSchema, 1)))
