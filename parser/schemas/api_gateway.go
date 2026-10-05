package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var gatewayEventIdentity = parser.Object(
	nullish("accessKey", parser.String()), nullish("accountId", parser.String()), nullish("apiKey", parser.String()), nullish("apiKeyId", parser.String()),
	nullish("caller", parser.String()), nullish("cognitoAuthenticationProvider", parser.String()), nullish("cognitoAuthenticationType", parser.String()),
	nullish("cognitoIdentityId", parser.String()), nullish("cognitoIdentityPoolId", parser.String()), nullish("principalOrgId", parser.String()),
	optional("sourceIp", parser.Union(ipAddress(4), ipAddress(6), parser.Literal("test-invoke-source-ip"))),
	nullish("user", parser.String()), nullish("userAgent", parser.String()), nullish("userArn", parser.String()), nullish("clientCert", APIGatewayCert),
)
var gatewayAuthorizer = parser.Union(
	parser.Refine[any](parser.Object(field("integrationLatency", parser.Number()), field("principalId", parser.String())).WithUnknownFields(parser.PreserveUnknown), func(input any) bool {
		// Claims identify the Cognito branch, even when Lambda metadata is present.
		_, claims := input.(map[string]any)["claims"]
		return !claims
	}, "claims must use the Cognito authorizer schema"),
	parser.Object(field("claims", parser.Dictionary(parser.Unknown())), optional("scopes", APIGatewayStringArray)),
)
var gatewayRequestContext = parser.Object(
	field("accountId", parser.String()), field("apiId", parser.String()), nullish("deploymentId", parser.String()), nullish("authorizer", gatewayAuthorizer),
	field("stage", parser.String()), field("protocol", parser.String()), field("identity", gatewayEventIdentity), field("requestId", parser.String()),
	field("requestTime", parser.String()), field("requestTimeEpoch", parser.Number()), nullish("resourceId", parser.String()), field("resourcePath", parser.String()),
	nullish("domainName", parser.String()), nullish("domainPrefix", parser.String()), nullish("extendedRequestId", parser.String()), field("httpMethod", APIGatewayHttpMethod),
	field("path", parser.String()), nullish("connectedAt", parser.Number()), nullish("connectionId", parser.String()), nullish("eventType", parser.Enum("CONNECT", "MESSAGE", "DISCONNECT")),
	nullish("messageDirection", parser.String()), nullish("messageId", parser.String()), nullish("routeKey", parser.String()), nullish("operationName", parser.String()),
)
var APIGatewayEventRequestContextSchema = parser.Refine[any](gatewayRequestContext, func(input any) bool {
	value := input.(map[string]any)
	message := value["messageId"]
	return message == nil || message == "" || value["eventType"] == "MESSAGE"
}, "messageId is available only when `eventType` is MESSAGE")
var APIGatewayProxyEventSchema = parser.Object(
	field("resource", parser.String()), field("path", parser.String()), field("httpMethod", APIGatewayHttpMethod), nullish("headers", APIGatewayRecord),
	nullish("multiValueHeaders", parser.Dictionary(APIGatewayStringArray)), field("queryStringParameters", parser.Nullable(APIGatewayRecord)),
	field("multiValueQueryStringParameters", parser.Nullable(parser.Dictionary(APIGatewayStringArray))), nullish("pathParameters", APIGatewayRecord), nullish("stageVariables", APIGatewayRecord),
	field("requestContext", APIGatewayEventRequestContextSchema), field("body", parser.Nullable(parser.String())), field("isBase64Encoded", parser.Boolean()),
)
var APIGatewayRequestAuthorizerEventSchema = parser.Object(
	field("type", parser.Literal("REQUEST")), field("methodArn", parser.String()), field("resource", parser.String()), field("path", parser.String()), field("httpMethod", APIGatewayHttpMethod),
	field("headers", APIGatewayRecord), field("multiValueHeaders", parser.Dictionary(APIGatewayStringArray)), field("queryStringParameters", APIGatewayRecord),
	field("multiValueQueryStringParameters", parser.Dictionary(APIGatewayStringArray)), field("pathParameters", APIGatewayRecord), field("stageVariables", APIGatewayRecord),
	field("requestContext", APIGatewayEventRequestContextSchema), optional("domainName", parser.String()), optional("deploymentId", parser.String()), optional("apiId", parser.String()),
)
var APIGatewayTokenAuthorizerEventSchema = parser.Object(field("type", parser.Literal("TOKEN")), field("authorizationToken", parser.String()), field("methodArn", parser.String()))
