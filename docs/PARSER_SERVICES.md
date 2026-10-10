---
description: "Parse supported AWS service and Kafka Lambda events into typed Go values with documented Powertools Parser contracts."
---

# Parser service events and Kafka

Reference: Powertools TypeScript v2.35.0 and Zod v4.1.12, as installed by the pinned development fixture project. This slice implements fifteen runtime schema exports from six additional source families. It does not implement the separate Kafka consumer utility, schema registries, Avro or Protobuf.

| Source family | Go exports in `parser/schemas` |
| --- | --- |
| Kafka | `KafkaRecordSchema`, `KafkaMskEventSchema`, `KafkaSelfManagedEventSchema` |
| CloudFormation | `CloudFormationCustomResourceCreateSchema`, `CloudFormationCustomResourceDeleteSchema`, `CloudFormationCustomResourceUpdateSchema` |
| Transfer Family | `TransferFamilySchema` |
| Connect outbound campaigns | `ConnectOutboundCampaignsCustomerProfileSchema`, `ConnectOutboundCampaignsSchema` |
| SES | `SesRecordSchema`, `SesSchema` |
| S3 | `S3Schema`, `S3SqsEventNotificationSchema`, `S3EventNotificationEventBridgeSchema`, `S3ObjectLambdaEventSchema` |

## Kafka extraction

`envelopes.Kafka(payload)` validates MSK or self-managed Kafka metadata, decodes Base64 key/value fields, and flattens topic-partition records into a typed slice. Values are text after decoding. Use `parser.JSONStringified(payload)` explicitly for JSON messages.

```go
orders, err := parser.Parse(ctx, json.RawMessage(eventJSON),
    envelopes.Kafka(parser.JSONStringified(orderSchema)))
```

Use `json.RawMessage` as the native Lambda handler input when source JSON property order matters; see [the runnable Kafka example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/parser/kafka/main.go). Numeric array-index keys enumerate first, in ascending order; other keys retain their JSON insertion order. Repeated JSON keys retain their first position and last value. Go maps have no insertion order: their JSON representation supplies deterministic sorted keys before numeric-index ordering is applied. The implementation cannot recover an order discarded by decoding into a map.

Outer records are validated before application payloads. Ordinary parsing stops at the first invalid payload and retains its unprefixed issue paths. Safe parsing aggregates all invalid payloads and prefixes paths with `records` and the topic key; the reference does not include the record index or `value`. Overall validation failure returns no partial success data. The original input is retained by safe parsing. Custom Go payloads can implement `SafeSchema` to select safe validation, including non-nil empty issue slices. Cancellation, operational errors and panics retain the standard Go Parser contract.

An empty records map is valid, but every present topic group must contain at least one record. The pinned schema rejects null tombstones. Bootstrap server splitting preserves whitespace and empty entries. Kafka Base64 decoding accepts URL-safe symbols, missing padding and ignored non-alphabet characters as the reference Buffer decoder does. Header arrays represent Unicode code points, not UTF-8 bytes; supplementary code points and adjacent UTF-16 surrogate pairs produce the same output as the reference.

## Service-specific behavior

- CloudFormation validates an absolute response URL, resource properties and the Create/Delete/Update discriminator. Update requires old properties. Update/Delete require and preserve `PhysicalResourceId` so parsed handlers retain the existing resource identifier. Create does not require it. This intentionally corrects the pinned TypeScript v2.35.0 omission against the [AWS request contract](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/crpg-ref.html).
- Transfer requires the password and an IPv4 source address; the protocol is an unrestricted string in the reference.
- Connect permits an empty customer-profile array and preserves customer data as a string.
- SES validates verdicts, policies, timestamps and the Lambda action shape. Receipt `dmarcPolicy` is optional, Lambda actions accept `Event` and `RequestResponse`, and common email header members are optional but validate their types when present. The documented `replyTo` array is retained; legacy `reply-to` input remains accepted. These changes deliberately correct the pinned TypeScript v2.35.0 schema against [AWS receiving notification contents](https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html) and [Lambda action modes](https://docs.aws.amazon.com/ses/latest/APIReference/API_LambdaAction.html). Processing time must be a positive integer within JavaScript's safe integer range. Fractional, zero, negative and out-of-range cases retain the reference issue fields.
- S3 notification keys are preserved without implicit URL decoding. Ordinary notification sizes are unrestricted numbers; the EventBridge variant requires nonnegative sizes. The source address accepts IPv4 or `s3.amazonaws.com`. S3-in-SQS reuses `SqsRecordSchema` and `JSONStringified`; EventBridge reuses `EventBridgeSchema`.
- Object Lambda accepts boolean or `"true"`/`"false"` MFA values and converts them to booleans. Its configuration payload accepts a string or an object whose unknown properties are stripped, matching the pinned schema.

## Reference evidence and boundaries

`node generate-parser-services.mjs` writes 270 actual reference cases: every listed export's valid/missing/null inputs and top-level field omissions/type changes, both Kafka sources and parse modes, key ordering, aggregation, buffer/header transformations, malformed UTF-8, integer limits and selected nested service fields. This milestone brought Parser to 874 differential cases; current totals are recorded in PARSER.md. Focused Parser tests also cover concurrent schema reuse, input preservation, duplicate JSON keys, native-map ordering, cancellation, callback error/panic identity, malformed input and safe-schema dispatch.

Two exception mappings are explicit in the test harness. Safe Kafka parsing of null throws a JavaScript `TypeError`; Go returns an operational error. Invalid header code points cause Zod's Standard Schema adapter to return a rejected Promise; Powertools rejects asynchronous validation synchronously. The generator attaches a rejection observer without changing that Promise or the Powertools result. It requires exactly the three expected `RangeError` rejections and records their causes separately. Go returns the code-point error directly. These cases are not reported as successful validation or normalized into ordinary validation issues.

The stream and Kafka text decoders share maximal-subpart UTF-8 replacement logic. Lone UTF-16 surrogates still lack an equivalent Unicode scalar representation in Go strings, and complete malformed encoding coverage remains open. Remaining union semantics and constraint metadata, all nested field permutations, every declaration/type mapping, performance budgets and live event-source behavior also remain open. AppSync/shared, AppSync Events and Cognito were subsequently implemented under [PARSER_IDENTITY.md](PARSER_IDENTITY.md). All fourteen envelope families have implementations, with exhaustive compatibility still tracked separately.

Acceptance on 2026-09-15 passed all 18 independently packaged modules and 15 standalone consumers, both CGO-disabled Linux builds, 250/250 Docker assertions and 14/14 Batch checks over the same artifacts. The runtime executed amd64; arm64 was cross-compiled only. The suite adds 24 service/Kafka checks across three successful invocations, including typed topic order, safe paths, CloudFormation stripping, Transfer IPv4, empty Connect profiles, SES receipt fields, S3 key/size preservation and Object Lambda transformations. Containers and network were cleaned without AWS access. See [PARSER_PLAN.md](PARSER_PLAN.md) and [LOCAL_VALIDATION.md](LOCAL_VALIDATION.md).
