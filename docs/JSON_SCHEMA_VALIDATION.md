---
description: "Validate Go Lambda inputs and responses against JSON Schema with Powertools, compiled validators and structured diagnostics."
---

# JSON Schema Validation

Validate payloads against JSON Schema documents. Check input before your business code and optionally validate its response.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/validation@v1.3.0
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

## Validate an order

Require a nonempty string `id`. The schema and payload are ordinary Go values; this program runs locally.

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
	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

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
	schema := map[string]any{
		"type":     "object",
		"required": []string{"id"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "minLength": 1},
		},
	}
	payload := map[string]any{"id": "ORD-123"}
	if _, err := validation.Validate(context.Background(), payload, schema, validation.Options{}); err != nil {
		return err
	}
	return appLog.Info("Order valid", logger.Fields{"order": payload})
}
```

The Logger timestamp varies on each run.

**Output**

```json
{
  "level": "INFO",
  "message": "Order valid",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "order": {
    "id": "ORD-123"
  }
}
```

## Reuse a compiled schema

Compile during initialization when the same schema handles many requests:

```go
validator, err := validation.Compile(ctx, schema, validation.Options{})
if err != nil {
    return err
}
value, err := validator.Validate(ctx, payload)
```

With `{"id":"ORD-123"}`, validation returns the payload. A missing or empty `id` returns a `SchemaValidationError`.

## Validate an embedded payload

```go
validator, err := validation.Compile(ctx, schema,
    validation.Options{Envelope: "detail"})
```

**Input**

```json
{
  "source": "orders",
  "detail": {
    "id": "ORD-123"
  }
}
```

**Validated value**

```json
{
  "id": "ORD-123"
}
```

## Choose a schema dialect

| Dialect | Configuration |
| --- | --- |
| Draft 7 | Default compiler |
| Draft 2019-09 | `Options{Compiler: validation.Draft201909Compiler{}}` |
| Draft 2020-12 | `Options{Compiler: validation.Draft202012Compiler{}}` |

See [compiler options](reference/JSON_SCHEMA_VALIDATION.md#optional-draft-2020-12-compiler) for injection examples. [Keywords](VALIDATION_KEYWORDS.md), [references](VALIDATION_REFERENCES.md) and [strict mode](VALIDATION_STRICT.md) cover schema configuration.

## Things to know

- Validation does not add defaults, coerce types or delete extra properties by default.
- Use `WrapHandler` to validate inbound/outbound values in a Lambda handler.
- Use [Parser](PARSER.md) when you want typed Go schema composition and event transformations.

## Next steps

See the [API and behavior notes](reference/JSON_SCHEMA_VALIDATION.md) for advanced options, and [module installation](MODULES.md) for dependencies.
