package appconfigagent_test

import (
	"context"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/parameters/appconfigagent"
)

// This example requires a configured AppConfig Agent or Lambda extension.
func ExampleGetConfig() {
	value, err := appconfigagent.GetConfig(context.Background(), "features",
		appconfigagent.Options{Application: "example", Environment: "development",
			Endpoint: "http://localhost:2772", Timeout: time.Second})
	if err != nil {
		panic(err)
	}
	_ = value // The Agent owns caching and polling.
}
