package main

import (
	"context"
	"errors"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/bedrock"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

func bedrockProbe(ctx context.Context, log *logger.Logger, tr *tracer.Tracer) (map[string]any, error) {
	app := bedrock.New(bedrock.Options{Diagnostic: func(context.Context, string, string, error) {}})
	schema := parser.Object(parser.Field{Name: "count", Schema: parser.Number()}, parser.Field{Name: "flag", Schema: parser.Boolean()}, parser.Field{Name: "invalid", Schema: parser.String()}, parser.Field{Name: "list", Schema: parser.String()})
	app.Tool(func(ctx context.Context, parameters *bedrock.Parameters, event bedrock.Event) (any, error) {
		parsed, err := parser.Parse(ctx, parameters.Values(), schema)
		if err != nil {
			return nil, err
		}
		invocation, _ := lambdacontext.FromContext(ctx)
		return map[string]any{"parameters": parsed, "request_id": invocation.AwsRequestID, "trace_id": tr.TraceID(ctx), "correlation_id": log.WithContext(ctx).GetCorrelationID()}, nil
	}, bedrock.Configuration{Name: "order"})
	app.Tool(func(context.Context, *bedrock.Parameters, bedrock.Event) (any, error) {
		response := bedrock.NewFunctionResponse("retry")
		response.ResponseState = bedrock.Reprompt
		return response, nil
	}, bedrock.Configuration{Name: "explicit"})
	app.Tool(func(context.Context, *bedrock.Parameters, bedrock.Event) (any, error) {
		return nil, &bedrock.NamedError{Name: "TypeError", Message: "failed"}
	}, bedrock.Configuration{Name: "error"})
	app.Tool(func(context.Context, *bedrock.Parameters, bedrock.Event) (any, error) { panic(42) }, bedrock.Configuration{Name: "panic"})
	app.Tool(func(context.Context, *bedrock.Parameters, bedrock.Event) (any, error) { return nil, nil }, bedrock.Configuration{Name: "empty"})
	app.Tool(func(context.Context, *bedrock.Parameters, bedrock.Event) (any, error) { return "hello", nil }, bedrock.Configuration{Name: "string"})
	input := bedrock.Event{"messageVersion": "1.0", "actionGroup": "orders", "function": "order", "agent": map[string]any{"name": "agent", "id": "id", "alias": "alias", "version": "1"}, "inputText": "order", "sessionId": "session", "sessionAttributes": map[string]any{"saved": "session"}, "promptSessionAttributes": map[string]any{"saved": "prompt"}, "knowledgeBasesConfiguration": map[string]any{"id": "kb"}, "parameters": []any{
		map[string]any{"name": "count", "type": "integer", "value": "3.5"}, map[string]any{"name": "flag", "type": "boolean", "value": "true"}, map[string]any{"name": "invalid", "type": "number", "value": "oops"}, map[string]any{"name": "list", "type": "array", "value": "[1,2]"},
	}}
	result := map[string]any{}
	for _, name := range []string{"order", "explicit", "missing", "error", "panic", "empty", "string"} {
		input["function"] = name
		value, err := app.Resolve(ctx, input)
		if err != nil {
			return nil, err
		}
		result[name] = value
	}
	_, err := app.Resolve(ctx, nil)
	var invalid *bedrock.Error
	result["invalid"] = errors.As(err, &invalid)
	return result, nil
}
