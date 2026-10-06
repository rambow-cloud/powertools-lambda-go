package secrets_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	sdk "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/secrets"
)

// This example requires AWS credentials and an existing Secrets Manager secret.
func ExampleProvider_Get() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}
	provider := secrets.New(sdk.NewFromConfig(cfg))
	value, err := provider.Get(ctx, "example/secret", secrets.GetOptions{})
	if err != nil {
		panic(err)
	}
	_ = value // Consume the secret without logging it.
}
