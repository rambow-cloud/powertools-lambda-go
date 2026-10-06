package jmespath_test

import (
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
)

func ExampleCompile() {
	expression, err := jmespath.Compile("detail.orderId")
	if err != nil {
		panic(err)
	}
	value, err := expression.Search(map[string]any{
		"detail": map[string]any{"orderId": "order-123"},
	})
	fmt.Println(value, err)
	// Output: order-123 <nil>
}
