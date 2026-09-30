package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var AppSyncIamIdentity = parser.Object(
	field("accountId", parser.String()), field("cognitoIdentityPoolId", parser.Nullable(parser.String())),
	field("cognitoIdentityId", parser.Nullable(parser.String())), field("sourceIp", array(parser.String())),
	field("username", parser.String()), field("userArn", parser.String()),
	field("cognitoIdentityAuthType", parser.Nullable(parser.String())), field("cognitoIdentityAuthProvider", parser.Nullable(parser.String())),
)
var AppSyncCognitoIdentity = parser.Object(
	field("sub", parser.String()), field("issuer", parser.String()), field("username", parser.String()),
	field("claims", parser.Dictionary(parser.Unknown())), field("sourceIp", array(ipAddress(4))),
	field("defaultAuthStrategy", parser.Nullable(parser.String())), field("groups", parser.Nullable(array(parser.String()))),
)
var AppSyncOidcIdentity = parser.Object(field("claims", parser.Unknown()), field("issuer", parser.String()), field("sub", parser.String()))
var AppSyncLambdaIdentity = parser.Object(field("resolverContext", parser.Unknown()))
var AppSyncResolverSchema = parser.Object(
	field("arguments", parser.Dictionary(parser.Unknown())),
	optional("identity", parser.Union(AppSyncCognitoIdentity, AppSyncIamIdentity, AppSyncOidcIdentity, AppSyncLambdaIdentity)),
	field("source", parser.Nullable(parser.Dictionary(parser.Unknown()))),
	field("request", parser.Object(field("domainName", parser.Nullable(parser.String())), field("headers", parser.Dictionary(parser.String())))),
	field("info", parser.Object(
		field("selectionSetList", array(parser.String())), field("selectionSetGraphQL", parser.String()),
		field("parentTypeName", parser.String()), field("fieldName", parser.String()), field("variables", parser.Dictionary(parser.Unknown())),
	)),
	field("prev", parser.Nullable(parser.Object(field("result", parser.Dictionary(parser.Unknown()))))),
	field("stash", parser.Dictionary(parser.Unknown())),
)
var AppSyncBatchResolverSchema = array(AppSyncResolverSchema)

var AppSyncLambdaAuthIdentity = parser.Object(field("handlerContext", parser.Dictionary(parser.Unknown())))
var AppSyncEventsRequestSchema = parser.Object(optional("headers", parser.Dictionary(parser.String())), field("domainName", parser.Nullable(parser.String())))
var AppSyncEventsInfoSchema = parser.Object(
	field("channel", parser.Object(field("path", parser.String()), field("segments", array(parser.String())))),
	field("channelNamespace", parser.Object(field("name", parser.String()))),
	field("operation", parser.Union(parser.Literal("PUBLISH"), parser.Literal("SUBSCRIBE"))),
)
var AppSyncEventsBaseSchema = parser.Object(
	field("identity", parser.Union(parser.Null(), AppSyncCognitoIdentity, AppSyncIamIdentity, AppSyncLambdaAuthIdentity, AppSyncOidcIdentity)),
	field("result", parser.Null()), field("request", AppSyncEventsRequestSchema), field("info", AppSyncEventsInfoSchema),
	field("error", parser.Null()), field("prev", parser.Null()), field("stash", parser.Object()),
	field("outErrors", array(parser.Unknown())), field("events", parser.Null()),
)
var AppSyncEventsPublishSchema = AppSyncEventsBaseSchema.Extend(
	field("info", AppSyncEventsInfoSchema.Extend(field("operation", parser.Literal("PUBLISH")))),
	field("events", array(parser.Object(field("payload", parser.Dictionary(parser.Unknown())), field("id", parser.String())), 1)),
)
var AppSyncEventsSubscribeSchema = AppSyncEventsBaseSchema.Extend(field("info", AppSyncEventsInfoSchema.Extend(field("operation", parser.Literal("SUBSCRIBE")))), field("events", parser.Null()))
