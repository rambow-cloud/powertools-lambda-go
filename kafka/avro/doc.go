// Package avro decodes Avro binary fields for the Lambda Kafka consumer.
//
// [New] creates a Kafka field configuration from an Avro schema string; schema
// parsing and record decoding remain lazy. [Deserialize] can decode one Base64
// datum directly using the same adapter contract.
//
// # Input boundaries
//
// The decoder rejects truncated input, trailing data and unsafe long values.
// Schema-registry metadata does not alter Avro decoding in the pinned reference.
// The application selects the schema and configures event delivery; this adapter
// does not contact a schema registry or Kafka broker.
//
// This optional module keeps Avro dependencies out of the core Kafka consumer.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/KAFKA_BINARY.md
package avro
