package ssm_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	sdk "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/ssm"
)

// This example requires AWS credentials and an existing Parameter Store value.
func ExampleProvider_Get() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}
	provider := ssm.New(sdk.NewFromConfig(cfg))
	value, err := provider.Get(ctx, "/example/feature", ssm.GetOptions{})
	if err != nil {
		panic(err)
	}
	_ = value // Reuse the provider across invocations to retain cached values.
}
