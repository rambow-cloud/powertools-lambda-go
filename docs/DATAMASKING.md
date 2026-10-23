---
description: "Erase or transform selected Go Lambda payload fields with Powertools Data Masking, ordered rules and optional regex or KMS providers."
---

# Data Masking

Remove sensitive values before logging or returning a payload. Mask selected fields, or encrypt them with an optional provider.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/datamasking@v1.3.0
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

## Mask a customer field

Mask the payload before passing it to Powertools Logger. The original payload stays unchanged, so log the returned copy. This example needs no AWS credentials; its log timestamp varies on each run.

**Input**

```json
{
  "customer": {
    "name": "Ada",
    "ssn": "123-45-6789"
  },
  "order_id": "ORD-123"
}
```

**Code**

```go
package main

import (
	"context"
	"os"

	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
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
	payload := map[string]any{
		"customer": map[string]any{"name": "Ada", "ssn": "123-45-6789"},
		"order_id": "ORD-123",
	}
	masker := datamasking.New(datamasking.Config{})
	masked, err := masker.Erase(context.Background(), payload, datamasking.EraseOptions{
		Fields: []string{"customer.ssn"},
	})
	if err != nil {
		return err
	}
	if err := appLog.Info("Order received", logger.Fields{"order": masked}); err != nil {
		return err
	}
	return nil
}
```

**Output**

```json
{
  "level": "INFO",
  "message": "Order received",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "order": {
    "customer": {
      "name": "Ada",
      "ssn": "*****"
    },
    "order_id": "ORD-123"
  }
}
```

## Choose a custom mask

The default replacement is `"*****"`, regardless of the original string length. Set `CustomMask` to use `"[redacted]"` instead. The following snippets run independently inside a function returning `error`, with a context `ctx` and the `masker` and `appLog` created above. Log excerpts omit Logger's standard fields.

```go
replacement := "[redacted]"
masked, err := masker.Erase(ctx, payload, datamasking.EraseOptions{
    Fields:     []string{"customer.ssn"},
    Rule:       datamasking.Rule{CustomMask: &replacement},
})
if err != nil { return err }
return appLog.Info("Order received", logger.Fields{"order": masked})
```

**Output (log excerpt)**

```json
{
  "message": "Order received",
  "order": {
    "customer": {
      "name": "Ada",
      "ssn": "[redacted]"
    },
    "order_id": "ORD-123"
  }
}
```

## Mask every item in an array

This separate call omits `CustomMask`, so each email uses the default `"*****"`. A custom mask affects only the call that supplies it; it does not change the `masker`. Pass the same `Rule` here if you want `"[redacted]"` for emails too.

```go
payload := map[string]any{"customers": []any{
    map[string]any{"email": "ada@example.com"},
    map[string]any{"email": "grace@example.com"},
}}
masked, err := masker.Erase(ctx, payload, datamasking.EraseOptions{
    Fields: []string{"customers[*].email"},
})
if err != nil { return err }
return appLog.Info("Customers received", logger.Fields{"payload": masked})
```

**Output (log excerpt)**

```json
{
  "message": "Customers received",
  "payload": {
    "customers": [
      {
        "email": "*****"
      },
      {
        "email": "*****"
      }
    ]
  }
}
```

## Encrypt instead of erasing

Configure a `Provider` and use `Encrypt` / `Decrypt` with `TransformOptions`. The [KMS guide](DATAMASKING_KMS.md) shows authenticated encryption with the AWS Encryption SDK.

Use [regular-expression rules](REGEX.md) when you need to replace only part of a field.

## Things to know

- Masks are irreversible; a replacement string is not encrypted data.
- Missing selectors fail by default. `IgnoreMissing` warns and continues.
- Use `users.0.secret` or `users[*].secret` for arrays; `users[0].secret` is not an array-index selector.
- An omitted `Fields` slice masks the whole value; an explicitly empty slice selects no fields.

## Next steps

See the [API and behavior notes](reference/DATAMASKING.md) for advanced options, and [module installation](MODULES.md) for dependencies.
