package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var cognitoUserAttributes = field("userAttributes", parser.Dictionary(parser.String()))
var cognitoClientMetadata = optional("clientMetadata", parser.Dictionary(parser.String()))
var cognitoUserNotFound = optional("userNotFound", parser.Boolean())

var CognitoTriggerBaseSchema = parser.Object(
	field("version", parser.String()), field("triggerSource", parser.String()), field("region", parser.String()),
	field("userPoolId", parser.String()), optional("userName", parser.String()),
	field("callerContext", parser.Object(field("awsSdkVersion", parser.String()), field("clientId", parser.String()))),
	field("request", parser.Object()), field("response", parser.Object()),
)
var PreSignupTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("triggerSource", parser.Literal("PreSignUp_SignUp")),
	field("request", parser.Object(cognitoUserAttributes, field("validationData", parser.Nullable(parser.Dictionary(parser.String()))), cognitoClientMetadata, cognitoUserNotFound)),
	field("response", parser.Object(field("autoConfirmUser", parser.Literal(false)), field("autoVerifyEmail", parser.Literal(false)), field("autoVerifyPhone", parser.Literal(false)))),
)
var PostConfirmationTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("triggerSource", parser.Literal("PostConfirmation_ConfirmSignUp")),
	field("request", parser.Object(cognitoUserAttributes, cognitoClientMetadata)), field("response", parser.Object()),
)
var PreAuthenticationTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("triggerSource", parser.Literal("PreAuthentication_Authentication")),
	field("request", parser.Object(cognitoUserAttributes, nullish("validationData", parser.Dictionary(parser.String())), cognitoUserNotFound)),
	field("response", parser.Object()),
)
var PostAuthenticationTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("triggerSource", parser.Literal("PostAuthentication_Authentication")),
	field("request", parser.Object(cognitoUserAttributes, optional("newDeviceUsed", parser.Boolean()), cognitoClientMetadata)),
)
var PreTokenGenerationTriggerGroupConfigurationSchema = parser.Object(
	field("groupsToOverride", array(parser.String())), field("iamRolesToOverride", array(parser.String())), field("preferredRole", parser.Nullable(parser.String())),
)
var PreTokenGenerationTriggerRequestSchema = parser.Object(cognitoUserAttributes, field("groupConfiguration", PreTokenGenerationTriggerGroupConfigurationSchema), cognitoClientMetadata)
var PreTokenGenerationTriggerSchemaV1 = CognitoTriggerBaseSchema.Extend(field("request", PreTokenGenerationTriggerRequestSchema))
var PreTokenGenerationTriggerSchemaV2AndV3 = CognitoTriggerBaseSchema.Extend(field("request", PreTokenGenerationTriggerRequestSchema.Extend(optional("scopes", array(parser.String())))))
var MigrateUserTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("userName", parser.String()),
	field("request", parser.Object(field("password", parser.String()), optional("validationData", parser.Dictionary(parser.String())), cognitoClientMetadata)),
	field("response", parser.Object(
		field("userAttributes", parser.Nullable(parser.Dictionary(parser.String()))), field("finalUserStatus", parser.Nullable(parser.String())),
		field("messageAction", parser.Nullable(parser.String())), field("desiredDeliveryMediums", parser.Nullable(array(parser.String()))),
		field("forceAliasCreation", parser.Nullable(parser.Boolean())), field("enableSMSMFA", parser.Nullable(parser.Boolean())),
	)),
)
var CustomMessageTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("request", parser.Object(cognitoUserAttributes, field("codeParameter", parser.String()), field("linkParameter", parser.Nullable(parser.String())), field("usernameParameter", parser.Nullable(parser.String())), cognitoClientMetadata)),
	field("response", parser.Object(field("smsMessage", parser.Nullable(parser.String())), field("emailMessage", parser.Nullable(parser.String())), field("emailSubject", parser.Nullable(parser.String())))),
)
var CustomEmailSenderTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("triggerSource", parser.Literal("CustomEmailSender_SignUp")),
	field("request", parser.Object(field("type", parser.Literal("customEmailSenderRequestV1")), field("code", parser.String()), cognitoClientMetadata, cognitoUserAttributes)),
)
var CustomSMSSenderTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("triggerSource", parser.Literal("CustomSMSSender_SignUp")),
	field("request", parser.Object(field("type", parser.Literal("customSMSSenderRequestV1")), field("code", parser.String()), cognitoClientMetadata, cognitoUserAttributes)),
)
var ChallengeResultSchema = parser.Object(
	field("challengeName", parser.Union(
		parser.Literal("CUSTOM_CHALLENGE"), parser.Literal("SRP_A"), parser.Literal("PASSWORD_VERIFIER"), parser.Literal("SMS_MFA"), parser.Literal("EMAIL_OTP"),
		parser.Literal("SOFTWARE_TOKEN_MFA"), parser.Literal("DEVICE_SRP_AUTH"), parser.Literal("DEVICE_PASSWORD_VERIFIER"), parser.Literal("ADMIN_NO_SRP_AUTH"),
	)), field("challengeResult", parser.Boolean()), optional("challengeMetadata", parser.String()),
)
var cognitoSession = field("session", array(ChallengeResultSchema, 1))
var DefineAuthChallengeTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("triggerSource", parser.Literal("DefineAuthChallenge_Authentication")),
	field("request", parser.Object(cognitoUserAttributes, cognitoSession, cognitoClientMetadata, cognitoUserNotFound)),
	field("response", parser.Object(nullish("challengeName", parser.String()), nullish("issueTokens", parser.Boolean()), nullish("failAuthentication", parser.Boolean()))),
)
var CreateAuthChallengeTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("triggerSource", parser.Literal("CreateAuthChallenge_Authentication")),
	field("request", parser.Object(cognitoUserAttributes, field("challengeName", parser.String()), cognitoSession, cognitoClientMetadata, cognitoUserNotFound)),
	field("response", parser.Object(nullish("publicChallengeParameters", parser.Dictionary(parser.String())), nullish("privateChallengeParameters", parser.Dictionary(parser.String())), nullish("challengeMetadata", parser.String()))),
)
var VerifyAuthChallengeTriggerSchema = CognitoTriggerBaseSchema.Extend(
	field("triggerSource", parser.Literal("VerifyAuthChallengeResponse_Authentication")),
	field("request", parser.Object(cognitoUserAttributes, field("privateChallengeParameters", parser.Dictionary(parser.String())), field("challengeAnswer", parser.String()), cognitoClientMetadata, cognitoUserNotFound)),
	field("response", parser.Object(field("answerCorrect", parser.Boolean()))),
)
