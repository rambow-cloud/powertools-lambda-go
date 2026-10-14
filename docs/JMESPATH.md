---
description: "Query JSON and extract Lambda event envelopes with Powertools Go JMESPath, compiled expressions."
---

# JMESPath

Select and transform values in JSON documents, including JSON/Base64 data inside Lambda event envelopes.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/jmespath@v1.2.0
```

## Select paid order IDs

This local program searches the `orders` array and returns only IDs whose status is `PAID`.

**Input**

```json
{
  "orders": [
    {
      "id": "ORD-123",
      "status": "PAID"
    },
    {
      "id": "ORD-456",
      "status": "PENDING"
    }
  ]
}
```

**Code**

```go
package main

import (
	"fmt"
	"log"

	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
)

func main() {
	data := map[string]any{"orders": []any{
		map[string]any{"id": "ORD-123", "status": "PAID"},
		map[string]any{"id": "ORD-456", "status": "PENDING"},
	}}
	value, err := jmespath.Search("orders[?status == 'PAID'].id", data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(value)
}
```

**Output**

```text
[ORD-123]
```

## Reuse an expression

```go
query, err := jmespath.Compile("orders[?status == 'PAID'].id")
if err != nil {
    return err
}
value, err := query.Search(data)
```

`value` is a Go array value, not a JSON string. Marshal it when your caller needs JSON.

## Decode an SQS body

```go
value, err := jmespath.ExtractDataFromEnvelope(event, jmespath.SQS)
```

**Input**

```json
{
  "Records": [
    {
      "body": "{\"id\":\"ORD-123\"}"
    }
  ]
}
```

**Result**

```json
[
  {
    "id": "ORD-123"
  }
]
```

`ExtractDataFromEnvelope` enables Powertools decoding functions automatically. For custom expressions, pass `WithPowertoolsFunctions()` to enable `powertools_json`, `powertools_base64` and `powertools_base64_gzip`.

## Things to know

- Compile reusable expressions once.
- Logger can use this optional module to extract correlation IDs; installing Logger alone does not install JMESPath.
- Bound compressed/untrusted payload sizes before decoding.

## Next steps

See the [API and behavior notes](reference/JMESPATH.md) for advanced options, and [module installation](MODULES.md) for dependencies.
