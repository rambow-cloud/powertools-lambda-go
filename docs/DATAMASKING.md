---
description: "Erase or transform selected Go Lambda payload fields with Powertools Data Masking, ordered rules and optional regex or KMS providers."
---

# Data Masking

Remove sensitive values before logging or returning a payload. Mask selected fields, or encrypt them with an optional provider.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/datamasking@v1.2.0
```

## Mask a customer field

The program returns a masked copy. It leaves the original payload unchanged and needs no AWS credentials.

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
	json "encoding/json/v2"
	"fmt"
	stdlog "log"

	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
)

func main() {
	payload := map[string]any{
		"customer": map[string]any{"name": "Ada", "ssn": "123-45-6789"},
		"order_id": "ORD-123",
	}
	masker := datamasking.New(datamasking.Config{})
	masked, err := masker.Erase(context.Background(), payload, datamasking.EraseOptions{
		Fields: []string{"customer.ssn"},
	})
	if err != nil {
		stdlog.Fatal(err)
	}
	encoded, err := json.Marshal(masked)
	if err != nil {
		stdlog.Fatal(err)
	}
	fmt.Println(string(encoded))
}
```

**Output**

```json
{
  "customer": {
    "name": "Ada",
    "ssn": "*****"
  },
  "order_id": "ORD-123"
}
```

## Choose a custom mask

```go
replacement := "[redacted]"
masked, err := masker.Erase(ctx, payload, datamasking.EraseOptions{
    Fields:     []string{"customer.ssn"},
    Rule:       datamasking.Rule{CustomMask: &replacement},
})
```

**Output**

```json
{
  "customer": {
    "name": "Ada",
    "ssn": "[redacted]"
  },
  "order_id": "ORD-123"
}
```

## Mask every item in an array

```go
masked, err := masker.Erase(ctx, payload, datamasking.EraseOptions{
    Fields: []string{"customers[*].email"},
})
```

For `{"customers":[{"email":"ada@example.com"},{"email":"grace@example.com"}]}`, the result is:

```json
{
  "customers": [
    {
      "email": "*****"
    },
    {
      "email": "*****"
    }
  ]
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
