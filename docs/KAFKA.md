# Kafka consumer

Kafka deserializes Lambda Kafka events into records with lazy key/value/header methods. Import `github.com/rambow-cloud/powertools-lambda-go/kafka`. It does not poll brokers, commit offsets or fetch schemas from a registry.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Complete example

Build the complete example at `./examples/kafka` with `CGO_ENABLED=0`. It configures JSON values, calls `Value(ctx)` for every record, and leaves the business operation explicit.

~~~go
--8<-- "examples/kafka/main.go"
~~~

## Input and output

For the input below, `record.Value(ctx)` returns `map[string]any{"id":"ORD-123"}`. The example returns `nil`, encoded as JSON null by Lambda, and writes no successful application log. Merely creating `consumer` or receiving a record does not decode its value. An invalid JSON string produces a diagnostic and falls back to decoded text; binary codec/parser failures return errors. This wrapper has no partial-batch response or offset-management API.

~~~json
{
  "eventSource": "aws:kafka",
  "records": {
    "orders-0": [
      {
        "topic": "orders",
        "partition": 0,
        "offset": 1,
        "timestamp": 1790812800000,
        "timestampType": "CREATE_TIME",
        "value": "eyJpZCI6Ik9SRC0xMjMifQ==",
        "headers": []
      }
    ]
  }
}
~~~

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `consumer` | Reusable decoding configuration; optional Avro/Protobuf decoders are supplied explicitly. |
| `ConsumerRecords` | Flattened records plus retained event fields. |
| `record` | `Key(ctx)`, `Value(ctx)` and `Headers(ctx)` decode on each read; originals and metadata are separately available. |

## TypeScript feature coverage

