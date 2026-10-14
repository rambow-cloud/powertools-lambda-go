# Optional Kafka binary adapters: API and behavior notes

Start with the [usage guide](../KAFKA_BINARY.md) for code and results. See [compatibility and provenance](COMPATIBILITY.md) for contributor reference material.


Avro and Protobuf are separate modules, `kafka/avro` and `kafka/protobuf`. The Kafka core retains no third-party module dependencies. Neither adapter retrieves schemas or contacts a registry: the application supplies the schema or message decoder, and the Lambda event supplies metadata.

## Avro

```go
import (
    "github.com/rambow-cloud/powertools-lambda-go/kafka"
    kafkaavro "github.com/rambow-cloud/powertools-lambda-go/kafka/avro"
)

consumer := kafka.New(kafka.Config{
    Value: kafkaavro.New(`{"type":"record","name":"Order","fields":[{"name":"id","type":"string"}]}`),
})
```

Pass the consumer to the existing Kafka wrapper. Key configuration works identically. `New` creates a field configuration without parsing the schema. Each non-null field read parses and decodes independently, preserving lazy failures and result isolation. An empty schema triggers the core MissingSchemaError only when the field is accessed. Metadata does not change Avro decoding in the reference.

The adapter uses hamba/avro v2.31.0 for schema parsing and primitive binary reads. A local schema cache prevents unrelated named types from leaking across calls. The adapter owns traversal that retains tagged unions, byte slices, JavaScript numbers and ignored logical annotations; the library's default native decoder has different mappings. Context is checked during traversal. Byte/string allocation is bounded by encoded input length; nesting is capped at 1024 levels. A datum may contain at most 1,048,576 array/map entries in total, across all blocks and nesting. This deliberate Go resource bound also limits null/empty-record items that occupy no wire bytes; the pinned TypeScript decoder does not impose this count limit.

Records and maps return map[string]any; arrays return []any; bytes/fixed return []byte; enum/string return strings; numbers return float64. Unions retain an object keyed by the selected branch name, except null. The Avro long guard uses the reference's range of -9007199254740990 through 9007199254740990 after its floating-point zigzag calculation. That calculation can lose a sign bit near the boundary; 160 additional fixtures preserve actual upstream results instead of silently substituting exact Go int64 decoding.

Truncated buffers, trailing bytes, invalid union/enum indices and precision failures have scoped exact reference errors. Broader invalid-schema wording, overlong wire values, filesystem schema strings, prototype behavior and native serialization remain open. Standard Go JSON serializes byte slices as Base64, unlike avro-js Buffer JSON. Do not infer complete serialization parity from successful decoding.

## Protobuf

For an existing generated message descriptor:

```go
import (
    "github.com/rambow-cloud/powertools-lambda-go/kafka"
    kafkaproto "github.com/rambow-cloud/powertools-lambda-go/kafka/protobuf"
    "google.golang.org/protobuf/types/known/wrapperspb"
)

descriptor := (&wrapperspb.StringValue{}).ProtoReflect().Descriptor()
consumer := kafka.New(kafka.Config{
    Value: kafkaproto.New(kafkaproto.FromDescriptor(descriptor)),
})
```

FromDescriptor accepts generated or runtime descriptors and returns fresh proto.Message results using the official Go Protobuf decoder. Use message reflection, generated conversions or protojson according to the application contract. The adapter does not pretend that Go message defaults, unknown fields and serialization are identical to protobufjs objects. Proto2 optional presence and 64-bit/bytes/repeated/enum/nested values have direct differential evidence; full native/error mapping remains open.

Applications can instead supply `Message{Decode: func(Input) (any, error) {...}}`. Input exposes the original Buffer, current Position and whether the reference supplied an explicit whole-buffer length. This preserves the public caller-supplied decode boundary without forcing an extra object conversion. Description is optional JSON for error messages, defaulting to `{}`. Supplied callbacks and descriptions are application-owned and must be safe for concurrent calls.

An existing metadata object without schemaId decodes the whole buffer. Missing/null metadata is an error, matching the pinned implementation. A schema ID longer than ten UTF-16 code units selects Glue handling: consume one uint32 and then decode. Other IDs select Confluent handling: decode an int32 or sint32 index count, skip that many bytes, then invoke the supplied decoder. This is the reference's byte-skip algorithm; it does not decode each index as an independent varint.

The reference tests the length property rather than requiring a string ID. JSON array IDs therefore use their array length, and object IDs can supply a numerically coerced length (including a numeric string or nested single-element arrays). The adapter reuses Commons ParseNumber for that conversion; 97 additional cases compare the selected reader position and complete errors. Native cyclic array coercion is bounded at 1024 steps and custom JavaScript prototypes remain outside this mapping.

The first convention defaults to int32. If decoding fails, the alternate convention is tried; success updates the preference, while two failures report the first exception. New shares a process-wide atomic preference. For an explicitly isolated scope, construct `var decoder kafkaproto.Decoder` and use `decoder.Field(message)`. Do not copy a Decoder after use. Callbacks run without a held mutex and may reenter decoding. Concurrent calls snapshot their initial preference; exact JavaScript scheduling and arbitrary reentrant preference mutation remain open.

Malformed negative indices can move the reference reader before the buffer start. The callback Input preserves that position; the native descriptor helper rejects it rather than slicing outside Go bounds. Native library decoder error text is retained as the cause and inside DeserializationError; exact protobufjs wire-error text is not claimed.

