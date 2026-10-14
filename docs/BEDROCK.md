---
description: "Resolve Amazon Bedrock Agent function requests in Go Lambda with Powertools, typed inputs, routing and response contracts."
---

# Bedrock Agent function resolver

Implement functions for a Bedrock Agent Action Group and return tool results to the agent.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/bedrock@v1.2.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## Create a greeting tool

Configure a function-based Action Group with a `greeting` function and a string parameter named `name`.

**Input**

Call `greeting` with the string parameter `name=Ada`.

??? example "Complete Lambda event"

    ```json
    {
      "messageVersion": "1.0",
      "agent": {
        "name": "orders-agent",
        "id": "AGENT12345",
        "alias": "ALIAS12345",
        "version": "DRAFT"
      },
      "inputText": "Greet Ada",
      "sessionId": "example-session",
      "actionGroup": "orders",
      "function": "greeting",
      "parameters": [
        {
          "name": "name",
          "type": "string",
          "value": "Ada"
        }
      ],
      "sessionAttributes": {},
      "promptSessionAttributes": {}
    }
    ```

**Code**

```go
package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/bedrock"
)

func main() {
	app := bedrock.New(bedrock.Options{})
	app.Tool(func(ctx context.Context, params *bedrock.Parameters, event bedrock.Event) (any, error) {
		return bedrock.NewFunctionResponse(fmt.Sprintf("Hello, %v", params.Get("name"))), nil
	}, bedrock.Configuration{Name: "greeting"})
	lambda.Start(app.Resolve)
}
```

**Output**

```json
{
  "messageVersion": "1.0",
  "response": {
    "actionGroup": "orders",
    "function": "greeting",
    "functionResponse": {
      "responseBody": {
        "TEXT": {
          "body": "Hello, Ada"
        }
      }
    }
  },
  "sessionAttributes": {},
  "promptSessionAttributes": {}
}
```

`NewFunctionResponse` returns the text verbatim. Returning an ordinary Go string instead JSON-encodes it, including string quote characters.

## Read and return structured values

```go
app.Tool(func(ctx context.Context, params *bedrock.Parameters, event bedrock.Event) (any, error) {
    return map[string]any{"customer": params.Get("name"), "status": "ready"}, nil
}, bedrock.Configuration{Name: "lookupCustomer"})
```

The TEXT body contains `{"customer":"Ada","status":"ready"}` for a request with `name=Ada`.

## Manage the conversation

Use the event's session attributes for conversation state and `FunctionResponse` for custom response state. See [response options](reference/BEDROCK.md) for failures and reprompting.

## Things to know

- This resolver supports function-based Action Groups, not OpenAPI Action Groups.
- It routes tool invocations; it does not call a foundation model.
- Malformed UTF-8 in returned keys or values is an error. Return valid text rather than partially encoded bodies.

## Next steps

See the [API and behavior notes](reference/BEDROCK.md) for advanced options, and [module installation](MODULES.md) for dependencies.
