---
description: "Parse Go Lambda events into typed values and validate handler inputs using Powertools Parser and documented event models."
---

# Parser

Parser validates and transforms Lambda event data into typed Go values. Import `github.com/rambow-cloud/powertools-lambda-go/parser`; built-in schemas and envelopes are subpackages. It does not require Zod at runtime. The reference uses Powertools v2.35.0 and Zod v4.1.12.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Install

Use Go 1.27 or newer and install the module in your own application:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/parser@v1.1.0
```

## Complete example

Define a schema, then call `Parse` to validate and convert a payload to a Go type.
This program runs locally without Lambda or AWS credentials.

~~~go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

type Order struct {
	ID string `json:"id"`
}

func main() {
	schema := parser.Typed[Order](parser.Object(
		parser.Field{Name: "id", Schema: parser.String()},
	))
	order, err := parser.Parse(context.Background(), map[string]any{"id": "ORD-123"}, schema)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(order.ID)
}
~~~

`Object` describes the fields to validate; `Typed[Order]` converts the validated
value into your struct. Match input names with explicit `json` tags.

## Input and output

The valid payload `{"id":"ORD-123"}` produces an `Order` and prints `ORD-123`.
Changing `id` to a number produces a `ParseError`; inspect its issues for the `id`
path. Use `SafeParse` when you prefer a result containing `Success`, `Data` and `Error`.

## Common tasks

- [Build a typed schema](#schema-contract).
- [Validate a Lambda handler or batch](#composition).
- [Choose an event envelope](#initial-event-contracts).

## Schema contract

`Schema[T].Validate(context.Context, any)` returns typed data, a slice of `Issue`, and an operational error. A nil issue slice means validation succeeded. A non-nil slice, including an empty slice, means validation failed. `SchemaFunc[T]` adapts application validators without requiring a particular schema engine. Issues carry message/path, optional code/expected fields and recursive union branch `Errors`. `Continuable` controls check/refinement evaluation and is omitted from serialized diagnostics.

`Parse` returns data or an error. `SafeParse` returns a `Result[T]`: validation failures contain `Error` and the original input reference; successful results contain `Data`. Operational errors and cancellation still return errors. `ParseError` returned by an envelope or custom Go schema represents a validation failure. Other errors propagate. Panics are not intercepted. Go has no JavaScript Promise-based validator return; validators execute synchronously and can honor context cancellation.

`WrapHandler` validates before invoking business code. `WrapSafeHandler` passes the safe result to application code for inline handling. Both preserve shared invocation identity and context values. Handler results, errors and panics retain their original behavior.

JSON helpers and typed conversion use `encoding/json/v2`. `Typed[T]` matches JSON names case-sensitively; use explicit tags such as `json:"id"` on application fields. Raw JSON rejects duplicate members and invalid Unicode, including repeated Kafka topic members. Base64 helpers retain their documented JSON-then-text fallback when strict JSON decoding fails.

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

`Base64Encoded` reuses Commons UTF-8 decoding, replacing each maximal malformed subpart separately. Plain input and the original-byte fallback strip exactly one leading BOM, matching `TextDecoder`. The gzip JSON path retains BOM, matching `Buffer.toString`; unsuccessful gzip JSON parsing still falls back to the original compressed bytes. Twenty pinned reference cases cover these paths, malformed byte runs, truncated sequences, single/double BOMs and valid Unicode in `testdata/utf8-v2.35.0.json`.

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

The additional 22 stream/notification schema exports, six envelopes and `DynamoDBMarshalled` helper are mapped in [PARSER_STREAMS.md](PARSER_STREAMS.md). They share record traversal and issue-prefix logic while preserving decoding differences: SNS/SQS bodies remain strings, Kinesis decodes JSON or text, Firehose decodes text, CloudWatch decompresses its envelope, and DynamoDB decodes image attribute values.

The eighteen HTTP schema exports and five HTTP body envelopes are mapped in [PARSER_HTTP.md](PARSER_HTTP.md). They reuse certificates/dictionaries and a common object-field extractor with EventBridge. Application body schemas replace the base body's rule; Base64 flags do not implicitly decode content.

## Evidence and remaining boundaries

??? info "Reference evidence and compatibility details"

    The fifteen service schema exports and Kafka envelope are mapped in [PARSER_SERVICES.md](PARSER_SERVICES.md). Kafka preserves topic order when supplied raw JSON, decodes message text explicitly and retains its distinct failure paths. S3 reuses the SQS/EventBridge schemas; Kafka and stream schemas share text decoding.

    The 29 AppSync/shared, AppSync Events and Cognito exports are mapped in [PARSER_IDENTITY.md](PARSER_IDENTITY.md). They share identity and Cognito request primitives while retaining source-specific nullable fields, union order, trigger literals and response constraints. Native Lambda examples demonstrate typed resolver arguments and Cognito event conversion after input validation.

    The generators execute the pinned Parser and Zod packages to produce 40 core, 106 stream, 458 HTTP, 270 service, 1,467 identity and 1,152 union/refinement cases (3,493 total). Every corpus compares success/data, original input association, and issue code/message/path/expected fields recursively through union branches. JSON syntax diagnostic suffixes are normalized at every tree depth. Kafka's null-input exception and three rejected validation promises have explicit operational-error mappings, documented in PARSER_SERVICES.md. Native BigInt/non-finite numbers use the existing Commons fixture representation, and JavaScript Sets are compared to Go value slices. Tests also cover empty issue slices, operational errors, cancellation, handler errors/panics, mutable defaults, schema extension, accepted absence, pipeline modes and concurrent reuse/error-tree ownership.

    [PARSER_EXPORTS.json](PARSER_EXPORTS.json) inventories imported runtime exports and declaration names; [PARSER_SCHEMA_MAP.json](PARSER_SCHEMA_MAP.json) maps all 90 runtime schema names to Go definitions and reference subpaths. Declaration names can include supporting local types, and complete inferred-type mapping remains open. Constraint-specific error metadata, remaining primitive/union edge cases, malformed Unicode and numeric boundaries, exhaustive envelope compatibility, performance measurements and cross-language behavior also remain open. Built-in number parsing uses float64; DynamoDB decoding retains large integer strings as `*big.Int`, which should be consumed through an appropriate schema/output type.

    Node and Zod are development-only reference generators. Deployed Go Lambda binaries do not use them. Follow [MODULES.md](MODULES.md) for local unpublished module verification and dependency maintenance.

## Objects and lifecycle

| Object | How to use it |
| --- | --- |
| `schema` | Define once and reuse to validate payloads. |
| `Order` | Your application type; JSON tags identify input fields. |
| Parsed result | A typed value available only after successful validation. |
| Lambda wrapper | Optional `WrapHandler` integration for validation before business logic. |

## TypeScript feature coverage

??? info "Compare with TypeScript v2.35.0"

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
