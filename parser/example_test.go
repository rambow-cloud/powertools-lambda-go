package parser_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

func ExampleParse() {
	schema := parser.Object(parser.Field{Name: "orderId", Schema: parser.String()})
	value, err := parser.Parse(context.Background(), map[string]any{"orderId": "order-123"}, schema)
	if err != nil {
		panic(err)
	}
	fmt.Println(value.(map[string]any)["orderId"])
	// Output: order-123
}
