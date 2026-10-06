package dynamodb_test

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	"github.com/rambow-cloud/powertools-lambda-go/idempotency/dynamodb"
)

// Actual persistence operations require a provisioned table and AWS permissions.
func ExampleNew() {
	// Supply the application's credentials provider before persistence operations.
	cfg := aws.Config{Region: "ap-east-1"}
	store, err := dynamodb.New(sdk.NewFromConfig(cfg), dynamodb.Options{
		TableName: "example-idempotency",
	})
	if err != nil {
		panic(err)
	}
	manager, err := idempotency.New(store, idempotency.Options{
		KeyPrefix: "orders", EventKeyJMESPath: "orderId",
	})
	if err != nil {
		panic(err)
	}
	_ = manager // Reuse this manager when wrapping business operations.
}
