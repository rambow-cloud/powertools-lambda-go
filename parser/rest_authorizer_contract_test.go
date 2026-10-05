package parser_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func restContext(input map[string]any) map[string]any {
	if context, ok := input["requestContext"].(map[string]any); ok {
		return context
	}
	return input
}

func TestRESTAuthorizerHandlerPreservesContext(t *testing.T) {
	for _, test := range []struct {
		name   string
		schema parser.Schema[any]
	}{
		{"APIGatewayEventRequestContextSchema", schemas.APIGatewayEventRequestContextSchema},
		{"APIGatewayProxyEventSchema", schemas.APIGatewayProxyEventSchema},
		{"APIGatewayRequestAuthorizerEventSchema", schemas.APIGatewayRequestAuthorizerEventSchema},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := serviceFixture(t, "http", test.name)
			authorizer := restContext(input)["authorizer"].(map[string]any)
			authorizer["tenantId"], authorizer["quota"], authorizer["enabled"] = "tenant-123", float64(7), true
			handler := parser.WrapHandler[any, any, any](test.schema, func(_ context.Context, value any) (any, error) { return value, nil })
			output, err := handler(context.Background(), input)
			if err != nil || !reflect.DeepEqual(jsonValue(output), input) {
				t.Fatalf("custom authorizer fields lost before handler: %v / %v", output, err)
			}
		})
	}
}

func TestRESTCognitoAuthorizerRemainsTyped(t *testing.T) {
	for _, metadata := range []bool{false, true} {
		input := serviceFixture(t, "http", "APIGatewayProxyEventSchema")
		authorizer := map[string]any{"claims": map[string]any{"sub": "user"}, "scopes": []any{"read"}, "ignored": true}
		if metadata {
			authorizer["integrationLatency"], authorizer["principalId"] = float64(1), "user"
		}
		restContext(input)["authorizer"] = authorizer
		output, err := parser.Parse(context.Background(), input, schemas.APIGatewayProxyEventSchema)
		want := map[string]any{"claims": map[string]any{"sub": "user"}, "scopes": []any{"read"}}
		if err != nil || !reflect.DeepEqual(jsonValue(restContext(output.(map[string]any))["authorizer"]), want) {
			t.Fatalf("Cognito branch no longer typed: %v / %v", output, err)
		}
		for _, invalid := range []struct {
			field string
			value any
		}{{"claims", nil}, {"claims", "bad"}, {"scopes", false}, {"scopes", []any{1}}} {
			// Reload the baseline so each control changes only its own field.
			ctx := serviceFixture(t, "http", "APIGatewayEventRequestContextSchema")
			bad := map[string]any{"claims": map[string]any{"sub": "user"}, "scopes": []any{"read"}}
			if metadata {
				bad["integrationLatency"], bad["principalId"] = float64(1), "user"
			}
			bad[invalid.field] = invalid.value
			ctx["authorizer"] = bad
			result, err := parser.SafeParse(context.Background(), ctx, schemas.APIGatewayEventRequestContextSchema)
			if err != nil || result.Success || result.Error == nil {
				t.Fatalf("malformed Cognito %s bypassed validation: %+v / %v", invalid.field, result, err)
			}
		}
	}
}

func TestRESTAuthorizerEnvelopeAndOptionalControls(t *testing.T) {
	input := serviceFixture(t, "http", "APIGatewayProxyEventSchema")
	payload := parser.JSONStringified(parser.Unknown())
	value, err := parser.Parse(context.Background(), input, envelopes.APIGateway(payload))
	if err != nil || !reflect.DeepEqual(jsonValue(value), map[string]any{"id": "http", "amount": float64(1)}) {
		t.Fatalf("authorizer context prevented payload delivery: %v / %v", value, err)
	}
	for _, absent := range []bool{true, false} {
		ctx := serviceFixture(t, "http", "APIGatewayEventRequestContextSchema")
		if absent {
			delete(ctx, "authorizer")
		} else {
			ctx["authorizer"] = nil
		}
		if _, err := parser.Parse(context.Background(), ctx, schemas.APIGatewayEventRequestContextSchema); err != nil {
			t.Fatalf("optional authorizer rejected: %v", err)
		}
	}
	for _, field := range []string{"integrationLatency", "principalId"} {
		ctx := serviceFixture(t, "http", "APIGatewayEventRequestContextSchema")
		ctx["authorizer"].(map[string]any)[field] = false
		result, err := parser.SafeParse(context.Background(), ctx, schemas.APIGatewayEventRequestContextSchema)
		if err != nil || result.Success || result.Error == nil {
			t.Fatalf("invalid custom authorizer %s accepted: %+v / %v", field, result, err)
		}
	}
}
