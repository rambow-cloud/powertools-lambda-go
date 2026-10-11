---
title: "Go Lambda data processing with Powertools"
description: "Process event batches, parse typed payloads, validate JSON, handle retries and decode Kafka data in Go AWS Lambda functions."
category: true
---

# Data processing

Turn incoming Lambda events into validated application data, and handle retries without repeating completed work. Choose utilities according to the event source and the operation your handler performs.

## Choose a utility

| Task | Guide |
| --- | --- |
| Process SQS, Kinesis or DynamoDB Streams records with partial failures | [Batch processing](../BATCH.md) |
| Prevent duplicate operation execution | [DynamoDB idempotency](../IDEMPOTENCY.md) or [Redis/Valkey persistence](../IDEMPOTENCY_CACHE.md) |
| Parse event payloads into typed values | [Parser](../PARSER.md) |
| Check inputs and responses against JSON Schema | [Validation](../JSON_SCHEMA_VALIDATION.md) |
| Query JSON and unwrap supported event envelopes | [JMESPath](../JMESPATH.md) |
| Decode Kafka payloads and preserve lazy codec boundaries | [Kafka](../KAFKA.md) |

## Compose a processing flow

Start from the Batch example when your handler receives records. Parse each record with an application-owned callback, validate the shape your application expects, and report failed records using the event source's documented response contract. Batch and Parser stay independently installable.

Use Idempotency around the operation whose repetition would duplicate a side effect. Review the persistence and expiration behavior in its guide instead of assuming every retry is safe. For Kafka, select the [source or JSON delivery mode](../KAFKA_MODES.md) before adding [Avro or Protobuf codecs](../KAFKA_BINARY.md).

See [Security and configuration](../security/index.md) for masking sensitive fields and loading application settings.
