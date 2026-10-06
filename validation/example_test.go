package validation_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

func ExampleCompile() {
	ctx := context.Background()
	schema, err := validation.Compile(ctx, map[string]any{
		"type": "object", "required": []any{"orderId"},
		"properties": map[string]any{"orderId": map[string]any{"type": "string"}},
	}, validation.Options{})
	if err != nil {
		panic(err)
	}
	value, err := schema.Validate(ctx, map[string]any{"orderId": "order-123"})
	if err != nil {
		panic(err)
	}
	fmt.Println(value.(map[string]any)["orderId"])
	// Output: order-123
}
