package parser_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

// Only named, single-field mutations in the pinned identity corpus are corrected.
func correctedCognitoData(name, schema string, input, baseline any) (any, bool) {
	var path []string
	var value any
	absent := false
	switch {
	case strings.HasSuffix(name, "-callerContext.clientId-null"):
		path = []string{"callerContext", "clientId"}
	case strings.HasSuffix(name, "-request.clientMetadata-null"):
		path = []string{"request", "clientMetadata"}
	case name == "PreTokenGenerationTriggerRequestSchema-clientMetadata-null":
		path = []string{"clientMetadata"}
	case (schema == "CustomEmailSenderTriggerSchema" || schema == "CustomSMSSenderTriggerSchema") && strings.HasSuffix(name, "-response-omit"):
		path, absent = []string{"response"}, true
	case (schema == "CustomEmailSenderTriggerSchema" || schema == "CustomSMSSenderTriggerSchema") && strings.HasSuffix(name, "-response-null"):
		path = []string{"response"}
	case name == "VerifyAuthChallengeTriggerSchema-response.answerCorrect-null":
		path = []string{"response", "answerCorrect"}
	case (schema == "DefineAuthChallengeTriggerSchema" || schema == "CreateAuthChallengeTriggerSchema") && (name == schema+"-empty-session" || name == schema+"-request.session.0-omit"):
		path, value = []string{"request", "session"}, []any{}
	case name == schema+"-other-trigger" && len(cognitoSources[schema]) > 0:
		value = input.(map[string]any)["triggerSource"]
		if source, valid := value.(string); !valid || !slices.Contains(cognitoSources[schema], source) {
			return nil, false
		}
		path = []string{"triggerSource"}
	default:
		return nil, false
	}
	data := jsonValue(baseline).(map[string]any)
	parent := data
	for _, key := range path[:len(path)-1] {
		parent = parent[key].(map[string]any)
	}
	if absent {
		delete(parent, path[len(path)-1])
	} else {
		parent[path[len(path)-1]] = value
	}
	return data, true
}

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
