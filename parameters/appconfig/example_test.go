package appconfig_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	sdk "github.com/aws/aws-sdk-go-v2/service/appconfigdata"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/appconfig"
)

// This example requires AWS credentials and an existing AppConfig deployment.
func ExampleProvider_Get() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}
	provider, err := appconfig.New(sdk.NewFromConfig(cfg), appconfig.Config{
		Application: "example", Environment: "development",
	})
	if err != nil {
		panic(err)
	}
	value, err := provider.Get(ctx, "features", appconfig.GetOptions{})
	if err != nil {
		panic(err)
	}
	_ = value // Reuse the provider so session tokens survive warm invocations.
}
