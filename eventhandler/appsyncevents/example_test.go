package appsyncevents_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncevents"
)

func ExampleResolver_Resolve() {
	app := appsyncevents.New(appsyncevents.Options{})
	app.OnPublish("/orders/*", func(_ context.Context, payload any, _ appsyncevents.Event) (any, error) {
		return payload, nil
	})
	event := map[string]any{
		"identity": nil, "result": nil, "error": nil, "prev": nil,
		"stash": map[string]any{}, "outErrors": []any{},
		"request": map[string]any{"headers": map[string]any{}, "domainName": nil},
		"info": map[string]any{"operation": "PUBLISH",
			"channel":          map[string]any{"path": "/orders/created", "segments": []any{"orders", "created"}},
			"channelNamespace": map[string]any{"name": "orders"}},
		"events": []any{map[string]any{"id": "event-1", "payload": "order-123"}},
	}
	value, err := app.Resolve(context.Background(), event)
	if err != nil {
		panic(err)
	}
	items := value.(map[string]any)["events"].([]any)
	item := items[0].(map[string]any)
	fmt.Println(item["id"], item["payload"])
	// Output: event-1 order-123
}
