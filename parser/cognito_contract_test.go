package parser_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

// These lists come from the AWS trigger-source tables, independently of schemas.
var cognitoSources = map[string][]string{
	"PreSignupTriggerSchema":         {"PreSignUp_SignUp", "PreSignUp_AdminCreateUser", "PreSignUp_ExternalProvider"},
	"PostConfirmationTriggerSchema":  {"PostConfirmation_ConfirmSignUp", "PostConfirmation_ConfirmForgotPassword"},
	"CustomEmailSenderTriggerSchema": {"CustomEmailSender_SignUp", "CustomEmailSender_Authentication", "CustomEmailSender_ForgotPassword", "CustomEmailSender_ResendCode", "CustomEmailSender_UpdateUserAttribute", "CustomEmailSender_VerifyUserAttribute", "CustomEmailSender_AdminCreateUser", "CustomEmailSender_AccountTakeOverNotification"},
	"CustomSMSSenderTriggerSchema":   {"CustomSMSSender_SignUp", "CustomSMSSender_Authentication", "CustomSMSSender_ForgotPassword", "CustomSMSSender_ResendCode", "CustomSMSSender_UpdateUserAttribute", "CustomSMSSender_VerifyUserAttribute", "CustomSMSSender_AdminCreateUser"},
}

func cognitoEvent(t *testing.T, name string) map[string]any {
	t.Helper()
	input := serviceFixture(t, "identity", name)
	delete(input["response"].(map[string]any), "stripped")
	return input
}

func checkCognitoHandler(t *testing.T, name string, input map[string]any) {
	t.Helper()
	handler := parser.WrapHandler[any, any, any](identitySchemas[name], func(_ context.Context, value any) (any, error) { return value, nil })
	output, err := handler(context.Background(), input)
	if err != nil || !reflect.DeepEqual(jsonValue(output), input) {
		t.Fatalf("Cognito flow lost or rejected before handler: %v / %v", output, err)
	}
}

func TestCognitoObservedFlows(t *testing.T) {
	for _, test := range []struct {
		name, schema string
		change       func(map[string]any)
	}{
		{"admin-create", "PreSignupTriggerSchema", func(e map[string]any) { e["triggerSource"] = "PreSignUp_AdminCreateUser" }},
		{"confirm-forgot", "PostConfirmationTriggerSchema", func(e map[string]any) { e["triggerSource"] = "PostConfirmation_ConfirmForgotPassword" }},
		{"sender-forgot", "CustomEmailSenderTriggerSchema", func(e map[string]any) {
			e["triggerSource"] = "CustomEmailSender_ForgotPassword"
			e["request"].(map[string]any)["clientMetadata"] = nil
			delete(e, "response")
		}},
		{"sender-admin", "CustomEmailSenderTriggerSchema", func(e map[string]any) {
			e["triggerSource"] = "CustomEmailSender_AdminCreateUser"
			e["request"].(map[string]any)["clientMetadata"] = nil
			e["callerContext"].(map[string]any)["clientId"] = "CLIENT_ID_NOT_APPLICABLE"
			delete(e, "response")
		}},
		{"sender-signup", "CustomEmailSenderTriggerSchema", func(e map[string]any) {
			e["request"].(map[string]any)["clientMetadata"] = nil
			delete(e, "response")
		}},
		{"define-first", "DefineAuthChallengeTriggerSchema", func(e map[string]any) { e["request"].(map[string]any)["session"] = []any{} }},
		{"create-first", "CreateAuthChallengeTriggerSchema", func(e map[string]any) { e["request"].(map[string]any)["session"] = []any{} }},
		{"verify-unanswered", "VerifyAuthChallengeTriggerSchema", func(e map[string]any) { e["response"].(map[string]any)["answerCorrect"] = nil }},
		{"admin-confirm", "PostConfirmationTriggerSchema", func(e map[string]any) { e["callerContext"].(map[string]any)["clientId"] = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := cognitoEvent(t, test.schema)
			test.change(input)
			checkCognitoHandler(t, test.schema, input)
		})
	}
}

func TestCognitoDocumentedSourcesAndSenderOptionalFields(t *testing.T) {
	for name, sources := range cognitoSources {
		for _, source := range sources {
			t.Run(source, func(t *testing.T) {
				input := cognitoEvent(t, name)
				input["triggerSource"] = source
				if name == "CustomEmailSenderTriggerSchema" || name == "CustomSMSSenderTriggerSchema" {
					delete(input, "response")
					delete(input["request"].(map[string]any), "clientMetadata")
				}
				checkCognitoHandler(t, name, input)
			})
		}
	}
	for _, name := range []string{"CustomEmailSenderTriggerSchema", "CustomSMSSenderTriggerSchema"} {
		input := cognitoEvent(t, name)
		input["response"] = nil
		input["request"].(map[string]any)["clientMetadata"] = nil
		checkCognitoHandler(t, name, input)
	}
}

func TestCognitoInvalidInputsDoNotReachHandler(t *testing.T) {
	for _, test := range []struct {
		schema, parent, field string
		value                 any
		absent                bool
	}{
		{"PostConfirmationTriggerSchema", "callerContext", "clientId", nil, true},
		{"PostConfirmationTriggerSchema", "callerContext", "clientId", false, false},
		{"CustomEmailSenderTriggerSchema", "request", "clientMetadata", false, false},
		{"CustomSMSSenderTriggerSchema", "request", "clientMetadata", map[string]any{"invalid": false}, false},
		{"CustomEmailSenderTriggerSchema", "", "response", false, false},
		{"CustomSMSSenderTriggerSchema", "", "response", "bad", false},
		{"PostConfirmationTriggerSchema", "", "response", nil, true},
		{"DefineAuthChallengeTriggerSchema", "request", "session", nil, true},
		{"CreateAuthChallengeTriggerSchema", "request", "session", nil, false},
		{"DefineAuthChallengeTriggerSchema", "request", "session", []any{map[string]any{"challengeName": "UNKNOWN", "challengeResult": false}}, false},
		{"VerifyAuthChallengeTriggerSchema", "response", "answerCorrect", nil, true},
		{"VerifyAuthChallengeTriggerSchema", "response", "answerCorrect", "false", false},
		{"PreSignupTriggerSchema", "response", "autoConfirmUser", true, false},
		{"PreSignupTriggerSchema", "", "triggerSource", "PostConfirmation_ConfirmSignUp", false},
		{"PostConfirmationTriggerSchema", "", "triggerSource", "PreSignUp_SignUp", false},
		{"CustomEmailSenderTriggerSchema", "", "triggerSource", "CustomSMSSender_SignUp", false},
		{"CustomSMSSenderTriggerSchema", "", "triggerSource", "CustomEmailSender_SignUp", false},
	} {
		input := cognitoEvent(t, test.schema)
		parent := input
		if test.parent != "" {
			parent = input[test.parent].(map[string]any)
		}
		if test.absent {
			delete(parent, test.field)
		} else {
			parent[test.field] = test.value
		}
		calls := 0
		handler := parser.WrapHandler[any, any, any](identitySchemas[test.schema], func(_ context.Context, value any) (any, error) { calls++; return value, nil })
		if _, err := handler(context.Background(), input); err == nil || calls != 0 {
			t.Fatalf("invalid %s.%s reached handler: calls=%d error=%v", test.schema, test.field, calls, err)
		}
	}
}
