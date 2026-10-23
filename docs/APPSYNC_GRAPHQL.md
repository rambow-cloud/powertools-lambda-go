---
description: "Resolve AWS AppSync GraphQL requests in Go Lambda with Powertools, including route dispatch, batching and error contracts."
---

# AppSync GraphQL

Route AppSync GraphQL resolver events to Go query, mutation and batch handlers.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncgraphql@v1.3.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## Resolve a query

Register `Query.hello` and configure AppSync to invoke this Lambda data source. Define a required `name` argument in your AppSync schema. The callback receives the arguments and full resolver event.

**Input**

`Query.hello` with arguments `{ "name": "Ada" }`.

??? example "Complete Lambda event"

    ```json
    {
      "arguments": {
        "name": "Ada"
      },
      "identity": null,
      "source": null,
      "prev": null,
      "stash": {},
      "request": {
        "headers": {},
        "domainName": null
      },
      "info": {
        "parentTypeName": "Query",
        "fieldName": "hello",
        "variables": {}
      }
    }
    ```

**Code**

```go
package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncgraphql"
)

func main() {
	app := appsyncgraphql.New(appsyncgraphql.Options{})
	app.OnQuery("hello", func(ctx context.Context, arguments, event any) (any, error) {
		name := arguments.(map[string]any)["name"]
		return map[string]any{"message": fmt.Sprintf("Hello, %v", name)}, nil
	})
	lambda.Start(app.Resolve)
}
```

**Output**

```json
{
  "message": "Hello, Ada"
}
```

## Register a mutation

```go
app.OnMutation("createOrder", func(ctx context.Context, arguments, event any) (any, error) {
    return arguments, nil
})
```

The route is selected by `info.parentTypeName` and `info.fieldName`. A `Mutation.createOrder` event with `{"id":"ORD-123"}` as its arguments returns that object.

## Handle batches

Use `OnBatchQuery`, `OnBatchMutation` or `OnBatchResolver` when AppSync sends an event array. An aggregate callback must return a non-nil slice or array. Use `BatchOptions{Individual: true}` to invoke callbacks sequentially for each event.

## Format AppSync scalar values

```go
value, err := appsyncgraphql.AWSDateTime(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
```

**Result**

```text
2026-10-01T00:00:00.000Z
```

`AWSDate`, `AWSTime`, `AWSTimestamp` and `MakeID` cover other common scalar values.

## Things to know

- Return a resolver value, not an HTTP proxy response.
- Send the full AppSync resolver envelope; arguments and field names alone are not enough.
- Missing routes raise `ResolverNotFoundException`; invalid aggregate results raise `InvalidBatchResponseException`.

## Next steps

See the [API and behavior notes](reference/APPSYNC_GRAPHQL.md) for advanced options, and [module installation](MODULES.md) for dependencies.
