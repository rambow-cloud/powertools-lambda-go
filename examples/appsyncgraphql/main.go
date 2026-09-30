package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncgraphql"
)

func main() {
	app := appsyncgraphql.New(appsyncgraphql.Options{})
	app.OnQuery("hello", func(_ context.Context, arguments, event any) (any, error) {
		return map[string]any{"message": "Hello", "arguments": arguments}, nil
	})
	lambda.Start(app.Resolve)
}
