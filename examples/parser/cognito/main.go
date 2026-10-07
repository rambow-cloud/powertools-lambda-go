// Command cognito validates a sign-up trigger before applying an application rule.
package main

import (
	"context"
	jsonv1 "encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func main() {
	schema := parser.Typed[events.CognitoEventUserPoolsPreSignup](schemas.PreSignupTriggerSchema)
	lambda.Start(parser.WrapHandler[jsonv1.RawMessage](schema, func(ctx context.Context, event events.CognitoEventUserPoolsPreSignup) (events.CognitoEventUserPoolsPreSignup, error) {
		if event.Request.UserAttributes["email"] == "" {
			return event, fmt.Errorf("email is required")
		}
		// Input validation does not restrict the handler's response mutations.
		return event, ctx.Err()
	}))
}
