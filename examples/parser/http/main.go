// Command http validates HTTP API v2 bodies before executing a typed Lambda handler.
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
)

type order struct {
	ID     string  `json:"id"`
	Amount float64 `json:"amount"`
}

func main() {
	payload := parser.Typed[order](parser.Object(parser.Field{Name: "id", Schema: parser.String()}, parser.Field{Name: "amount", Schema: parser.Refine(parser.Number(), func(value any) bool { return value.(float64) >= 0 }, "amount must be non-negative")}))
	handler := parser.WrapSafeHandler[map[string]any](envelopes.APIGatewayV2(parser.JSONStringified(payload)), func(ctx context.Context, result parser.Result[order]) (events.APIGatewayV2HTTPResponse, error) {
		if !result.Success {
			return events.APIGatewayV2HTTPResponse{StatusCode: 400, Body: "Invalid request"}, nil
		}
		if err := ctx.Err(); err != nil {
			return events.APIGatewayV2HTTPResponse{}, err
		}
		return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: result.Data.ID}, nil
	})
	lambda.Start(handler)
}
