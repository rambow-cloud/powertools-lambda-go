package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var APIGatewayRequestAuthorizerV2Schema = parser.Object(
	optional("jwt", parser.Object(field("claims", parser.Dictionary(parser.Unknown())), field("scopes", parser.Nullable(APIGatewayStringArray)))),
	optional("iam", parser.Object(optional("accessKey", parser.String()), optional("accountId", parser.String()), optional("callerId", parser.String()), nullish("principalOrgId", parser.String()), optional("userArn", parser.String()), optional("userId", parser.String()),
		nullish("cognitoIdentity", parser.Object(field("amr", APIGatewayStringArray), field("identityId", parser.String()), field("identityPoolId", parser.String()))))),
	nullish("lambda", parser.Dictionary(parser.Unknown())),
)
var APIGatewayRequestContextV2Schema = parser.Object(
	field("accountId", parser.String()), field("apiId", parser.String()), optional("authorizer", APIGatewayRequestAuthorizerV2Schema),
	nullish("authentication", parser.Object(optional("clientCert", APIGatewayCert))), field("domainName", parser.String()), field("domainPrefix", parser.String()),
	field("http", parser.Object(field("method", APIGatewayHttpMethod), field("path", parser.String()), field("protocol", parser.String()), field("sourceIp", gatewaySourceIP), field("userAgent", parser.String()))),
	field("requestId", parser.String()), field("routeKey", parser.String()), field("stage", parser.String()), field("time", parser.String()), field("timeEpoch", parser.Number()),
)
var APIGatewayProxyEventV2Schema = parser.Object(
	field("version", parser.String()), field("routeKey", parser.String()), field("rawPath", parser.String()), field("rawQueryString", parser.String()), optional("cookies", APIGatewayStringArray),
	field("headers", APIGatewayRecord), optional("queryStringParameters", APIGatewayRecord), field("requestContext", APIGatewayRequestContextV2Schema), optional("body", parser.String()),
	nullish("pathParameters", APIGatewayRecord), field("isBase64Encoded", parser.Boolean()), nullish("stageVariables", APIGatewayRecord),
)
var APIGatewayRequestAuthorizerEventV2Schema = parser.Object(
	field("version", parser.Literal("2.0")), field("type", parser.Literal("REQUEST")), field("routeArn", parser.String()), nullish("identitySource", APIGatewayStringArray),
	field("routeKey", parser.String()), field("rawPath", parser.String()), field("rawQueryString", parser.String()), optional("cookies", APIGatewayStringArray),
	optional("headers", APIGatewayRecord), optional("queryStringParameters", APIGatewayRecord), field("requestContext", APIGatewayRequestContextV2Schema),
	nullish("pathParameters", APIGatewayRecord), nullish("stageVariables", APIGatewayRecord),
)

// Lambda Function URLs use the reference HTTP API v2 event contract.
var LambdaFunctionUrlSchema = APIGatewayProxyEventV2Schema.Extend()
