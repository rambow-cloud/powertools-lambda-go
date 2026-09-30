package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var APIGatewayProxyWebsocketEventSchema = parser.Object(
	field("type", parser.String()), field("methodArn", parser.String()), nullish("headers", APIGatewayRecord), field("multiValueHeaders", parser.Dictionary(APIGatewayStringArray)),
	field("queryStringParameters", parser.Nullable(APIGatewayRecord)), field("multiValueQueryStringParameters", parser.Nullable(parser.Dictionary(APIGatewayStringArray))), nullish("stageVariables", APIGatewayRecord),
	field("requestContext", parser.Object(
		field("routeKey", parser.String()), field("eventType", parser.Enum("CONNECT", "DISCONNECT", "MESSAGE")), field("extendedRequestId", parser.String()), field("requestTime", parser.String()),
		field("messageDirection", parser.Enum("IN", "OUT")), field("stage", parser.String()), field("connectedAt", parser.Number()), field("requestTimeEpoch", parser.Number()),
		field("identity", parser.Object(field("sourceIp", parser.String()), optional("userAgent", parser.String()))), field("requestId", parser.String()), field("domainName", parser.String()), field("connectionId", parser.String()), field("apiId", parser.String()),
	)), field("isBase64Encoded", parser.Boolean()), nullish("body", parser.String()),
)
