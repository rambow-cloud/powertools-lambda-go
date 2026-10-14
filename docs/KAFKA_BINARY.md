---
description: "Decode Kafka Avro and Protobuf payloads with optional Powertools Go adapters and documented binary format contracts."
---

# Kafka Avro and Protobuf

Decode binary values with optional adapters. Your application supplies the schema or message descriptor; the adapters do not fetch schemas from a registry.

## Decode an Avro order

Install `kafka/avro@v1.2.0` and import it as `kafkaavro`. The Base64 value below encodes a record with one string field.

**Input**

```text
Dk9SRC0xMjM=
```

**Code**

```go
schema := `{"type":"record","name":"Order","fields":[{"name":"id","type":"string"}]}`
value, err := kafkaavro.Deserialize(ctx, "Dk9SRC0xMjM=", schema, nil)
if err != nil { return err }
encoded, err := json.Marshal(value)
if err != nil { return err }
fmt.Println(string(encoded))
```

Here `json` is `encoding/json/v2` and the surrounding function returns `error`.

**Output**

```json
{
  "id": "ORD-123"
}
```

For a Lambda consumer, use the same schema in the field configuration:

```go
consumer := kafka.New(kafka.Config{Value: kafkaavro.New(schema)})
```

## Decode a Protobuf message

Install `kafka/protobuf@v1.2.0` and import it as `kafkaproto`. This example uses the well-known `StringValue` descriptor from `wrapperspb` and `protojson` from the official Go Protobuf library.

```go
descriptor := (&wrapperspb.StringValue{}).ProtoReflect().Descriptor()
var decoder kafkaproto.Decoder
value, err := decoder.Deserialize(ctx, "CgVIZWxsbw==",
    kafkaproto.FromDescriptor(descriptor), map[string]any{})
if err != nil { return err }
encoded, err := protojson.Marshal(value.(proto.Message))
if err != nil { return err }
fmt.Println(string(encoded))
```

**Output**

```json
"Hello"
```

For consumer configuration use `kafkaproto.New(kafkaproto.FromDescriptor(descriptor))`. Use your own generated message descriptor for application messages.

## Things to know

- Avro records become Go maps; Protobuf descriptors return fresh `proto.Message` values.
- Use `protojson` for Protobuf JSON output; ordinary Go JSON is not its wire contract.
- Preserve Protobuf schema metadata. An empty metadata object selects the whole message buffer; a missing object is rejected.
- Avro rejects trailing/truncated data, excessive nesting and more than 1,048,576 cumulative collection entries per datum.
- Producer framing and Lambda event-source modes matter. See [delivery modes](KAFKA_MODES.md); do not assume arbitrary registry wire frames are supported.

## Next steps

[API and behavior notes](reference/KAFKA_BINARY.md) · [HTTP](HTTP.md) · [Modules](MODULES.md)
