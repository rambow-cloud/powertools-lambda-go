package parser_test

import (
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func TestIdentityTypeScriptReference(t *testing.T) {
	runReference(t, "testdata/identity-v2.35.0.json", 1467)
}

var identitySchemas = map[string]parser.Schema[any]{
	"AppSyncIamIdentity":                                schemas.AppSyncIamIdentity,
	"AppSyncCognitoIdentity":                            schemas.AppSyncCognitoIdentity,
	"AppSyncOidcIdentity":                               schemas.AppSyncOidcIdentity,
	"AppSyncLambdaIdentity":                             schemas.AppSyncLambdaIdentity,
	"AppSyncResolverSchema":                             schemas.AppSyncResolverSchema,
	"AppSyncBatchResolverSchema":                        schemas.AppSyncBatchResolverSchema,
	"AppSyncLambdaAuthIdentity":                         schemas.AppSyncLambdaAuthIdentity,
	"AppSyncEventsRequestSchema":                        schemas.AppSyncEventsRequestSchema,
	"AppSyncEventsInfoSchema":                           schemas.AppSyncEventsInfoSchema,
	"AppSyncEventsBaseSchema":                           schemas.AppSyncEventsBaseSchema,
	"AppSyncEventsPublishSchema":                        schemas.AppSyncEventsPublishSchema,
	"AppSyncEventsSubscribeSchema":                      schemas.AppSyncEventsSubscribeSchema,
	"CognitoTriggerBaseSchema":                          schemas.CognitoTriggerBaseSchema,
	"PreSignupTriggerSchema":                            schemas.PreSignupTriggerSchema,
	"PostConfirmationTriggerSchema":                     schemas.PostConfirmationTriggerSchema,
	"PreAuthenticationTriggerSchema":                    schemas.PreAuthenticationTriggerSchema,
	"PostAuthenticationTriggerSchema":                   schemas.PostAuthenticationTriggerSchema,
	"PreTokenGenerationTriggerGroupConfigurationSchema": schemas.PreTokenGenerationTriggerGroupConfigurationSchema,
	"PreTokenGenerationTriggerRequestSchema":            schemas.PreTokenGenerationTriggerRequestSchema,
	"PreTokenGenerationTriggerSchemaV1":                 schemas.PreTokenGenerationTriggerSchemaV1,
	"PreTokenGenerationTriggerSchemaV2AndV3":            schemas.PreTokenGenerationTriggerSchemaV2AndV3,
	"MigrateUserTriggerSchema":                          schemas.MigrateUserTriggerSchema,
	"CustomMessageTriggerSchema":                        schemas.CustomMessageTriggerSchema,
	"CustomEmailSenderTriggerSchema":                    schemas.CustomEmailSenderTriggerSchema,
	"CustomSMSSenderTriggerSchema":                      schemas.CustomSMSSenderTriggerSchema,
	"ChallengeResultSchema":                             schemas.ChallengeResultSchema,
	"DefineAuthChallengeTriggerSchema":                  schemas.DefineAuthChallengeTriggerSchema,
	"CreateAuthChallengeTriggerSchema":                  schemas.CreateAuthChallengeTriggerSchema,
	"VerifyAuthChallengeTriggerSchema":                  schemas.VerifyAuthChallengeTriggerSchema,
}
