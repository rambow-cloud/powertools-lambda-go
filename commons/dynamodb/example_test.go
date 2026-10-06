package dynamodb_test

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rambow-cloud/powertools-lambda-go/commons/dynamodb"
)

func ExampleUnmarshalItem() {
	item, err := dynamodb.UnmarshalItem(map[string]types.AttributeValue{
		"name":  &types.AttributeValueMemberS{Value: "orders"},
		"count": &types.AttributeValueMemberN{Value: "3"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(item["name"], item["count"])
	// Output: orders 3
}
