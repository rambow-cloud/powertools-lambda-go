package envelopes_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
)

func ExampleEventBridge() {
	schema := envelopes.EventBridge(parser.Object(
		parser.Field{Name: "orderId", Schema: parser.String()},
	))
	event := map[string]any{
		"version": "0", "id": "example-event", "detail-type": "OrderCreated",
		"source": "example.orders", "account": "000000000000",
		"time": "2026-01-01T00:00:00Z", "region": "ap-east-1", "resources": []any{},
		"detail": map[string]any{"orderId": "order-123"},
	}
	value, err := parser.Parse(context.Background(), event, schema)
	if err != nil {
		panic(err)
	}
	fmt.Println(value.(map[string]any)["orderId"])
	// Output: order-123
}
