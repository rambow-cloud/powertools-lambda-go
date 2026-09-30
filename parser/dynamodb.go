package parser

import (
	"context"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// DynamoDBMarshalled decodes raw AttributeValue objects before schema validation.
// Raw binary values and large integers retain the Commons conversion contract.
func DynamoDBMarshalled[T any](schema Schema[T]) Schema[T] {
	decoded := SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) {
		value, ok := objectValue(input)
		if !ok {
			return nil, []Issue{{Code: "custom", Message: "Could not unmarshall DynamoDB stream record"}}, nil
		}
		result, err := commons.UnmarshallDynamoDB(value)
		if err != nil {
			return nil, []Issue{{Code: "custom", Message: "Could not unmarshall DynamoDB stream record"}}, nil
		}
		return result, nil, nil
	})
	return Pipe(decoded, schema)
}
