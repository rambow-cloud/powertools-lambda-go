package dynamodb_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	sdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/dynamodb"
)

// This example requires AWS credentials and a configuration table with data.
func ExampleProvider_Get() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}
	provider, err := dynamodb.New(sdk.NewFromConfig(cfg), dynamodb.Config{
		TableName: "example-configuration", KeyAttribute: "id", ValueAttribute: "value",
	})
	if err != nil {
		panic(err)
	}
	value, err := provider.Get(ctx, "feature", dynamodb.GetOptions{})
	if err != nil {
		panic(err)
	}
	_ = value
}
