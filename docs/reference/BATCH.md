# Batch processing: API and behavior notes

Start with the [Batch processing guide](../BATCH.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

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

### Add schema validation

When records need validation beyond JSON decoding, compose Parser with Batch.
The maintained example validates each `id` and retains the original SQS message ID
for retries:

~~~go
--8<-- "examples/batch/main.go"
~~~


## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `processor` | Immutable reusable source and processing configuration. |
| `handler` | Per-record business callback; a returned error marks that record failed. |
| Lambda wrapper / `Report` | The wrapper returns retry identifiers; `Process` additionally exposes ordered record results. |

