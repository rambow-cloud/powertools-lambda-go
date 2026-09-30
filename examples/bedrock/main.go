package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/bedrock"
)

func main() {
	app := bedrock.New(bedrock.Options{})
	app.Tool(func(_ context.Context, parameters *bedrock.Parameters, event bedrock.Event) (any, error) {
		return fmt.Sprintf("Hello, %v", parameters.Get("name")), nil
	}, bedrock.Configuration{Name: "greeting", Description: "Greet a person by name"})
	lambda.Start(app.Resolve)
}
