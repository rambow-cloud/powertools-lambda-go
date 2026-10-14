---
description: "Decode Kafka Lambda events in Go with Powertools JSON handling, headers and optional Avro or Protobuf adapters."
---

# Kafka consumer

Decode key, value and header data in Lambda Kafka events, with optional Avro and Protobuf adapters.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/kafka@v1.2.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## Read a JSON order

This local program decodes and prints one JSON value. The Kafka value below is Base64-encoded `{"id":"ORD-123"}`; no broker or Lambda runtime is needed.

**Input**

One Kafka record whose Base64 value contains `{ "id": "ORD-123" }`.

??? example "Complete Lambda event"

    ```json
    {
      "eventSource": "aws:kafka",
      "records": {
        "orders-0": [
          {
            "topic": "orders",
            "partition": 0,
            "offset": 1,
            "value": "eyJpZCI6Ik9SRC0xMjMifQ==",
            "headers": []
          }
        ]
      }
    }
    ```

**Code**

```go
package main

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"log"

	"github.com/rambow-cloud/powertools-lambda-go/kafka"
)

func main() {
	ctx := context.Background()
	consumer := kafka.New(kafka.Config{Value: &kafka.FieldConfig{Type: kafka.JSON}})
	event := map[string]any{"records": map[string]any{"orders-0": []any{
		map[string]any{"value": "eyJpZCI6Ik9SRC0xMjMifQ=="},
	}}}
	records, err := consumer.Deserialize(ctx, event)
	if err != nil {
		log.Fatal(err)
	}
	value, err := records.Records[0].Value(ctx)
	if err != nil {
		log.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(encoded))
}
```

**Output**

```json
{
  "id": "ORD-123"
}
```

The output above is the decoded value returned by `record.Value(ctx)`, not the Lambda handler response. For a deployed Lambda, use `kafka.WrapHandler` and process the decoded values inside your callback; see the [complete handler](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/kafka/main.go).

## Read a record

```go
value, err := record.Value(ctx)
key, keyErr := record.Key(ctx)
headers, headerErr := record.Headers(ctx)
```

Handle each error before using the result. Metadata such as the topic and offset is available in `record.Fields`; original encoded values remain available separately.

## Choose a decoder

| Payload | Configuration |
| --- | --- |
| Text | Omit the field configuration |
| JSON | `&kafka.FieldConfig{Type: kafka.JSON}` |
| Avro | [Optional Avro adapter](KAFKA_BINARY.md) |
| Protobuf | [Optional Protobuf adapter](KAFKA_BINARY.md) |

Use [delivery modes](KAFKA_MODES.md) to choose SOURCE or JSON event-source settings. Register a parser callback when decoded payloads also need application validation.

## Things to know

- Values are decoded on access, not when the event is first received. Repeated reads decode again.
- A Kafka record with `value: null` is a tombstone; apply your application's deletion policy. Missing values are distinct from null.
- Invalid JSON emits a diagnostic and returns decoded text. Codec and parser errors return errors.
- This utility does not poll brokers, manage offsets, fetch registry schemas or generate partial-batch responses.

## Next steps

See the [API and behavior notes](reference/KAFKA.md) for advanced options, and [module installation](MODULES.md) for dependencies.
