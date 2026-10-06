package kafka_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/kafka"
)

func ExampleWrapHandler() {
	consumer := kafka.New(kafka.Config{})
	handler := kafka.WrapHandler(consumer, func(ctx context.Context, event *kafka.ConsumerRecords) (any, error) {
		return event.Records[0].Value(ctx)
	})
	value, err := handler(context.Background(), json.RawMessage(
		`{"records":{"orders-0":[{"value":"aGVsbG8=","headers":[]}]}}`))
	fmt.Println(value, err)
	// Output: hello <nil>
}
