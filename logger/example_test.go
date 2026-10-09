package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func ExampleNew() {
	var output bytes.Buffer
	log := logger.New(logger.WithServiceName("orders"),
		logger.WithLevel(logger.InfoLevel), logger.WithOutput(&output))
	if err := log.Info("Order accepted", logger.Fields{"order_id": "order-123"}); err != nil {
		panic(err)
	}
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		panic(err)
	}
	fmt.Println(entry["service"], entry["message"], entry["order_id"])
	// Output: orders Order accepted order-123
}

func ExampleWrapRawHandler() {
	var output bytes.Buffer
	log := logger.New(logger.WithOutput(&output), logger.WithLevel(logger.InfoLevel))
	enabled := true
	handler := logger.WrapRawHandler(log, func(_ context.Context, event struct {
		Name string `json:"name"`
	}) (string, error) {
		return event.Name, nil
	}, logger.HandlerOptions{LogEvent: &enabled})
	response, err := lambda.NewHandler(handler).Invoke(context.Background(), []byte(`{"name":"Alice","age":30}`))
	if err != nil {
		panic(err)
	}
	var entry struct {
		Event map[string]any `json:"event"`
	}
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		panic(err)
	}
	fmt.Println(string(response), entry.Event["name"], entry.Event["age"])
	// Output: "Alice" Alice 30
}
