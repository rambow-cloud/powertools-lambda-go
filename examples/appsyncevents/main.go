package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncevents"
)

func main() {
	app := appsyncevents.New(appsyncevents.Options{WarnOnLargePayload: true})
	app.OnPublish("/orders/*", func(ctx context.Context, payload any, event appsyncevents.Event) (any, error) {
		return payload, nil
	})
	app.OnSubscribe("/orders/private", func(ctx context.Context, event appsyncevents.Event) error {
		if event["identity"] == nil {
			return &appsyncevents.UnauthorizedError{Message: "Authentication required"}
		}
		return nil
	})
	lambda.Start(app.Resolve)
}
