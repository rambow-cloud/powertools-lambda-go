# Batch implementation checklist

Reference: TypeScript v2.35.0. Implement behavior from the pinned package, with record-level and composed Lambda verification. Full project completion also requires the remaining modules and existing parity gates in CHECKLIST.md.

- [x] B-01: Implement SQS Standard, Kinesis and DynamoDB failure identifiers and exact response envelopes; verify pinned reference cases and typed stream wrappers.
- [x] B-02: Implement FIFO first-failure stopping, failed-group skipping, and empty/missing group behavior; verify synchronous/asynchronous reference cases and warm reuse.
- [x] B-03: Implement default parallel and sequential processing, optional bounded concurrency, full-batch error policy, empty batches, cancellation, and panic conversion; verify worker bounds and error identity.
- [x] B-04: Expose invocation-owned ordered results, completion-ordered successes/failures, errors, custom processor interfaces, and typed Lambda wrappers; verify 100 overlapping calls and context preservation.
- [x] B-05: Generate actual TypeScript fixtures for ordinary, FIFO, stream, invalid, complete failure, parallel order, and repeated-invocation scenarios. The existing 43 sequential cases are extended by three controlled parallel cases, twelve warm calls and fifteen malformed-record cases under issue #185; typed-Go differences remain explicit in the finite audit below (2026-10-13).
- [x] B-05a: Generate and compare 43 sequential reference scenarios across synchronous/asynchronous processors. Record four invalid envelopes; Go tests verify typed missing/null envelope rejection and warm reuse. Concurrent completion-order reference fixtures remain under B-05.
- [x] B-06: Complete initial concrete Parser integration. The application composes Parser with Batch and DynamoDB-backed Idempotency; invalid orders are rejected before business execution. All 184 Docker assertions and 14 Batch artifact checks passed (2026-09-15). Broader malformed-input/service acceptance remains under B-08; Batch retains no Parser module dependency.
- [x] B-06a: Implement WithParser and verify parsed results, original retry identifiers, and unwrap-compatible parsing failures without importing a schema/query engine.
- [x] B-06b: Compose typed parsing with the Idempotency wrapper and DynamoDB store; verify duplicate suppression, original failed-record identifiers, deletion after failure, successful retry, validation rejection, and scoped Logger/OTel behavior in the 155-assertion Docker run (2026-09-15).
- [x] B-07: Verify 15 packaged modules and 12 independent consumers, both CGO-disabled Lambda architecture builds, 129/129 Docker assertions, and 14/14 Batch composition checks from the same run. Evidence: [BATCH_ACCEPTANCE.json](BATCH_ACCEPTANCE.json). Docker startup required runtime-only continuation using the already verified binaries.
- [ ] B-08: Complete live event-source retry/checkpoint validation, exhaustive malformed event/parser behavior, and throughput/allocation budgets.

## Finite boundary audit (#185)

This audit covers the named local contracts of the implemented utility against
TypeScript Powertools 2.35.0. Ordinary Go tests consume checked-in reference data;
Node is needed only to regenerate it. No production API or behavior changes were
required by these cases.

| Boundary | Evidence | Classification |
| --- | --- | --- |
| Ordinary, FIFO, streams, partial/full failure and suppression | Existing 43 reference scenarios and 16 full-failure regression combinations | Verified; Go's populated response alongside a full-failure error is the existing #53 correction |
| Parallel completion order | Three actual TypeScript barrier cases; `TestParallelCompletionBoundaryReference` | Verified input-ordered results and completion-ordered success/failure/retry lists. Two Go workers and channel fences prove collection order without scheduling sleeps; the pinned processor uses its default parallel mode |
| Repeated invocations and report ownership | Twelve actual warm calls across SQS/FIFO/Kinesis/DynamoDB; `TestWarmInvocationBoundaryReference` and existing 100-call overlap test | Failure/group state clears before later success and empty calls; earlier reports retain their slices |
| Empty/missing FIFO groups and parser failures | Existing missing/empty-group reference cases; `TestFIFOParserFailureAndNativeGroups` | Empty/missing/null attribute maps are safe; invalid numeric group values fail in native decoding. Parser failure applies the configured FIFO stop/group-skip rule |
| Malformed records | Fifteen actual upstream cases and `TestMalformedNativeLambdaBoundaryReference` using the pinned Lambda SDK | Valid string identifiers match. Numeric identifiers fail decoding before the business callback. The native Go differences below remain explicit |
| Malformed envelopes | Native Lambda calls for missing/null/object/string Records, primitive record elements and empty arrays, across three sources | Invalid envelopes fail before the record callback; empty arrays return the exact empty partial-failure response |
| Parser failure identifiers and error identity | `TestParserOriginalStreamAndMessageIdentifiers` across three native record types | Original identifiers, including a sequence above the JavaScript safe integer range, survive transformation/failure; `ParsingError` unwraps the cause and warm reuse succeeds |
| Cancellation, bounded concurrency and panic conversion | Existing `TestConcurrencyLimitsAndInvocationIsolation` and `TestFullFailurePanicCancellationAndParser` | Verified local utility contract; callbacks must still honor cancellation |

### Native Go differences

The actual TypeScript malformed fixtures are preserved, including omitted
`itemIdentifier` properties and collector `TypeError`s. Native Go event structs
cannot represent JavaScript `undefined` or property-access errors. A missing/null
record or nested stream container becomes a zero-valued struct. Failed SQS and
Kinesis records then produce an empty string identifier; DynamoDB omits empty
sequence identifiers. Numeric identifiers are rejected by the SDK decoder. These
are explicit input-model differences, not a promise that Lambda accepts malformed
retry responses. Validate required fields with an application parser when needed.

Go records every FIFO-skipped result explicitly, while the pinned processor's
process return array can omit skipped records. Both retry lists retain those
identifiers. The existing full-failure response correction is retained.

`tools/reference/generate-batch-boundaries.mjs` captures the actual upstream
processors. A completion observer waits until each upstream collector has run;
it does not recreate the processor algorithm. Go's bounded-worker test uses the
next dispatched record as a fence proving the previous record was collected.

The Batch packaged tests, vet, tidy/isolation and standalone consumer pass with
CGO disabled on Windows amd64. This finite audit does not close B-08: live event
source retries/checkpoints remain in #201/#202 and resource budgets in #200.
Hosted CI provides separate Linux runtime acceptance; no AWS calls are authorized
by this audit.
