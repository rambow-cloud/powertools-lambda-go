// Command batch demonstrates a typed SQS Lambda batch handler.
package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/batch"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

type order struct {
	ID string `json:"id"`
}

func main() {
	processor, err := batch.NewSQS[string](batch.Options{MaxConcurrency: 4})
	if err != nil {
		log.Fatal(err)
	}
	schema := parser.JSONStringified(parser.Typed[order](parser.Object(parser.Field{Name: "id", Schema: parser.String()})))
	handler := batch.WithParser(func(ctx context.Context, record events.SQSMessage) (order, error) {
		return parser.Parse(ctx, record.Body, schema)
	}, func(ctx context.Context, value order) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return value.ID, nil
	})
	lambda.Start(batch.WrapSQS(processor, handler))
}
