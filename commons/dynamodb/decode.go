// Package dynamodb adapts shared DynamoDB conversion to AWS SDK types.
package dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// AttributeError preserves the public error identity across raw and SDK adapters.
type AttributeError = commons.DynamoDBAttributeError

// Number applies the shared safe-number and large-integer conversion.
func Number(value string) (any, error) { return commons.DynamoDBNumber(value) }

// UnmarshalAttribute uses the SDK's native binary/set decoding with shared numeric conversion.
func UnmarshalAttribute(input types.AttributeValue) (any, error) {
	var value any
	decoder := attributevalue.NewDecoder(func(o *attributevalue.DecoderOptions) { o.UseNumber = true })
	if err := decoder.Decode(input, &value); err != nil {
		return nil, err
	}
	return normalize(value)
}
func UnmarshalItem(input map[string]types.AttributeValue) (map[string]any, error) {
	value, err := UnmarshalAttribute(&types.AttributeValueMemberM{Value: input})
	if err != nil {
		return nil, err
	}
	return value.(map[string]any), nil
}
func normalize(value any) (any, error) {
	switch v := value.(type) {
	case attributevalue.Number:
		return Number(string(v))
	case map[string]any:
		for key, item := range v {
			converted, err := normalize(item)
			if err != nil {
				return nil, err
			}
			v[key] = converted
		}
		return v, nil
	case []any:
		for i, item := range v {
			converted, err := normalize(item)
			if err != nil {
				return nil, err
			}
			v[i] = converted
		}
		return v, nil
	case []attributevalue.Number:
		result := make([]any, len(v))
		for i, item := range v {
			converted, err := Number(string(item))
			if err != nil {
				return nil, err
			}
			result[i] = converted
		}
		return result, nil
	default:
		return value, nil
	}
}

// UnmarshallDynamoDB delegates raw AttributeValue conversion to dependency-free Commons.
func UnmarshallDynamoDB(input map[string]any) (map[string]any, error) {
	return commons.UnmarshallDynamoDB(input)
}
