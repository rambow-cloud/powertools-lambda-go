# Parser

Parser validates and transforms Lambda event data into typed Go values. Import `github.com/rambow-cloud/powertools-lambda-go/parser`; built-in schemas and envelopes are subpackages. It does not require Zod at runtime. The reference uses Powertools v2.35.0 and Zod v4.1.12.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Complete example

The complete Lambda example validates EventBridge metadata and an order in `detail`, then passes a typed order to the business handler. Build `./examples/parser` with `CGO_ENABLED=0`.

~~~go
--8<-- "examples/parser/main.go"
~~~

## Input and output

For the valid input below, the handler returns the JSON string `"ORD-123"`. It does not print a log record. Changing `amount` to `-1` produces a `ParseError` before business code runs. Its top-level message is `Failed to parse EventBridge envelope`; inspect its issues for the `detail.amount` path and `amount must be non-negative` message. The detailed issues are not automatically printed. An absent field differs from explicit JSON null. Use `SafeParse` or `WrapSafeHandler` when the application should decide how to respond to validation failures.

~~~json
{
  "version": "0",
  "id": "00000000-0000-4000-8000-000000000001",
  "detail-type": "OrderCreated",
  "source": "com.example.orders",
  "account": "123456789012",
  "time": "2026-10-01T00:00:00Z",
  "region": "ap-east-1",
  "resources": [],
  "detail": {
    "id": "ORD-123",
    "amount": 42
  }
}
~~~

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `payload` | Reusable schema; object fields, refinements and typed output are composed before serving requests. |
| `input` | Validated `order`, not the raw EventBridge event. |
| `Result[T]` / `ParseError` | Safe parsing keeps validation failures separate from operational/cancellation errors. |

## TypeScript feature coverage

