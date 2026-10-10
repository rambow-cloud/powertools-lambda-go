---
description: "Understand typed Go parsing contracts for stream and notification Lambda events, payload transformations and validation boundaries."
---

# Stream and notification Parser contracts

Reference: installed TypeScript Parser v2.35.0 with Zod v4.1.12. The source audit covers the runtime exports of `schemas/sns`, `schemas/dynamodb`, `schemas/kinesis`, `schemas/kinesis-firehose` and `schemas/cloudwatch`, their envelopes, and `helpers/dynamodb`. These initial implementations remain subject to the full edge/error parity gate in PARSER_PLAN.md.

## Schema exports

The following names are exported unchanged from the Go `parser/schemas` package. Supporting local variables in the TypeScript source are not invented as public Go exports.

| Source family | Go exports | Main behavior |
| --- | --- | --- |
| SNS | SnsNotificationSchema, SnsSqsNotificationSchema, SnsRecordSchema, SnsSchema | Notification metadata, URL/timestamp checks, SNS casing differences and nonempty records |
| DynamoDB | DynamoDBStreamChangeRecordBase, DynamoDBStreamChangeRecord, DynamoDBStreamRecord, DynamoDBStreamSchema | Raw base versus decoded Keys/NewImage/OldImage, event enums and tumbling-window fields |
| DynamoDB to Kinesis | DynamoDBStreamToKinesisChangeRecord, DynamoDBStreamToKinesisRecord | Omitted stream-only fields, required recordFormat/tableName and nullable userIdentity |
| DynamoDB identity | UserIdentity | Service identity and dynamodb.amazonaws.com principal |
| Kinesis | KinesisDataStreamRecordPayload, KinesisDataStreamRecord, KinesisDataStreamSchema, KinesisDynamoDBStreamSchema | Base64 JSON/text/gzip decoding, window fields and embedded DynamoDB records |
| Firehose | KinesisFirehoseRecordSchema, KinesisFirehoseSqsRecordSchema, KinesisFirehoseSchema, KinesisFirehoseSqsSchema | Positive arrival times, decoded UTF-8 text or decoded SQS records |
| CloudWatch | CloudWatchLogEventSchema, CloudWatchLogsDecodeSchema, CloudWatchLogsSchema | Log metadata, nonempty decoded log events and Base64/gzip envelope decoding |

Kinesis ordinary stream events require at least one record. The separate KinesisDynamoDBStreamSchema accepts an empty record list in the pinned reference. DynamoDB tumbling state values are strings; Kinesis state values may contain arbitrary values. SNS notifications require the lower-case `UnsubscribeUrl` field, while notifications delivered through SQS omit that requirement and accept an optional upper-case `UnsubscribeURL` string. These differences are preserved.

## Envelope mapping

| TypeScript | Go constructor | Payload presented to the application schema |
| --- | --- | --- |
| SnsEnvelope | envelopes.SNS | Each Sns.Message string |
| SnsSqsEnvelope | envelopes.SNSSQS | Message string after decoding and validating the notification JSON in each SQS body |
| KinesisEnvelope | envelopes.Kinesis | Decoded JSON value or text from kinesis.data |
| KinesisFirehoseEnvelope | envelopes.KinesisFirehose | Decoded data text; use JSONStringified for JSON payloads |
| CloudWatchEnvelope | envelopes.CloudWatch | Each message string from decompressed awslogs.data.logEvents |
| DynamoDBStreamEnvelope | envelopes.DynamoDBStream | Each decoded NewImage/OldImage, returned as Images[T] pairs |

Ordinary parsing stops at the first invalid application payload. Safe parsing collects failed records; DynamoDB safe parsing also collects both invalid images within a record. Paths include the record index and the original source path. The shared private record traversal does not duplicate schema decoding or assume every service body contains JSON.

`Images[T]` has optional `*T` NewImage and OldImage fields. Missing images remain absent in JSON output; every input record still contributes one image pair. An empty present image is validated. `DynamoDBStreamChangeRecordBase` retains raw attributes so applications can extend selected fields with `parser.DynamoDBMarshalled`. Extending that base does not implicitly decode other fields. The decoded schemas explicitly apply a shared transformation after base validation.

## Shared DynamoDB conversion

`parser.DynamoDBMarshalled(schema)` converts a raw attribute map and validates the decoded result. Conversion failures become a custom Parser issue; application-schema issues retain their own paths. Number conversion preserves large integers as `*big.Int`, raw B values remain unchanged and set values use deduplicated Go slices.

The pure conversion now lives in root Commons as `UnmarshallDynamoDB`, `DynamoDBNumber` and `DynamoDBAttributeError`. The existing `commons/dynamodb` module delegates its raw conversion and number API, aliases the error type and retains native AWS SDK decoding. Parser therefore reuses existing conversion logic while its dependency graph remains SDK-free. Neither a new module nor a second decoder was added.

## Evidence and boundaries

Local acceptance on 2026-09-15 passed all 18 packaged modules, 15 standalone consumers, both CGO-disabled Lambda architecture builds, 205/205 Docker assertions and 14/14 Batch artifact checks. The Lambda emulator executed the six new envelopes and the large-integer helper. Root Commons and Parser have no external module dependencies. See MODULE_ACCEPTANCE.json, LOCAL_ACCEPTANCE.json and BATCH_ACCEPTANCE.json. Disposable containers and their internal network were removed; no AWS deployment was performed.

`generate-parser-streams.mjs` produces 106 cases from the actual reference packages: valid/missing inputs for all 22 schema exports, ordinary/safe envelope behavior, invalid records/images, missing images, empty-list differences, notification casing, timestamps/URLs, window fields, Base64/gzip inputs, SQS-in-Firehose and large DynamoDB integers. The existing forty core cases continue to test SQS traversal after consolidation. Additional Go tests cover schema omission/extension isolation, pipeline/refinement mode preservation, callback errors and concurrent reuse.

Go exposes ParseError consistently for built-in validation failures; some upstream ordinary envelope paths instead leak ZodError. The differential test compares structured issues, not JavaScript exception class identity or complete Zod error trees. JSON decoder syntax suffixes are normalized. BigInt and non-finite numeric values use the shared fixture representation; TypeScript Sets map to Go slices. URL validation currently uses Go's absolute URL parser, and exhaustive WHATWG URL, malformed UTF-8, permissive Buffer and malformed DynamoDB attribute behavior remain open. Passing these cases does not certify every Zod schema or invalid-input edge.
