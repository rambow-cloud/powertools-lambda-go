// Package envelopes validates Lambda event envelopes and extracts typed payloads.
//
// Construct an envelope schema around an application payload schema, then pass it
// to Parser's Parse or WrapHandler. [SQS] validates records and parses body strings;
// [EventBridge] validates metadata and selects detail. Other constructors support
// HTTP, stream, Kafka and service-specific envelopes.
//
// # Validation order
//
// Envelope schemas validate service metadata before delivering payloads to business
// code. Failures retain payload paths in Parser issues. Extraction and decoding
// differ by event family; the guide explains when to use a JSON-string schema or
// a schema for an already decoded value.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/PARSER.md
package envelopes