Compared with the [official v2.35.0 parser guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/parser.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| Manual / handler parsing | `Parse`, `WrapHandler` | Typed callbacks replace decorators/Middy. |
| Safe / inline handling | `SafeParse`, `WrapSafeHandler` | Non-nil issues indicate rejection; operational errors still propagate. |
| Built-in schemas | `parser/schemas` | 90 runtime schema definitions mapped; inferred-type parity remains open. |
| Envelopes | `parser/envelopes` | Fourteen envelopes; source-specific decoding and failure paths. |
| Custom validation / types | `SchemaFunc`, `Typed`, `Refine`, `Transform`, `Pipe` | Synchronous context-aware Go validators; no Zod/Promise runtime. |
| Parse errors / unions | `Issue`, `ParseError`, `Union` | Recursive branch diagnostics; native/metadata boundaries remain. |

Executable evidence: [parser/reference_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parser/reference_test.go), [parser/identity_reference_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parser/identity_reference_test.go), [parser/union_reference_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parser/union_reference_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.

## Schema contract

`Schema[T].Validate(context.Context, any)` returns typed data, a slice of `Issue`, and an operational error. A nil issue slice means validation succeeded. A non-nil slice, including an empty slice, means validation failed. `SchemaFunc[T]` adapts application validators without requiring a particular schema engine. Issues carry message/path, optional code/expected fields and recursive union branch `Errors`. `Continuable` controls check/refinement evaluation and is omitted from serialized diagnostics.

`Parse` returns data or an error. `SafeParse` returns a `Result[T]`: validation failures contain `Error` and the original input reference; successful results contain `Data`. Operational errors and cancellation still return errors. `ParseError` returned by an envelope or custom Go schema represents a validation failure. Other errors propagate. Panics are not intercepted. Go has no JavaScript Promise-based validator return; validators execute synchronously and can honor context cancellation.

`WrapHandler` validates before invoking business code. `WrapSafeHandler` passes the safe result to application code for inline handling. Both preserve shared invocation identity and context values. Handler results, errors and panics retain their original behavior.

## Composition

Available constructors include `String`, `Number`, `Boolean`, `Null`, `Literal`, `Enum`, `Unknown`, `Nullable`, `Union`, `Array`, `Dictionary` and ordered `Object` fields. `Field.Optional` permits an absent field; nullable schemas permit explicit null. `Null` requires explicit null and rejects an absent field. `WithDefault` applies to absence and snapshots mutable defaults. Objects strip unknown fields by default; `PreserveUnknown` and `RejectUnknown` provide explicit alternatives. `Extend` and `Omit` return new object schemas.

`Refine` adds a predicate issue and continues after earlier non-aborting check failures. `Transform` converts validated values, with callback errors treated as operational errors. `Pipe` validates the first schema's output through a second schema. `Typed[T]` converts validated output using Go JSON field tags; incompatible destination types return decoding errors. `Any` adapts generic schema outputs while retaining intermediate typed values for check evaluation. Object, Array, Dictionary, Union, Nullable, Transform, Typed, Any, Pipe, Refine and JSON/Base64 helpers preserve optional `SafeSchema` aggregation mode.

Union branches retain declaration order. The first success wins; a sole branch with only continuable check failures keeps its diagnostics, while ambiguous failures retain recursive branch errors. A nested `ParseError` becomes validation issues that parent containers can prefix and aggregate; other operational errors and panics propagate. See [recursive error and composition contracts](PARSER_ERRORS.md), including branch-relative paths, error ownership and remaining metadata gaps.

Built-in object and collection validation creates output values. Reused defaults and unknown values are cloned through Commons. Custom validators and conversion callbacks own their mutation and concurrency behavior. Reusable schema configuration should be constructed before processing requests.

```go
type Order struct {
    ID string `json:"id"`
    Amount float64 `json:"amount"`
}

payload := parser.Typed[Order](parser.Object(
    parser.Field{Name: "id", Schema: parser.String()},
    parser.Field{Name: "amount", Schema: parser.Refine(
        parser.Number(),
        func(value any) bool { return value.(float64) >= 0 },
        "amount must be non-negative",
    )},
))

orders, err := parser.Parse(ctx, event,
    envelopes.SQS(parser.JSONStringified(payload)))
```

`JSONStringified` requires a string and then validates decoded JSON. `Base64Encoded` accepts Base64 JSON, gzip-compressed Base64 JSON, or decoded text, matching the reference helper's fallback sequence. Base64 decoding reuses Commons. Malformed Base64 can produce an operational error. A malformed JSON issue retains the `Invalid JSON` prefix; the detailed syntax diagnostic comes from Go's decoder.

## Initial event contracts

| Reference export | Go equivalent | Behavior |
| --- | --- | --- |
| SqsMsgAttributeDataTypeSchema | schemas.SqsMsgAttributeDataTypeSchema | String, Number, Binary and custom string data types |
| SqsMsgAttributeSchema | schemas.SqsMsgAttributeSchema | Nullable string/binary values and optional lists |
| SqsAttributesSchema | schemas.SqsAttributesSchema | Required delivery fields and optional FIFO/trace/dead-letter attributes |
| SqsRecordSchema | schemas.SqsRecordSchema | Record metadata, body and message attributes |
| SqsSchema | schemas.SqsSchema | Nonempty records and stripped unknown fields |
| EventBridgeSchema | schemas.EventBridgeSchema | Event metadata, UTC ISO timestamp, detail and optional replay name |
| SqsEnvelope | envelopes.SQS | Validate envelope, then each body; JSON decoding is explicit |
| EventBridgeEnvelope | envelopes.EventBridge | Validate metadata and detail together, return typed detail |

SQS ordinary parsing stops at the first invalid payload record. Safe parsing aggregates invalid payload records and prefixes paths with `Records`, the record index, and `body`. Neither returns partial successes on an overall failure. Both validate outer record metadata before parsing bodies. EventBridge aggregates metadata and detail issues. Its unextended `unknown` detail schema accepts absence, as observed in the pinned distribution; replacing detail with a required payload schema changes that behavior.

Use `batch.WithParser` with `parser.Parse` to preserve original retry identifiers. An Idempotency wrapper can follow parsing so malformed inputs cannot acquire persistence records. Batch and Idempotency do not depend on Parser: the application composes the modules. See [the Batch example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/batch/main.go) and [the EventBridge example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/parser/main.go).

The additional 22 stream/notification schema exports, six envelopes and `DynamoDBMarshalled` helper are mapped in [PARSER_STREAMS.md](PARSER_STREAMS.md). They share record traversal and issue-prefix logic while preserving decoding differences: SNS/SQS bodies remain strings, Kinesis decodes JSON or text, Firehose decodes text, CloudWatch decompresses its envelope, and DynamoDB decodes image attribute values.

The eighteen HTTP schema exports and five HTTP body envelopes are mapped in [PARSER_HTTP.md](PARSER_HTTP.md). They reuse certificates/dictionaries and a common object-field extractor with EventBridge. Application body schemas replace the base body's rule; Base64 flags do not implicitly decode content.

## Evidence and remaining boundaries

The fifteen service schema exports and Kafka envelope are mapped in [PARSER_SERVICES.md](PARSER_SERVICES.md). Kafka preserves topic order when supplied raw JSON, decodes message text explicitly and retains its distinct failure paths. S3 reuses the SQS/EventBridge schemas; Kafka and stream schemas share text decoding.

The 29 AppSync/shared, AppSync Events and Cognito exports are mapped in [PARSER_IDENTITY.md](PARSER_IDENTITY.md). They share identity and Cognito request primitives while retaining source-specific nullable fields, union order, trigger literals and response constraints. Native Lambda examples demonstrate typed resolver arguments and Cognito event conversion after input validation.

The generators execute the pinned Parser and Zod packages to produce 40 core, 106 stream, 458 HTTP, 270 service, 1,467 identity and 1,152 union/refinement cases (3,493 total). Every corpus compares success/data, original input association, and issue code/message/path/expected fields recursively through union branches. JSON syntax diagnostic suffixes are normalized at every tree depth. Kafka's null-input exception and three rejected validation promises have explicit operational-error mappings, documented in PARSER_SERVICES.md. Native BigInt/non-finite numbers use the existing Commons fixture representation, and JavaScript Sets are compared to Go value slices. Tests also cover empty issue slices, operational errors, cancellation, handler errors/panics, mutable defaults, schema extension, accepted absence, pipeline modes and concurrent reuse/error-tree ownership.

[PARSER_EXPORTS.json](PARSER_EXPORTS.json) inventories imported runtime exports and declaration names; [PARSER_SCHEMA_MAP.json](PARSER_SCHEMA_MAP.json) maps all 90 runtime schema names to Go definitions and reference subpaths. Declaration names can include supporting local types, and complete inferred-type mapping remains open. Constraint-specific error metadata, remaining primitive/union edge cases, malformed Unicode and numeric boundaries, exhaustive envelope compatibility, performance measurements and cross-language behavior also remain open. Built-in number parsing uses float64; DynamoDB decoding retains large integer strings as `*big.Int`, which should be consumed through an appropriate schema/output type.

Node and Zod are development-only reference generators. Deployed Go Lambda binaries do not use them. Follow [MODULES.md](MODULES.md) for local unpublished module verification and dependency maintenance.
