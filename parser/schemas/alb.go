package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var AlbSchema = parser.Object(
	field("httpMethod", parser.String()), field("path", parser.String()), field("body", parser.String()), field("isBase64Encoded", parser.Boolean()),
	optional("headers", APIGatewayRecord), optional("multiValueHeaders", parser.Dictionary(APIGatewayStringArray)), optional("queryStringParameters", APIGatewayRecord), optional("multiValueQueryStringParameters", parser.Dictionary(APIGatewayStringArray)),
	field("requestContext", parser.Object(field("elb", parser.Object(field("targetGroupArn", parser.String()))))),
)
var AlbMultiValueHeadersSchema = AlbSchema.Extend(field("multiValueHeaders", parser.Dictionary(APIGatewayStringArray)), field("multiValueQueryStringParameters", parser.Dictionary(APIGatewayStringArray)))
