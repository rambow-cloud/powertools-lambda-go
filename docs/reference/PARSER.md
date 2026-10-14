# Parser: API and behavior notes

Start with the [Parser guide](../PARSER.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

## Schema contract

`Schema[T].Validate(context.Context, any)` returns typed data, a slice of `Issue`, and an operational error. A nil issue slice means validation succeeded. A non-nil slice, including an empty slice, means validation failed. `SchemaFunc[T]` adapts application validators without requiring a particular schema engine. Issues carry message/path, optional code/expected fields and recursive union branch `Errors`. `Continuable` controls check/refinement evaluation and is omitted from serialized diagnostics.

`Parse` returns data or an error. `SafeParse` returns a `Result[T]`: validation failures contain `Error` and the original input reference; successful results contain `Data`. Operational errors and cancellation still return errors. `ParseError` returned by an envelope or custom Go schema represents a validation failure. Other errors propagate. Panics are not intercepted. Go has no JavaScript Promise-based validator return; validators execute synchronously and can honor context cancellation.

`WrapHandler` validates before invoking business code. `WrapSafeHandler` passes the safe result to application code for inline handling. Both preserve shared invocation identity and context values. Handler results, errors and panics retain their original behavior.

JSON helpers and typed conversion use `encoding/json/v2`. `Typed[T]` matches JSON names case-sensitively; use explicit tags such as `json:"id"` on application fields. Raw JSON rejects duplicate members and invalid Unicode, including repeated Kafka topic members. Base64 helpers retain their documented JSON-then-text fallback when strict JSON decoding fails.


## Composition

Available constructors include `String`, `Number`, `Boolean`, `Null`, `Literal`, `Enum`, `Unknown`, `Nullable`, `Union`, `Array`, `Dictionary` and ordered `Object` fields. `Field.Optional` permits an absent field; nullable schemas permit explicit null. `Null` requires explicit null and rejects an absent field. `WithDefault` applies to absence and snapshots mutable defaults. Objects strip unknown fields by default; `PreserveUnknown` and `RejectUnknown` provide explicit alternatives. `Extend` and `Omit` return new object schemas.

`Refine` adds a predicate issue and continues after earlier non-aborting check failures. `Transform` converts validated values, with callback errors treated as operational errors. `Pipe` validates the first schema's output through a second schema. `Typed[T]` converts validated output using Go JSON field tags; incompatible destination types return decoding errors. `Any` adapts generic schema outputs while retaining intermediate typed values for check evaluation. Object, Array, Dictionary, Union, Nullable, Transform, Typed, Any, Pipe, Refine and JSON/Base64 helpers preserve optional `SafeSchema` aggregation mode.

Union branches retain declaration order. The first success wins; a sole branch with only continuable check failures keeps its diagnostics, while ambiguous failures retain recursive branch errors. A nested `ParseError` becomes validation issues that parent containers can prefix and aggregate; other operational errors and panics propagate. See [recursive error and composition contracts](../PARSER_ERRORS.md), including branch-relative paths, error ownership and remaining metadata gaps.

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

`Base64Encoded` reuses Commons UTF-8 decoding, replacing each maximal malformed subpart separately. Plain input and the original-byte fallback strip exactly one leading BOM, matching `TextDecoder`. The gzip JSON path retains BOM, matching `Buffer.toString`; unsuccessful gzip JSON parsing still falls back to the original compressed bytes. Twenty pinned reference cases cover these paths, malformed byte runs, truncated sequences, single/double BOMs and valid Unicode in `testdata/utf8-the upstream baseline.json`.

### Parse an EventBridge Lambda event

Add a built-in envelope when Lambda supplies service metadata around your payload.
This maintained example validates EventBridge `detail`, then calls a typed handler:

~~~go
--8<-- "examples/parser/main.go"
~~~

Input detail `{"id":"ORD-123","amount":42}` returns `"ORD-123"`; a negative amount
fails validation before the business callback runs.


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

The additional 22 stream/notification schema exports, six envelopes and `DynamoDBMarshalled` helper are mapped in [PARSER_STREAMS.md](../PARSER_STREAMS.md). They share record traversal and issue-prefix logic while preserving decoding differences: SNS/SQS bodies remain strings, Kinesis decodes JSON or text, Firehose decodes text, CloudWatch decompresses its envelope, and DynamoDB decodes image attribute values.

The eighteen HTTP schema exports and five HTTP body envelopes are mapped in [PARSER_HTTP.md](../PARSER_HTTP.md). They reuse certificates/dictionaries and a common object-field extractor with EventBridge. Application body schemas replace the base body's rule; Base64 flags do not implicitly decode content.


## Objects and lifecycle

The [finite boundary audit](../PARSER_PLAN.md#finite-boundary-audit-188) records
fourteen-envelope invalid inputs, explicit native type/error mappings and strict
JSON behavior; broader export and resource gates remain separate.

| Object | How to use it |
| --- | --- |
| `schema` | Define once and reuse to validate payloads. |
| `Order` | Your application type; JSON tags identify input fields. |
| Parsed result | A typed value available only after successful validation. |
| Lambda wrapper | Optional `WrapHandler` integration for validation before business logic. |

