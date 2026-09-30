// Package dynamodb retrieves configuration values from a DynamoDB table.
package dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rambow-cloud/powertools-lambda-go/commons/awssdk"
	values "github.com/rambow-cloud/powertools-lambda-go/commons/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

type Client interface {
	GetItem(context.Context, *sdk.GetItemInput, ...func(*sdk.Options)) (*sdk.GetItemOutput, error)
	Query(context.Context, *sdk.QueryInput, ...func(*sdk.Options)) (*sdk.QueryOutput, error)
}

type Config struct{ TableName, KeyAttribute, SortAttribute, ValueAttribute string }
type Provider struct {
	client Client
	cache  *parameters.Cache
	config Config
}
type GetOptions struct {
	parameters.Options
	SDKOptions *sdk.GetItemInput
}
type MultipleOptions struct {
	parameters.Options
	SDKOptions *sdk.QueryInput
}

func New(client Client, config Config) (*Provider, error) {
	if config.TableName == "" {
		return nil, fmt.Errorf("table name is required")
	}
	if config.KeyAttribute == "" {
		config.KeyAttribute = "id"
	}
	if config.SortAttribute == "" {
		config.SortAttribute = "sk"
	}
	if config.ValueAttribute == "" {
		config.ValueAttribute = "value"
	}
	return &Provider{client: client, cache: parameters.NewCache(nil), config: config}, nil
}
func (p *Provider) ClearCache() { p.cache.ClearCache() }

func userAgent(o *sdk.Options) { o.APIOptions = append(o.APIOptions, awssdk.UserAgent("parameters")) }

func (p *Provider) Get(ctx context.Context, name string, options GetOptions) (any, error) {
	return p.cache.Get(ctx, name, options.Options, func(ctx context.Context) (any, error) {
		input := sdk.GetItemInput{}
		if options.SDKOptions != nil {
			input = *options.SDKOptions
		}
		input.TableName = aws.String(p.config.TableName)
		input.Key = map[string]types.AttributeValue{p.config.KeyAttribute: &types.AttributeValueMemberS{Value: name}}
		input.ProjectionExpression = aws.String("#value")
		input.ExpressionAttributeNames = map[string]string{"#value": p.config.ValueAttribute}
		out, err := p.client.GetItem(ctx, &input, userAgent)
		if err != nil {
			return nil, err
		}
		if out == nil || out.Item[p.config.ValueAttribute] == nil {
			return nil, nil
		}
		return values.UnmarshalAttribute(out.Item[p.config.ValueAttribute])
	})
}

func (p *Provider) GetMultiple(ctx context.Context, path string, options MultipleOptions) (map[string]any, error) {
	return p.cache.GetMultiple(ctx, path, options.Options, func(ctx context.Context) (map[string]any, error) {
		input := sdk.QueryInput{}
		if options.SDKOptions != nil {
			input = *options.SDKOptions
		}
		input.TableName = aws.String(p.config.TableName)
		input.KeyConditionExpression = aws.String("#key = :key")
		input.ExpressionAttributeValues = map[string]types.AttributeValue{":key": &types.AttributeValueMemberS{Value: path}}
		input.ExpressionAttributeNames = map[string]string{"#key": p.config.KeyAttribute, "#sk": p.config.SortAttribute, "#value": p.config.ValueAttribute}
		input.ProjectionExpression = aws.String("#sk, #value")
		result := make(map[string]any)
		pages := sdk.NewQueryPaginator(p.client, &input)
		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx, userAgent)
			if err != nil {
				return nil, err
			}
			for _, item := range page.Items {
				value, err := values.UnmarshalItem(item)
				if err != nil {
					return nil, err
				}
				name, exists := value[p.config.SortAttribute]
				if !exists {
					return nil, fmt.Errorf("query result has no sort attribute %q", p.config.SortAttribute)
				}
				result[fmt.Sprint(name)] = value[p.config.ValueAttribute]
			}
		}
		return result, nil
	})
}
