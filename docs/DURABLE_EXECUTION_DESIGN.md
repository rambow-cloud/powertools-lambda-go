# Durable execution and Idempotency boundaries

Planning acceptance for [issue #120](https://github.com/rambow-cloud/powertools-lambda-go/issues/120), reviewed on 2026-10-08. No durable runtime, persistence replacement, integration dependency or production API is added here.

## Current platform finding

AWS [enabled custom Durable Execution SDKs in July 2026](https://aws.amazon.com/about-aws/whats-new/2026/07/aws-lambda-durable-custom-sdk/), including Lambda-supported custom runtimes. The managed-runtime tutorial alone is insufficient to conclude that Go cannot use durable functions.

AWS now publishes an [experimental Go SDK](https://github.com/aws/aws-durable-execution-sdk-go/blob/8acd08cbad44b871a64960919b0a5f850726698c/README.md). The assessed commit is `8acd08cbad44b871a64960919b0a5f850726698c`; its warning explicitly excludes production use and its API can change. It provides `provided.al2023` examples, local durable testing, serialization customization, replay-aware logging and experimental plugin hooks. Treat it as an evaluation candidate, not a stable dependency or a Powertools compatibility guarantee. Pin an exact reviewed revision if a prototype is later approved.

AWS exposes [checkpoint](https://docs.aws.amazon.com/lambda/latest/api/API_CheckpointDurableExecution.html) and [execution-state](https://docs.aws.amazon.com/lambda/latest/api/API_GetDurableExecutionState.html) APIs. The checkpoint token advances with state; calling the service APIs from a Go AWS client does not itself implement the replay protocol. Prefer evaluating the existing SDK over building a second workflow engine.

## Two different replay contracts

| Boundary | Existing Idempotency | Platform durable execution |
| --- | --- | --- |
| Identity | Application-selected payload/function persistence key | Durable execution plus deterministic operation/attempt identity |
| Stored state | Conditional in-progress/completed record and response with expiry | Checkpoints, operation history, waits/callbacks and execution state |
| Duplicate handling | Replays a stored completed response without rerunning the wrapped handler | Reconstructs workflow progress across separate invocations |
| Failure window | Side effect can occur before completion is persisted | Step side effect can occur before its checkpoint is recorded |
| Suspension | No durable wait or continuation scheduler | Runtime/service protocol owns suspension and resumption |
| Guarantee | Scoped response replay and conditional acquisition | SDK/service-specific operation semantics; no universal exactly-once side effects |

Retain DynamoDB/Redis persistence as application-level duplicate protection. A durable workflow may call Idempotency inside a live step when its business contract needs it, but must not use the Idempotency table as a hidden checkpoint store. Use the existing persistence interfaces; no unconditional durable dependency belongs in root Commons or core Idempotency.

## Proposed optional integration contract

| Concern | Proposed requirement |
| --- | --- |
| Invocation identity | Create one ordinary shared utility invocation scope for each runtime invocation; composed wrappers reuse it. A resumed execution gets a new request ID, not a reused process-global scope |
| Workflow identity | Carry execution ARN, explicit operation ID/name and attempt as separate contextual attributes; never redefine Commons invocation/cold-start identity |
| Deterministic business key | Application chooses a stable execution/operation/business identifier; no request ID, clock, random value or unordered map traversal in replay-sensitive identity |
| Side effects | Execute business writes in the SDK's live operation body. No duplicate metric/log/span emission merely because completed operations are replayed |
| Retry observability | Count live attempts separately from successful logical operations; document possible repeated emissions after a crash before checkpoint commit |
| Logging | Evaluate the SDK's replay-aware `slog.Handler` seam before adding another Logger bridge; preserve SDK identifiers and replay suppression, avoid duplicate root fields |
| Tracing | Use OTel, one ordinary invocation span per resume and live-attempt spans; represent workflow continuity with explicit attributes/links, never keep an open span/context across suspended processes |
| Cleanup | Close invocation-owned utility scopes on completion/error/suspension. Keep injected clients/exporters application-owned; do not persist a context, mutex, span or Logger instance |

The candidate plugin hooks can supply a derived `context.Context` around invocation/operation work. Prototype that seam and the existing utilities before adding a public abstraction. Review hook ordering, concurrent child operations, replay notification behavior and suspension handling at the pinned revision. This is a proposal, not an implemented adapter.

## Serialization, errors and cancellation

Checkpoint values need an explicit versioned JSON contract. Evaluate the SDK's serializer customization with `encoding/json/v2`, including exact member names, nil maps/slices, omitted values, duplicate members, invalid Unicode, large numbers, timestamps and custom marshalers. Persist only JSON values; schema/serializer upgrades must remain compatible with executions already in flight. Neither the current Idempotency canonicalization nor the SDK's default JSON encoder automatically establishes that cross-version contract.

Serialization failures must retain the SDK's operation/direction information and permanent-versus-retryable distinction. Preserve typed business errors, cancellation and underlying operational causes. Invocation timeout, workflow timeout, business failure and SDK suspension are different outcomes; an observability wrapper must not turn a wait/suspension signal into a handler failure or swallow one as success. The SDK controls checkpoint tokens, durable retry and continuation.

Use per-invocation deadlines for live SDK/KMS/DynamoDB/HTTP calls. Cancellation must stop owned work and avoid a new business side effect after its deadline; it must not erase a previously committed result or reset another branch. A new invocation reconstructs state from checkpoints rather than retaining old Go goroutines. Do not promise exactly-once delivery for an external effect without the application's transactional/idempotency design.

## Dedicated acceptance before implementation

- Pin the experimental SDK and fixtures outside maintained core dependencies. Review its license, transitive dependencies, toolchain/JSON behavior and platform requirements before selecting an optional module/API. No local `replace` directives; preserve root's lack of external dependencies.
- Use the SDK's local testing seam and local Runtime API fixtures for step-success, wait/resume, callbacks, retries, permanent errors, cancellation and malformed/missing checkpoint state. Validate exact execution/operation outcomes rather than only the final response.
- Inject a crash after a business effect and before checkpoint success. Verify the documented retry behavior and demonstrate how an explicitly configured Idempotency key reduces duplicates; retain the unavoidable failure-window limits.
- Count logs, EMF records and OTel spans across initial execution, replay, live retry and child branches. Completed replay creates no new business-attempt record; resumed invocations retain distinct request IDs and correct shared utility scope closure.
- Round-trip JSON v2 checkpoint results across fresh process instances, previous supported serializer versions and changed application struct definitions. Cover invalid/tampered/oversized data, precise numbers, null/absence, custom marshalers, errors and payload-limit boundaries.
- Test deterministic operation ordering, independent branches, concurrent hooks, late callbacks, exporter failure and bounded shutdown without modifying durable scheduling. Keep SDK goroutine/context ownership rules; utilities must not create durable operations from instrumentation callbacks.
- Run affected packaged modules and independent consumers with `GOWORK=off`, `CGO_ENABLED=0`, then Linux amd64/arm64 `provided.al2023` builds and local integration. Existing response-replay evidence does not satisfy this acceptance.

Cloud acceptance requires a separate explicit account/profile/cost/resource task, ap-east-1 service availability and a pinned deployed artifact. Verify actual suspension/reinvocation, checkpoint IAM permissions and cleanup; never add cloud credentials to PR CI. No cloud call is authorized by this design.

## Decision

The platform is capable of custom-SDK durable execution, and an official Go prototype exists. The project has no durable integration yet, and the candidate's experimental API is the dependency risk. Complete #120's planning scope and retain the current supported subset. A separately reviewed prototype can evaluate OTel/Logger integration without changing Idempotency; production support remains a later acceptance decision.
