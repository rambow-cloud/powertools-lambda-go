// Package kafka lazily deserializes AWS Lambda Kafka event records.
//
// [New] creates a [Consumer] with [Config] for key, value and header behavior.
// [WrapHandler] adapts the consumer to a JSON Lambda handler. [ConsumerRecords]
// retains event metadata and records expose decoded fields on demand using context.
//
// # Decoder composition
//
// Primitive and JSON modes are available in this module. Independent kafka/avro
// and kafka/protobuf modules supply optional binary field decoders; importing
// the core consumer does not load them. Custom decoders own their concurrency,
// result ownership and input validation.
//
// This package parses events delivered by Lambda; it does not poll brokers or
// manage consumer offsets. Lazy decoding failures retain typed consumer errors.
// The guide distinguishes supported raw and registry modes and compatibility limits.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/KAFKA.md
package kafka
