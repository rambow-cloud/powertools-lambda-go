// Command kafka validates ordered Kafka records before executing a typed Lambda handler.
package main

import (
	"context"
	jsonv1 "encoding/json"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
)

type order struct {
	ID     string  `json:"id"`
	Amount float64 `json:"amount"`
}

func main() {
	payload := parser.Typed[order](parser.Object(parser.Field{Name: "id", Schema: parser.String()}, parser.Field{Name: "amount", Schema: parser.Number()}))
	// RawMessage retains the topic key order received from the Lambda runtime.
	handler := parser.WrapHandler[jsonv1.RawMessage](envelopes.Kafka(parser.JSONStringified(payload)), func(ctx context.Context, orders []order) (int, error) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		return len(orders), nil
	})
	lambda.Start(handler)
}
