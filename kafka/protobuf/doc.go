// Package protobuf adapts Protobuf message decoders to the Lambda Kafka consumer.
//
// [New] creates a Kafka field configuration from a [Message]. [FromDescriptor]
// supports generated or runtime descriptors through the official Go Protobuf
// implementation and returns fresh native messages. Custom [Message.Decode]
// callbacks receive the buffer and prefix position in [Input].
//
// # Wire and ownership boundaries
//
// [Decoder] handles the documented Confluent index conventions. Custom callbacks
// must validate input, be concurrency-safe and avoid mutable shared results.
// Native messages follow Go Protobuf semantics rather than protobufjs object
// defaults or JavaScript serialization.
//
// The application owns descriptors and schema-registry configuration; this adapter
// does not fetch schemas. This module is an optional companion to core Kafka.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/KAFKA_BINARY.md
package protobuf
