---
description: "Handle AWS AppSync Events in Go Lambda with Powertools routing, publish and subscribe handlers and documented event contracts."
---

# AppSync Events

Handle AppSync Events publish and subscribe requests using channel routes.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncevents@v1.3.1
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## Handle a published order

The `/orders/*` handler returns each published payload unchanged. Connect this Lambda resolver to your AppSync Events namespace.

**Input**

Publish `{ "id": "ORD-123" }` on `/orders/created`.

??? example "Complete Lambda event"

    ```json
    {
      "info": {
        "operation": "PUBLISH",
        "channel": {
          "path": "/orders/created",
          "segments": [
            "orders",
            "created"
          ]
        },
        "channelNamespace": {
          "name": "orders"
        }
      },
      "events": [
        {
          "id": "e1",
          "payload": {
            "id": "ORD-123"
          }
        }
      ],
      "identity": null,
      "result": null,
      "error": null,
      "prev": null,
      "request": {
        "headers": {},
        "domainName": null
      },
      "stash": {},
      "outErrors": []
    }
    ```

**Code**

```go
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncevents"
)

func main() {
	app := appsyncevents.New(appsyncevents.Options{})
	app.OnPublish("/orders/*", func(ctx context.Context, payload any, event appsyncevents.Event) (any, error) {
		return payload, nil
	})
	lambda.Start(app.Resolve)
}
```

**Output**

```json
{
  "events": [
    {
      "id": "e1",
      "payload": {
        "id": "ORD-123"
      }
    }
  ]
}
```

## Authorize a subscription

```go
app.OnSubscribe("/orders/private", func(ctx context.Context, event appsyncevents.Event) error {
    if event["identity"] == nil {
        return &appsyncevents.UnauthorizedError{Message: "Authentication required"}
    }
    return nil
})
```

A subscription without identity raises `UnauthorizedException`. It is an invocation error, rather than a successful response with a null value.

## Choose publication mode

| Mode | Callback receives | Result |
| --- | --- | --- |
| Default | One payload at a time | Ordered `{id, payload}` or `{id, error}` items |
| `PublishOptions{Aggregate: true}` | The event collection | Your aggregate result |

Register exact channel paths for specific actions or `*` routes for a channel family. See [routing options](reference/APPSYNC_EVENTS.md#routing-and-process-state).

## Things to know

- Individual callbacks can run concurrently. Synchronize any callback-owned shared state.
- Ordinary callback errors become response errors. Authorization errors propagate after all started workers finish.
- Large-payload warnings do not truncate events. Keep AppSync service limits in mind.
- The resolver handles incoming invocations; it does not publish messages to the AppSync service.

## Next steps

See the [API and behavior notes](reference/APPSYNC_EVENTS.md) for advanced options, and [module installation](MODULES.md) for dependencies.
