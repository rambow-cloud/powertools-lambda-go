// Command parser demonstrates validated EventBridge payloads in a typed handler.
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
	payload := parser.Typed[order](parser.Object(
		parser.Field{Name: "id", Schema: parser.String()},
		parser.Field{Name: "amount", Schema: parser.Refine(parser.Number(), func(value any) bool { return value.(float64) >= 0 }, "amount must be non-negative")},
	))
	handler := parser.WrapHandler[events.CloudWatchEvent](envelopes.EventBridge(payload), func(ctx context.Context, input order) (string, error) { return input.ID, ctx.Err() })
	lambda.Start(handler)
}
