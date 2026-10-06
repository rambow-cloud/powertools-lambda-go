package logger_test

import (
	"bytes"
	"encoding/json"
	"fmt"

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