Compared with the [official v2.35.0 kafka guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/kafka.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| ESM SOURCE / JSON / no registry | `Config`, schema metadata and codec options | Delivery-mode details in [Kafka modes](KAFKA_MODES.md); no registry lookup. |
| Primitive / JSON key and value | `FieldConfig`, `Record.Key/Value` | Lazy decoding; null/absence remain distinct. |
| Avro / Protobuf | Optional `kafka/avro`, `kafka/protobuf` modules | Native/metadata format boundaries documented in [binary formats](KAFKA_BINARY.md). |
| Headers / record metadata | `Headers`, original values and `Fields` | Retains metadata; header decoding differs from Parser. |
| Additional parsing | `FieldConfig.Parser` | Explicit synchronous validator callback; issues become ParserError. |
| Idempotency integration | Decode then execute Idempotency on payload | Implemented by the local fixture; no implicit identity from lazy record objects. |
| Errors / troubleshooting | Consumer error hierarchy and diagnostic callback | JSON fallbacks, tombstones and adapter failures are documented below. |

Executable evidence: [kafka/reference_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/kafka/reference_test.go), [kafka/avro/avro_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/kafka/avro/avro_test.go), [kafka/protobuf/protobuf_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/kafka/protobuf/protobuf_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.

## Native Lambda usage

```go
consumer := kafka.New(kafka.Config{
    Value: &kafka.FieldConfig{Type: kafka.JSON},
})
handler := kafka.WrapHandler(consumer, func(ctx context.Context, event *kafka.ConsumerRecords) (any, error) {
    for _, record := range event.Records {
        value, err := record.Value(ctx)
        if err != nil {
            return nil, err
        }
        if value == nil {
            continue // Tombstone: apply the application's deletion policy.
        }
        // Process value and record.Fields["offset"] here.
    }
    return nil, nil
})
lambda.Start(handler)
```

Complete imports are in [the example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/kafka/main.go). The wrapper receives `json.RawMessage`, preserves topic insertion order, checks the event and creates records, then passes the original context to the handler. Application response values and errors propagate unchanged. It does not generate partial batch responses or manage Kafka offsets.

## Public mapping

| TypeScript surface | Go surface | Behavior |
| --- | --- | --- |
| `kafkaConsumer(handler, config)` | `New(Config)`, `WrapHandler`, `Consumer.Deserialize` | No payload decoding before handler entry |
| `SchemaType.JSON/AVRO/PROTOBUF` | `JSON`, `Avro`, `Protobuf` | JSON is built in; Avro and Protobuf use optional adapter modules |
| `record.key/value/headers` getters | `Record.Key/Value/Headers(ctx)` | Decode on every access; no result/error caching |
| Original key/value/headers | `OriginalKey/OriginalValue/OriginalHeaders` | Retain encoded fields; missing fields use `Undefined{}` |
| Schema metadata | `KeySchemaMetadata/ValueSchemaMetadata` | Retained without registry access |
| Event and record metadata | `ConsumerRecords.Fields`, `Record.Fields` | Unknown fields survive; event records are flattened |
| `parserSchema` Standard Schema | `FieldConfig.Parser` and `ParseResult` | Synchronous transformation; non-nil issues mean failure, including empty slices |
| Consumer error hierarchy | `ConsumerError`, `DeserializationError`, `MissingSchemaError`, `ParserError` | `errors.As` matches the base or concrete type; `ErrorName` supplies the reference name |

Missing values use `Undefined{}`; null uses nil. An empty encoded key is undefined, while an empty encoded value is an empty string. Neither absent fields nor tombstones invoke the codec or parser. A present configuration with no type follows the upstream runtime fallthrough and produces undefined for non-null input; leave the field configuration nil for ordinary text decoding.

Primitive/JSON fields use strict Commons Base64 validation and TextDecoder-style UTF-8, including removal of one leading BOM. Headers use Buffer-style byte coercion and UTF-8 replacement, retaining a BOM. This differs from Parser's Kafka header code-point conversion, so the consumer must not delegate header handling to that schema.

JSON parse failures emit `Failed to parse JSON from base64 value: ...` on each read and return the decoded text. The default diagnostic writes to stderr. Inject `Config.Diagnostic` to route messages through an application logger. Callbacks must be safe for concurrent invocations. Parser callback errors propagate unchanged; validation issues become `ParserError.Issues`.

RawMessage preserves JSON topic insertion order and numeric property ordering. A native `map[string]any` uses sorted nonnumeric keys because Go maps have no insertion order. Native JSON event structs can also be passed to `Deserialize`; they are snapshotted using their JSON tags. Fields and originals are shallow references for native maps: nested mutations are visible, while replacing an original top-level property after deserialization does not replace the captured getter input. Treat retained inputs as immutable during concurrent access.

## Composition and dependency boundaries

An application can call a Parser schema's `Validate` method inside the parser callback and convert its issues into `[]any`. The Kafka module has no Parser dependency. Unexpected validator errors should be returned unchanged. After a successful `Value` read, pass the decoded payload to `idempotency.Execute` or an idempotent handler; do not use the complete lazy Record as an implicit idempotency key. The local integration probe demonstrates validation before persistence, duplicate suppression and shared logging/tracing context.

Binary formats require an explicit `FieldConfig.Decoder`. Its arguments are the original context, encoded string, schema, and captured schema metadata. The core reports a missing adapter during event preparation, even if the field is absent; an empty event invokes no codec selection. With an adapter present, missing schema errors occur only on non-null field access. Use kafka/avro.New or kafka/protobuf.New to supply the implemented optional adapter; the core does not import either dependency.

## Remaining differences

- Avro/Protobuf adapters and scoped registry-prefix handling are implemented separately. Full native/schema/error parity and actual service mode acceptance remain open.
- Go configuration fields are copied at construction; arbitrary live mutation of JavaScript configuration objects is not reproduced. Supplied schemas and callbacks remain application-owned.
- Native JavaScript prototypes, arbitrary array-like objects, asynchronous validators, property descriptors and lone UTF-16 surrogates have no complete Go mapping. Selected malformed inputs are tested; exhaustive coercion remains open.
- JSON diagnostics retain Go's underlying JSON error rather than Node's exact SyntaxError detail/stack. Plain returned errors retain Go's runtime type names; `ErrorName` is an explicit API, not an override of aws-lambda-go Runtime API error serialization.
- Record methods make lazy failures explicit. Default Go JSON marshaling of the consumer structures is not a replacement for evaluating all JavaScript getters.
- Functional concurrency checks run with CGO disabled; no race-detector equivalence, live Kafka/AWS acceptance, performance budget or published release is claimed.

Reference evidence: [165 scenarios](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/kafka/testdata/README.md), a complete 28-module/25-consumer baseline with subsequent affected-module checks, CGO-disabled Linux amd64/arm64 builds, 829/829 local RIE assertions, 95/95 streaming checks and 14/14 Batch artifact checks. Docker executed amd64; arm64 was cross-compiled. No AWS resources were used. Fixed-version implementation sources are installed by the reference package lock; no upstream code is patched to generate expected values.

The 22 additional JSON-mode scenarios passed a subsequent core-only packaged check. The later Protobuf metadata fix and 172 mixed events passed affected-module checks and rebuilt Docker acceptance. See [KAFKA_MODES.md](KAFKA_MODES.md).
