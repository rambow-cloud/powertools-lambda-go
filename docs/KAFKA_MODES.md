---
description: "Understand Kafka Lambda source and JSON event modes, payload decoding and Go Powertools compatibility boundaries."
---

# Kafka event format contracts

The Go consumer follows the pinned TypeScript v2.35.0 explicit field configuration. `EventRecordFormat` is an event source mapping setting, not a field required by the consumer. Key and value configurations are independent. No schema registry client is introduced.

## Lambda delivery boundary

Lambda validates selected key/value attributes. In JSON mode, those attributes contain Base64-encoded JSON, while their schema metadata still describes the original format. Unselected attributes retain their original representation. In SOURCE mode, validated attributes contain Base64-encoded source bytes with producer metadata removed. See the [AWS payload contract](https://docs.aws.amazon.com/lambda/latest/dg/services-consume-kafka-events.html#services-consume-kafka-events-payload-format).

| Delivered field | Go field configuration |
| --- | --- |
| JSON bytes, including converted Avro/Protobuf | `&kafka.FieldConfig{Type: kafka.JSON}` |
| Plain text | Leave the field configuration nil |
| Avro source bytes | `kafkaavro.New(schema)` |
| Protobuf source bytes | `kafkaproto.New(message)` with the supplied schema metadata |

Do not choose Avro decoding solely because `dataFormat` is `AVRO`: that metadata also appears on converted JSON. Do not run an Avro binary decoder over a JSON-mode value. The core preserves metadata and passes it to a selected binary decoder; it does not infer a decoder from metadata or fetch missing schemas.

The pinned Protobuf adapter has additional prefix behavior: absent metadata fails; an existing metadata object without `schemaId` supplies the whole buffer; a long ID selects the Glue prefix path; shorter IDs select adaptive Confluent index handling. These are verified upstream behaviors, not a claim that every service configuration emits an identical prefix. See [KAFKA_BINARY.md](KAFKA_BINARY.md).

## Differential evidence

The core generator adds 22 cases to its original 143: both event source names, each original schema format, key-only/value-only/both conversion, events without registry metadata and no implicit codec inference. Expected values come from executing the installed v2.35.0 CommonJS consumer, including retained metadata, original values, repeated lazy reads and errors. Payloads are constructed examples, not live service captures.

The optional adapters separately compare 370 Avro cases, 220 Protobuf prefix cases and nine native Protobuf message cases. Those corpora cover binary decoding and supplied metadata but do not establish end-to-end registry validation or producer framing removal.

The integration development module adds [172 mixed event scenarios](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/integration/kafkamodes/testdata/README.md). Every event runs directly and through the actual Go Lambda SDK handler, comparing repeated reads, parser calls, complete errors, metadata and original fields. Both sources, SOURCE/JSON formats, Glue/Confluent metadata and every text/JSON/Avro/Protobuf key/value pair are covered. These constructed events validate consumer behavior without claiming service delivery or producer framing removal. Missing registry metadata remains an upstream Protobuf error; callers can explicitly supply an empty metadata object for a plain message, as verified separately.

## Native Go Lambda input

Use `kafka.WrapHandler` with the Go Lambda SDK. It accepts `json.RawMessage` before any record fields can be discarded:

```go
consumer := kafka.New(kafka.Config{
    Key: kafkaavro.New(keySchema),
    Value: kafkaproto.New(kafkaproto.FromDescriptor(messageDescriptor)),
})
lambda.Start(kafka.WrapHandler(consumer,
    func(ctx context.Context, event *kafka.ConsumerRecords) (int, error) {
        for _, record := range event.Records {
            if _, err := record.Key(ctx); err != nil {
                return 0, err
            }
            if _, err := record.Value(ctx); err != nil {
                return 0, err
            }
        }
        return len(event.Records), nil
    }))
```

The pinned aws-lambda-go v1.55.0 `events.KafkaRecord` has no `keySchemaMetadata` or `valueSchemaMetadata` fields. Its string key/value fields also use `omitempty`, so a JSON round trip collapses missing, empty and null values. Passing that already-decoded type into `Deserialize` cannot recover discarded information. A regression test records this SDK boundary. RawMessage input preserves it for both MSK and self-managed events, including unknown fields; no duplicate event model or mandatory SDK dependency is needed in the consumer.

Remaining work includes service-captured events and actual delivery/retry acceptance when authorized, full binary schema compatibility and JavaScript-native type/serialization boundaries. KAF-MODES remains open for service-backed examples.
