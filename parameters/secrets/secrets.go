package secrets

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/rambow-cloud/powertools-lambda-go/commons/awssdk"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

type Client interface {
	GetSecretValue(context.Context, *sdk.GetSecretValueInput, ...func(*sdk.Options)) (*sdk.GetSecretValueOutput, error)
}
type Provider struct {
	client Client
	cache  *parameters.Cache
}
type GetOptions struct {
	parameters.Options
	SDKOptions *sdk.GetSecretValueInput
}

func New(client Client) *Provider { return &Provider{client: client, cache: parameters.NewCache(nil)} }
func (p *Provider) ClearCache()   { p.cache.ClearCache() }

func userAgent(o *sdk.Options) { o.APIOptions = append(o.APIOptions, awssdk.UserAgent("parameters")) }

func (p *Provider) Get(ctx context.Context, name string, options GetOptions) (any, error) {
	input := sdk.GetSecretValueInput{}
	if options.SDKOptions != nil {
		input = *options.SDKOptions
	}
	input.SecretId = aws.String(name)
	key, err := json.Marshal(input)
	if err != nil {
		return nil, parameters.GetError(name, err)
	}
	options.RequestKey = string(key)
	return p.cache.Get(ctx, name, options.Options, func(ctx context.Context) (any, error) {
		out, err := p.client.GetSecretValue(ctx, &input, userAgent)
		if err != nil {
			var missing *types.ResourceNotFoundException
			if options.ThrowOnMissing && errors.As(err, &missing) {
				return nil, nil
			}
			return nil, err
		}
		if out == nil {
			return nil, nil
		}
		if out.SecretString != nil && *out.SecretString != "" {
			return *out.SecretString, nil
		}
		if out.SecretBinary != nil {
			return out.SecretBinary, nil
		}
		return nil, nil
	})
}
