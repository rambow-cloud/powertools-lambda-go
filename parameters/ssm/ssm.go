// Package ssm retrieves and writes AWS Systems Manager parameters.
package ssm

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/commons/awssdk"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

type Client interface {
	GetParameter(context.Context, *sdk.GetParameterInput, ...func(*sdk.Options)) (*sdk.GetParameterOutput, error)
	GetParameters(context.Context, *sdk.GetParametersInput, ...func(*sdk.Options)) (*sdk.GetParametersOutput, error)
	GetParametersByPath(context.Context, *sdk.GetParametersByPathInput, ...func(*sdk.Options)) (*sdk.GetParametersByPathOutput, error)
	PutParameter(context.Context, *sdk.PutParameterInput, ...func(*sdk.Options)) (*sdk.PutParameterOutput, error)
}

type Provider struct {
	client Client
	cache  *parameters.Cache
}

func New(client Client) *Provider { return &Provider{client: client, cache: parameters.NewCache(nil)} }
func (p *Provider) ClearCache()   { p.cache.ClearCache() }

type GetOptions struct {
	parameters.Options
	Decrypt    *bool
	SDKOptions *sdk.GetParameterInput
}

type MultipleOptions struct {
	parameters.Options
	Decrypt    *bool
	Recursive  *bool
	SDKOptions *sdk.GetParametersByPathInput
}

func userAgent(o *sdk.Options) { o.APIOptions = append(o.APIOptions, awssdk.UserAgent("parameters")) }

func decrypt(explicit, sdkValue *bool) (*bool, error) {
	if explicit != nil {
		return explicit, nil
	}
	if sdkValue != nil {
		return sdkValue, nil
	}
	value, err := commons.BoolEnv("POWERTOOLS_PARAMETERS_SSM_DECRYPT", true, false)
	return aws.Bool(value), err
}

func parameterInput(name string, options GetOptions) (sdk.GetParameterInput, error) {
	input := sdk.GetParameterInput{}
	if options.SDKOptions != nil {
		input = *options.SDKOptions
	}
	input.Name = aws.String(name)
	value, err := decrypt(options.Decrypt, input.WithDecryption)
	input.WithDecryption = aws.Bool(aws.ToBool(value))
	return input, err
}

func (p *Provider) Get(ctx context.Context, name string, options GetOptions) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, parameters.GetError(name, err)
	}
	input, err := parameterInput(name, options)
	if err != nil {
		return nil, parameters.GetError(name, err)
	}
	options.RequestKey = strconv.FormatBool(aws.ToBool(input.WithDecryption))
	return p.cache.Get(ctx, name, options.Options, func(ctx context.Context) (any, error) {
		return p.getInput(ctx, input, options.ThrowOnMissing)
	})
}

func (p *Provider) get(ctx context.Context, name string, options GetOptions) (any, error) {
	input, err := parameterInput(name, options)
	if err != nil {
		return nil, err
	}
	return p.getInput(ctx, input, options.ThrowOnMissing)
}

func (p *Provider) getInput(ctx context.Context, input sdk.GetParameterInput, throwOnMissing bool) (any, error) {
	out, err := p.client.GetParameter(ctx, &input, userAgent)
	if err != nil {
		var missing *types.ParameterNotFound
		if throwOnMissing && errors.As(err, &missing) {
			return nil, nil
		}
		return nil, err
	}
	if out == nil || out.Parameter == nil || out.Parameter.Value == nil {
		return nil, nil
	}
	return *out.Parameter.Value, nil
}

func (p *Provider) GetMultiple(ctx context.Context, path string, options MultipleOptions) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, parameters.GetError(path, err)
	}
	input := sdk.GetParametersByPathInput{}
	if options.SDKOptions != nil {
		input = *options.SDKOptions
	}
	input.Path = aws.String(path)
	value, err := decrypt(options.Decrypt, input.WithDecryption)
	if err != nil {
		return nil, parameters.GetError(path, err)
	}
	input.WithDecryption = aws.Bool(aws.ToBool(value))
	if options.Recursive != nil {
		input.Recursive = options.Recursive
	}
	input.Recursive = aws.Bool(aws.ToBool(input.Recursive))
	key, err := json.Marshal(input)
	if err != nil {
		return nil, parameters.GetError(path, err)
	}
	options.RequestKey = string(key)
	return p.cache.GetMultiple(ctx, path, options.Options, func(ctx context.Context) (map[string]any, error) {
		result := make(map[string]any)
		pages := sdk.NewGetParametersByPathPaginator(p.client, &input, func(o *sdk.GetParametersByPathPaginatorOptions) { o.StopOnDuplicateToken = true })
		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx, userAgent)
			if err != nil {
				return nil, err
			}
			for _, item := range page.Parameters {
				if item.Name == nil {
					continue
				}
				name := strings.TrimPrefix(strings.TrimPrefix(*item.Name, path), "/")
				if item.Value == nil {
					result[name] = nil
				} else {
					result[name] = *item.Value
				}
			}
		}
		return result, nil
	})
}

// Set passes native SDK write options through and supplies the reference defaults.
// Writes do not invalidate cached reads; call ClearCache or use ForceFetch afterward.
func (p *Provider) Set(ctx context.Context, name, value string, options *sdk.PutParameterInput) (int64, error) {
	input := sdk.PutParameterInput{}
	if options != nil {
		input = *options
	}
	input.Name, input.Value = aws.String(name), aws.String(value)
	if input.Type == "" {
		input.Type = types.ParameterTypeString
	}
	if input.Tier == "" {
		input.Tier = types.ParameterTierStandard
	}
	if input.Overwrite == nil {
		input.Overwrite = aws.Bool(false)
	}
	result, err := p.client.PutParameter(ctx, &input, userAgent)
	if err != nil {
		return 0, &parameters.SetParameterError{Name: name, Err: err}
	}
	if result == nil {
		return 0, &parameters.SetParameterError{Name: name, Err: errors.New("empty SDK response")}
	}
	return result.Version, nil
}
