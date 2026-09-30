package ssm

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	sdk "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/internal/parameterdefaults"
)

var defaultProvider = parameterdefaults.New[*Provider]()

func getDefault(ctx context.Context) (*Provider, error) {
	p, err := defaultProvider.Get(ctx, func(ctx context.Context) (*Provider, error) {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, err
		}
		return New(sdk.NewFromConfig(cfg)), nil
	})
	return p, parameters.GetError("default SSM provider", err)
}

func GetParameter(ctx context.Context, name string, options GetOptions) (any, error) {
	p, err := getDefault(ctx)
	if err != nil {
		return nil, err
	}
	return p.Get(ctx, name, options)
}
func GetParameters(ctx context.Context, path string, options MultipleOptions) (map[string]any, error) {
	p, err := getDefault(ctx)
	if err != nil {
		return nil, err
	}
	return p.GetMultiple(ctx, path, options)
}
func GetParametersByName(ctx context.Context, names map[string]GetOptions, options ByNameOptions) (map[string]any, error) {
	p, err := getDefault(ctx)
	if err != nil {
		return nil, err
	}
	return p.GetParametersByName(ctx, names, options)
}
func SetParameter(ctx context.Context, name, value string, options *sdk.PutParameterInput) (int64, error) {
	p, err := getDefault(ctx)
	if err != nil {
		return 0, &parameters.SetParameterError{Name: name, Err: err}
	}
	return p.Set(ctx, name, value, options)
}
