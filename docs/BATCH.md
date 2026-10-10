---
description: "Process SQS, Kinesis and DynamoDB Streams batches in Go Lambda with Powertools partial failures, FIFO policies and typed handlers."
---

# Batch processing

Batch processes SQS Standard/FIFO, Kinesis and DynamoDB Streams records and builds Lambda partial-failure responses. Import `github.com/rambow-cloud/powertools-lambda-go/batch`. Processing does not call an AWS service; configure your event source mapping separately.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Complete example

The complete Lambda example below parses each SQS body as an order, processes valid records and reports failed record IDs. Build it with `CGO_ENABLED=0` using `./examples/batch`; configure `ReportBatchItemFailures` on the SQS event source mapping.

~~~go
--8<-- "examples/batch/main.go"
~~~

## Input and output

For the input below, record `m1` succeeds and `m2` fails its `id` string validation. The handler returns `{"batchItemFailures":[{"itemIdentifier":"m2"}]}`. It does not return the successful business value `ORD-123` to SQS. `Report.Results` is available when calling `Process` directly. This program has no explicit application log call. If every record fails, the default is a `FullBatchFailureError` instead of this partial response.

Even when every record fails, `Process` returns a populated `Report.Response` alongside `FullBatchFailureError`; Lambda wrappers return the same response alongside the error. This corrects the empty full-failure response in TypeScript v2.35.0. Set `SuppressFullBatchFailure: true` to return the failure response without that error. A Lambda invocation that returns an error still fails the entire batch. FIFO-skipped records remain in the retry list, and DynamoDB Streams entries without a sequence identifier remain omitted.

~~~json
{
  "Records": [
    {
      "messageId": "m1",
      "body": "{\"id\":\"ORD-123\"}"
    },
    {
      "messageId": "m2",
      "body": "{\"id\":123}"
    }
  ]
}
~~~

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `processor` | Immutable reusable source and processing configuration. |
| `schema` | Reusable body validation; parse failures retain original record IDs. |
| `handler` | Per-record business callback; a returned error marks that record failed. |
| Lambda wrapper / `Report` | The wrapper returns retry identifiers; `Process` additionally exposes ordered record results. |

## TypeScript feature coverage

Compared with the [official v2.35.0 batch guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/batch.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| SQS / Kinesis / DynamoDB | Source-specific constructors and wrappers | Preserves MessageId/sequence identifiers. |
| Partial / complete failures | Failure response and `FullBatchFailureError` | Lambda mapping must enable partial responses. |
| FIFO / sequential processing | `NewSQSFIFO`, `Sequential`, `SkipGroupOnError` | Failed-group rules; skipped records remain explicit in Go reports. |
| Parallel processing | `MaxConcurrency` | Go concurrency bound; contexts and panics are handled explicitly. |
| Parser integration | `WithParser` plus `parser.Parse` | Implemented in this complete example and local runtime fixture. |
| Custom processor / results / context | `Source`, `RecordProcessor`, `Process` | Interfaces replace subclassing; live checkpoint/retry gates remain. |

Executable evidence: [batch/reference_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/batch/reference_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.

## Processing and response semantics

- Default processing is parallel, matching the asynchronous TypeScript processor. `Sequential: true` corresponds to sequential async processing or the synchronous processor. `MaxConcurrency` bounds active handlers; zero permits all records in parallel. This limit is a Go extension.
- FIFO processing is always sequential, regardless of the concurrency setting. By default a failure marks every remaining record for retry without invoking its handler. `SkipGroupOnError: true` only skips later records in the failed group and allows other groups to proceed.
- As in the pinned source, an empty or missing group ID is not remembered as a failed group. FIFO event sources normally supply it; applications should validate malformed event data when needed.
- All-failed nonempty batches return `*FullBatchFailureError` unless `SuppressFullBatchFailure` is enabled. Its `RecordErrors` and `Unwrap() []error` preserve individual causes. Like the reference's response state before cleanup fails, the response remains empty on this error; returning the error from a Lambda handler retries the whole batch.
- Empty batches succeed with exactly `{"batchItemFailures":[]}`. Typed wrappers reject missing/null Records slices; malformed Records types are rejected by the Lambda JSON decoder before the handler is called.
- SQS identifiers are MessageId strings. Stream identifiers are full sequence-number strings, preserving values beyond numeric precision limits. Every failure is reported; the module does not select or truncate to the lowest sequence number. Lambda controls stream checkpoint/retry behavior.
- The DynamoDB collector reproduces the pinned behavior of omitting empty sequence numbers. This is a malformed-record boundary, not a recommendation to accept incomplete stream events.

`Process` returns a call-owned `Report` with input-ordered `Results`, completion-ordered `Successes`/`Failures`, and collected errors. Each result records its original record, handler value, error, and whether processing was skipped. Unlike the TypeScript FIFO process return array, Go includes explicit entries for all skipped records. This does not change the Lambda failure response.

Processors store immutable configuration only. All failure lists, group sets, and responses belong to the current call, so warm reuse and concurrent Lambda invocation tests do not depend on a mutable processor registration store. Records and handler return values remain application-owned; do not mutate shared nested objects concurrently.

## Context, errors, and composition

Typed wrappers preserve Lambda context and share the existing invocation identity. Cancellation prevents new handler calls and records unstarted items as failures. Started handlers must honor context; the processor waits for them before returning. A handler panic becomes a `*PanicError` record failure, preserving an underlying error through `Unwrap`, rather than crashing an unrelated worker goroutine. FIFO skip errors remain distinguishable from handler failures.

`WithParser` composes a typed `(context, record) -> (parsed, error)` callback with a parsed-value handler. Parsing failures become `*ParsingError` and retain the original record identifier for retries. It does not force a dependency on Parser or JMESPath. The local fixture composes it with JMESPath JSON decoding, Logger invocation logging, and OTel child spans. Concrete Parser composition is shown above; the local runtime fixture also verifies parsed records with Idempotency, Logger and Tracer.

No SDK API is called by the processor. Configure the event source mapping with `ReportBatchItemFailures`; returning this JSON alone does not enable service-side partial retries. Cloud event-source configuration, stream checkpointing, visibility timeouts, redrive policies, and actual retry behavior require separate AWS validation.

## Reference and verification scope

The pinned TypeScript v2.35.0 implementation supplies 43 reference scenarios: synchronous/asynchronous sequential processing, empty/success/partial/full-failure batches, suppressed full failure, FIFO stopping and failed groups, missing/empty group IDs, and missing DynamoDB sequence identifiers. Four invalid event envelopes are recorded separately. Go tests cover typed wrappers, parser failures, context preservation, panic/error causes, bounded concurrency, 100 overlapping calls, and warm FIFO reuse. The current acceptance result is tracked in [BATCH_PLAN.md](BATCH_PLAN.md) and [LOCAL_VALIDATION.md](LOCAL_VALIDATION.md).

Outstanding gates include exhaustive malformed-event/parser and concurrent completion-order differential coverage, live event-source retries/checkpoints, and performance/allocation budgets. This does not claim complete specification or service acceptance from local fixtures.

Sources: [pinned Batch source](https://github.com/aws-powertools/powertools-lambda-typescript/tree/7bcc27b1574493f9452688673658f52b80c53847/packages/batch/src), [SQS partial responses](https://docs.aws.amazon.com/lambda/latest/dg/services-sqs-errorhandling.html), and [Kinesis partial responses](https://docs.aws.amazon.com/lambda/latest/dg/services-kinesis-batchfailurereporting.html).
