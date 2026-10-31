---
description: "Parse Go Lambda events into typed values and validate handler inputs using Powertools Parser and documented event models."
---

# Parser

Validate incoming data and convert it into typed Go values. Use built-in event schemas and envelopes for Lambda payloads.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/parser@v1.3.1
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.1
```

## Parse an order

Describe the input shape, then use `Typed[Order]` to obtain a Go struct. This example runs locally.

**Input**

```json
{
  "id": "ORD-123"
}
```

**Code**

```go
package main

import (
	"context"
	"os"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

type Order struct {
	ID string `json:"id"`
}

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	if err := run(appLog); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func run(appLog *logger.Logger) error {
	schema := parser.Typed[Order](parser.Object(
		parser.Field{Name: "id", Schema: parser.String()},
	))
	order, err := parser.Parse(context.Background(), map[string]any{"id": "ORD-123"}, schema)
	if err != nil {
		return err
	}
	return appLog.Info("Order parsed", logger.Fields{"order": order})
}
```

The Logger timestamp varies on each run.

**Output**

```json
{
  "level": "INFO",
  "message": "Order parsed",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "order": {
    "id": "ORD-123"
  }
}
```

## Handle invalid input without throwing

Run this snippet in a function returning `error`, using `ctx`, `schema` and `appLog`.

```go
result, err := parser.SafeParse(ctx, map[string]any{"id": 123}, schema)
if err != nil {
    return err
}
return appLog.Info("Order checked", logger.Fields{"success": result.Success})
```

**Output**

```json
{
  "level": "INFO",
  "message": "Order checked",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "success": false
}
```

`result.Error` contains the validation issues, including the `id` path. An operational failure still returns an error.

## Choose how to handle extra fields

| Schema option | Input `{"id":"ORD-123","debug":true}` |
| --- | --- |
| Default `Object` | Removes `debug` from the validated object |
| `PreserveUnknown` | Keeps the extra field in the validated object |
| `RejectUnknown` | Returns a validation issue |

Use `Field.Optional` for an absent field and `Nullable` for explicit JSON null. They are different cases.

## Parse Lambda event envelopes

Choose a guide for the event you receive:

- [SQS, Kinesis and DynamoDB Streams](PARSER_STREAMS.md).
- [API Gateway and HTTP](PARSER_HTTP.md).
- [Cognito and identity events](PARSER_IDENTITY.md).
- [S3, EventBridge and other services](PARSER_SERVICES.md).

`WrapHandler` validates before business code runs. `WrapSafeHandler` lets your handler decide what to do with a failed result. See [handling errors](PARSER_ERRORS.md).

## Things to know

- Construct reusable schemas outside the handler.
- Use explicit `json` tags; typed conversion uses case-sensitive JSON names.
- Use [JSON Schema Validation](JSON_SCHEMA_VALIDATION.md) when your contract is a JSON Schema document rather than a schema built in Go.

## Next steps

See the [API and behavior notes](reference/PARSER.md) for advanced options, and [module installation](MODULES.md) for dependencies.
