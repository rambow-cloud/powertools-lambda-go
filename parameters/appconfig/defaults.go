package appconfig

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	sdk "github.com/aws/aws-sdk-go-v2/service/appconfigdata"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/internal/parameterdefaults"
)

var defaultProvider = parameterdefaults.New[*Provider]()

type DefaultOptions struct {
	GetOptions
	Application, Environment string
}

// GetAppConfig uses the application and environment from the first successful initialization.
// Use separate providers for different applications or environments.
func GetAppConfig(ctx context.Context, name string, options DefaultOptions) (any, error) {
	p, err := defaultProvider.Get(ctx, func(ctx context.Context) (*Provider, error) {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, err
		}
		return New(sdk.NewFromConfig(cfg), Config{Application: options.Application, Environment: options.Environment})
	})
	if err != nil {
		return nil, parameters.GetError(name, err)
	}
	return p.Get(ctx, name, options.GetOptions)
}
