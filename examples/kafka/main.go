// Command kafka demonstrates lazy JSON decoding in a native Lambda handler.
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/kafka"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	consumer := kafka.New(kafka.Config{Value: &kafka.FieldConfig{Type: kafka.JSON}})
	handler := kafka.WrapHandler(consumer, func(ctx context.Context, event *kafka.ConsumerRecords) (any, error) {
		requestLog := appLog.WithContext(ctx)
		for _, record := range event.Records {
			value, err := record.Value(ctx)
			if err != nil {
				return nil, err
			}
			if value == nil {
				continue
			}
			if err := requestLog.Info("Order received", logger.Fields{"order": value}); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	lambda.Start(logger.WrapHandler(appLog, handler))
}
