package bedrock_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/bedrock"
)

func ExampleResolver_Resolve() {
	app := bedrock.New(bedrock.Options{})
	app.Tool(func(_ context.Context, parameters *bedrock.Parameters, _ bedrock.Event) (any, error) {
		return parameters.Get("orderId"), nil
	}, bedrock.Configuration{Name: "lookupOrder", Description: "Look up an order"})
	event := map[string]any{
		"actionGroup": "orders", "function": "lookupOrder", "messageVersion": "1.0",
		"inputText": "Find my order", "sessionId": "example-session",
		"agent":             map[string]any{"name": "example", "id": "example-agent", "alias": "example", "version": "1"},
		"sessionAttributes": map[string]any{}, "promptSessionAttributes": map[string]any{},
		"parameters": []any{map[string]any{"name": "orderId", "type": "string", "value": "order-123"}},
	}
	value, err := app.Resolve(context.Background(), event)
	if err != nil {
		panic(err)
	}
	response := value.(map[string]any)["response"].(map[string]any)
	fmt.Println(response["actionGroup"], response["function"])
	// Output: orders lookupOrder
}
