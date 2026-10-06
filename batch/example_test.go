package batch_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rambow-cloud/powertools-lambda-go/batch"
)

func ExampleWrapSQS() {
	processor, err := batch.NewSQS[string](batch.Options{Sequential: true})
	if err != nil {
		panic(err)
	}
	handler := batch.WrapSQS(processor, func(_ context.Context, record events.SQSMessage) (string, error) {
		if record.Body == "retry" {
			return "", errors.New("temporary failure")
		}
		return record.Body, nil
	})
	response, err := handler(context.Background(), events.SQSEvent{Records: []events.SQSMessage{
		{MessageId: "accepted", Body: "order-123"},
		{MessageId: "failed", Body: "retry"},
	}})
	fmt.Println(response.BatchItemFailures[0].ItemIdentifier, err)
	// Enable ReportBatchItemFailures on the event source mapping in production.
	// Output: failed <nil>
}
