package secrets

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	sdk "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/internal/parameterdefaults"
)

var defaultProvider = parameterdefaults.New[*Provider]()

func GetSecret(ctx context.Context, name string, options GetOptions) (any, error) {
	p, err := defaultProvider.Get(ctx, func(ctx context.Context) (*Provider, error) {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, err
		}
		return New(sdk.NewFromConfig(cfg)), nil
	})
	if err != nil {
		return nil, parameters.GetError(name, err)
	}
	return p.Get(ctx, name, options)
}
