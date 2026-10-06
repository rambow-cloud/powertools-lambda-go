package appsyncgraphql_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncgraphql"
)

func ExampleResolver_Resolve() {
	app := appsyncgraphql.New(appsyncgraphql.Options{})
	app.OnQuery("order", func(_ context.Context, input, _ any) (any, error) {
		return input.(map[string]any)["id"], nil
	})
	event := map[string]any{
		"arguments": map[string]any{"id": "order-123"},
		"identity":  nil, "source": nil, "prev": nil, "stash": map[string]any{},
		"request": map[string]any{"headers": map[string]any{}, "domainName": nil},
		"info": map[string]any{"parentTypeName": "Query", "fieldName": "order",
			"variables": map[string]any{}},
	}
	value, err := app.Resolve(context.Background(), event)
	fmt.Println(value, err)
	// Output: order-123 <nil>
}
