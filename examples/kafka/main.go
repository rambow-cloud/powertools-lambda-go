// Command kafka demonstrates lazy JSON decoding in a native Lambda handler.
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/kafka"
)

func main() {
	consumer := kafka.New(kafka.Config{Value: &kafka.FieldConfig{Type: kafka.JSON}})
	lambda.Start(kafka.WrapHandler(consumer, func(ctx context.Context, event *kafka.ConsumerRecords) (any, error) {
		for _, record := range event.Records {
			value, err := record.Value(ctx)
			if err != nil {
				return nil, err
			}
			if value == nil {
				continue
			}
			// Process the decoded value here. Return an error to fail this invocation.
		}
		return nil, nil
	}))
}
