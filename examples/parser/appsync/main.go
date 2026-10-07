// Command appsync validates resolver metadata and typed arguments before handling a query.
package main

import (
	"context"
	jsonv1 "encoding/json"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

type query struct {
	Arguments struct {
		ID string `json:"id"`
	} `json:"arguments"`
}

func main() {
	schema := parser.Typed[query](schemas.AppSyncResolverSchema.Extend(parser.Field{Name: "arguments", Schema: parser.Object(parser.Field{Name: "id", Schema: parser.String()})}))
	lambda.Start(parser.WrapHandler[jsonv1.RawMessage](schema, func(ctx context.Context, input query) (string, error) { return input.Arguments.ID, ctx.Err() }))
}
