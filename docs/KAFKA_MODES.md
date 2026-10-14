---
description: "Understand Kafka Lambda source and JSON event modes, payload decoding and Go Powertools compatibility boundaries."
---

# Kafka delivery modes

Choose your decoder to match the event source mapping. Lambda can deliver original binary data in SOURCE mode or convert registry-backed messages to JSON first.

## Read JSON-mode messages

**Input**

A JSON-mode Kafka record containing `{ "id": "ORD-123" }`.

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
            "valueSchemaMetadata": {
              "dataFormat": "JSON"
            },
            "headers": []
          }
        ]
      }
    }
    ```

**Code**

```go
consumer := kafka.New(kafka.Config{Value: &kafka.FieldConfig{Type: kafka.JSON}})
records, err := consumer.Deserialize(ctx, event)
if err != nil { return err }
value, err := records.Records[0].Value(ctx)
```

`event` is the decoded input above. Handle `err` before using `value`.

**Output**

```json
{
  "id": "ORD-123"
}
```

This is the decoded record value, not the Lambda handler response.

## Select a mode

| Lambda delivery | Consumer value configuration |
| --- | --- |
| JSON text, including registry-converted JSON | `FieldConfig{Type: kafka.JSON}` |
| SOURCE Avro data | Explicit [Avro schema/adapter](KAFKA_BINARY.md) |
| SOURCE Protobuf data | Explicit [Protobuf descriptor/adapter](KAFKA_BINARY.md) |
| Plain text without a schema | Omit the field configuration |

Configure key and value independently. Preserve the schema metadata supplied by Lambda; it is not a request to fetch a schema.

## Things to know

- Keep the Kafka wrapper's raw event entry point when metadata presence and numeric tokens matter; typed SDK event models can lose that information.
- A registry schema ID does not make the library a schema registry client.
- Self-managed Kafka and MSK events both use the record decoding APIs. Deployment configuration belongs to the event source mapping.

## Next steps

[API and behavior notes](reference/KAFKA_MODES.md) · [HTTP](HTTP.md) · [Modules](MODULES.md)
