// Package main validates a typed Lambda event and response using JSON Schema.
package main

import (
	"context"
	jsonv1 "encoding/json"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

type order struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	if err := run(appLog); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func run(appLog *logger.Logger) error {
	inbound, err := validation.Compile(context.Background(), jsonv1.RawMessage(`{"type":"object","required":["id","amount"],"properties":{"id":{"type":"string","minLength":1},"amount":{"type":"integer","minimum":1}},"additionalProperties":false}`), validation.Options{})
	if err != nil {
		return err
	}
	outbound, err := validation.Compile(context.Background(), jsonv1.RawMessage(`{"type":"string","minLength":1}`), validation.Options{})
	if err != nil {
		return err
	}
	handler := validation.WrapHandler[jsonv1.RawMessage](inbound, outbound, func(_ context.Context, input order) (string, error) { return input.ID, nil })
	lambda.Start(handler)
	return nil
}
