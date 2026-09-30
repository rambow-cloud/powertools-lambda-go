package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func parserIdentityProbe(ctx context.Context) (map[string]any, error) {
	result := map[string]any{}
	resolver := json.RawMessage(`{"arguments":{"id":"resolver","amount":1},"source":null,"identity":{"resolverContext":{"role":"reader"}},"request":{"domainName":null,"headers":{}},"info":{"selectionSetList":["id"],"selectionSetGraphQL":"{id}","parentTypeName":"Query","fieldName":"order","variables":{}},"prev":null,"stash":{}}`)
	type resolverInput struct {
		Arguments order `json:"arguments"`
	}
	resolverSchema := parser.Typed[resolverInput](schemas.AppSyncResolverSchema.Extend(parser.Field{Name: "arguments", Schema: parser.Any(orderInputSchema)}))
	handler := parser.WrapHandler[json.RawMessage](resolverSchema, func(ctx context.Context, input resolverInput) (order, error) { return input.Arguments, ctx.Err() })
	value, err := handler(ctx, resolver)
	if err != nil {
		return nil, err
	}
	result["resolver"] = value
	batch, err := parser.Parse(ctx, []json.RawMessage{resolver, resolver}, schemas.AppSyncBatchResolverSchema)
	if err != nil {
		return nil, err
	}
	result["batch_count"] = len(batch.([]any))
	info := map[string]any{"channel": map[string]any{"path": "/orders/new", "segments": []string{"orders", "new"}}, "channelNamespace": map[string]any{"name": "orders"}, "operation": "PUBLISH"}
	event := map[string]any{"identity": map[string]any{"handlerContext": map[string]any{"role": "reader"}}, "result": nil, "request": map[string]any{"domainName": nil}, "info": info, "error": nil, "prev": nil, "stash": map[string]any{"stripped": true}, "outErrors": []any{}, "events": []any{map[string]any{"payload": map[string]any{"id": "order"}, "id": "1"}}}
	result["publish"], err = parser.Parse(ctx, event, schemas.AppSyncEventsPublishSchema)
	if err != nil {
		return nil, err
	}
	info["operation"] = "SUBSCRIBE"
	event["events"] = nil
	result["subscribe"], err = parser.Parse(ctx, event, schemas.AppSyncEventsSubscribeSchema)
	if err != nil {
		return nil, err
	}
	signup := json.RawMessage(`{"version":"1","triggerSource":"PreSignUp_SignUp","region":"ap-east-1","userPoolId":"pool","userName":"user","callerContext":{"awsSdkVersion":"version","clientId":"client"},"request":{"userAttributes":{"email":"synthetic@example.test"},"validationData":null},"response":{"autoConfirmUser":false,"autoVerifyEmail":false,"autoVerifyPhone":false}}`)
	calls := 0
	signupHandler := parser.WrapHandler[json.RawMessage](parser.Typed[events.CognitoEventUserPoolsPreSignup](schemas.PreSignupTriggerSchema), func(ctx context.Context, event events.CognitoEventUserPoolsPreSignup) (events.CognitoEventUserPoolsPreSignup, error) {
		calls++
		event.Response.AutoConfirmUser = true
		return event, ctx.Err()
	})
	signedUp, err := signupHandler(ctx, signup)
	if err != nil {
		return nil, err
	}
	result["signup"] = signedUp
	var invalidEvent map[string]any
	if err := json.Unmarshal(signup, &invalidEvent); err != nil {
		return nil, err
	}
	invalidEvent["response"].(map[string]any)["autoConfirmUser"] = true
	invalid, err := json.Marshal(invalidEvent)
	if err != nil {
		return nil, err
	}
	_, err = signupHandler(ctx, invalid)
	var failure *parser.ParseError
	if !errors.As(err, &failure) {
		return nil, errors.New("invalid signup response flags passed input validation")
	}
	result["signup_issues"] = failure.Issues
	result["signup_calls"] = calls
	token := json.RawMessage(`{"version":"3","triggerSource":"TokenGeneration_Authentication","region":"ap-east-1","userPoolId":"pool","callerContext":{"awsSdkVersion":"version","clientId":"client"},"request":{"userAttributes":{},"groupConfiguration":{"groupsToOverride":[],"iamRolesToOverride":[],"preferredRole":null},"scopes":["read"]},"response":{"stripped":true}}`)
	result["token_v1"], err = parser.Parse(ctx, token, schemas.PreTokenGenerationTriggerSchemaV1)
	if err != nil {
		return nil, err
	}
	result["token_v3"], err = parser.Parse(ctx, token, schemas.PreTokenGenerationTriggerSchemaV2AndV3)
	if err != nil {
		return nil, err
	}
	challenge := json.RawMessage(`{"version":"1","triggerSource":"DefineAuthChallenge_Authentication","region":"ap-east-1","userPoolId":"pool","callerContext":{"awsSdkVersion":"version","clientId":"client"},"request":{"userAttributes":{},"session":[]},"response":{}}`)
	safe, err := parser.SafeParse(ctx, challenge, schemas.DefineAuthChallengeTriggerSchema)
	if err != nil {
		return nil, err
	}
	if safe.Error != nil {
		result["challenge_issues"] = safe.Error.Issues
	}
	return result, nil
}
