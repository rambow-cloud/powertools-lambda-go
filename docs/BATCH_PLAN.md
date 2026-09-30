# Batch implementation checklist

Reference: TypeScript v2.35.0. Implement behavior from the pinned package, with record-level and composed Lambda verification. Full project completion also requires the remaining modules and existing parity gates in CHECKLIST.md.

- [x] B-01: Implement SQS Standard, Kinesis and DynamoDB failure identifiers and exact response envelopes; verify pinned reference cases and typed stream wrappers.
- [x] B-02: Implement FIFO first-failure stopping, failed-group skipping, and empty/missing group behavior; verify synchronous/asynchronous reference cases and warm reuse.
- [x] B-03: Implement default parallel and sequential processing, optional bounded concurrency, full-batch error policy, empty batches, cancellation, and panic conversion; verify worker bounds and error identity.
- [x] B-04: Expose invocation-owned ordered results, completion-ordered successes/failures, errors, custom processor interfaces, and typed Lambda wrappers; verify 100 overlapping calls and context preservation.
- [ ] B-05: Generate actual TypeScript fixtures for ordinary, FIFO, stream, invalid, complete failure, parallel order, and repeated-invocation scenarios.
- [x] B-05a: Generate and compare 43 sequential reference scenarios across synchronous/asynchronous processors. Record four invalid envelopes; Go tests verify typed missing/null envelope rejection and warm reuse. Concurrent completion-order reference fixtures remain under B-05.
- [x] B-06: Complete initial concrete Parser integration. The application composes Parser with Batch and DynamoDB-backed Idempotency; invalid orders are rejected before business execution. All 184 Docker assertions and 14 Batch artifact checks passed (2026-09-15). Broader malformed-input/service acceptance remains under B-08; Batch retains no Parser module dependency.
- [x] B-06a: Implement WithParser and verify parsed results, original retry identifiers, and unwrap-compatible parsing failures without importing a schema/query engine.
- [x] B-06b: Compose typed parsing with the Idempotency wrapper and DynamoDB store; verify duplicate suppression, original failed-record identifiers, deletion after failure, successful retry, validation rejection, and scoped Logger/OTel behavior in the 155-assertion Docker run (2026-09-15).
- [x] B-07: Verify 15 packaged modules and 12 independent consumers, both CGO-disabled Lambda architecture builds, 129/129 Docker assertions, and 14/14 Batch composition checks from the same run. Evidence: [BATCH_ACCEPTANCE.json](BATCH_ACCEPTANCE.json). Docker startup required runtime-only continuation using the already verified binaries.
- [ ] B-08: Complete live event-source retry/checkpoint validation, exhaustive malformed event/parser behavior, and throughput/allocation budgets.
