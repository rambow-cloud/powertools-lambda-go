---
description: "Compose Powertools for Go utilities with correct object lifetimes, Lambda invocation scope, typed handlers and reusable clients."
---

# Usage patterns

Go exposes functions, functional options and small interfaces in place of TypeScript constructors, decorators, Middy middleware and subclassing. Start with [the complete Lambda quickstart](GETTING_STARTED.md), then use the feature guides for inputs and outputs. See [feature comparison](FEATURE_PARITY.md) for the pinned v2.35.0 scope.

## Packages and objects

| Name | Meaning | Lifetime |
| --- | --- | --- |
| `logger`, `metrics`, `tracer` | Imported packages containing constructors and option types | Compile-time namespace |
| `appLog`, `appMetrics`, `trace` | Objects created with `New(...)` | Usually initialized once before `lambda.Start` |
| `requestLog`, `requestMetrics` | Objects obtained with `WithContext(ctx)` inside their wrapper | One invocation; writes after completion are rejected |
| `stdlog` | Standard `log` package under an explicit alias | Plain-text fallback; separate from structured Logger |
| Router/schema/provider | Reusable registered routes, compiled rules or caches | Created once; each call receives its own request context |

Variable names are application choices. A variable called `log` can hide the standard library package of the same name. Use explicit names to make object methods and package functions distinguishable.

## Wrapper composition

For a typed `handler`, compose the initialized utilities in this order:

~~~go
lambda.Start(tracer.WrapHandler(trace,
    logger.WrapHandler(appLog,
        metrics.WrapHandler(appMetrics, handler))))
~~~

This composition fragment requires initialized `trace`, `appLog`, `appMetrics`, the imports and a business handler. Tracer creates the active span first. Logger binds log state to the invocation. Metrics opens its request store and flushes when the handler finishes. All wrappers reuse the shared invocation identity. Inside the handler, obtain `appLog.WithContext(ctx)` and `appMetrics.WithContext(ctx)` and pass that same `ctx` into downstream calls.

Calling `WithContext(context.Background())` outside a wrapped invocation does not create an isolated request. Direct root logging uses process-level state; direct root Metrics additions require an explicit `Flush`. Root mutable state must not be used as an invocation scratchpad. Join handler-owned background work before returning.

HTTP observability middleware uses explicit request scopes in separate modules. Use [HTTP observability](HTTP_OBSERVABILITY.md) instead of adding overlapping instrumentation that emits duplicate metrics or spans.

## Results, output and errors

| Operation | Successful result | Emitted output |
| --- | --- | --- |
| Logger `Info` / `Error` | `nil` unless serialization/output fails; filtered calls also return nil | JSON on the selected stream, if eligible; ERROR does not terminate the handler |
| Metrics `AddMetric` | Value buffered, or validation/automatic-publication error | Normally none until `Flush` or wrapper completion |
| Tracer capture | Original business result/error | Spans sent through the configured provider; no implicit `Span:` console output |
| Parser / Validation | Validated typed data or original/extracted payload | No automatic business log; invalid input returns validation errors |
| Router resolution | Resolver value or HTTP/Bedrock/AppSync response | Response is separate from a log record |
| Parameters / Metadata | Retrieved data or error | No implicit log of the fetched configuration |
| Batch | Partial failure identifiers or full failure error | Successful record return values are not the SQS response |
| Idempotency | Fresh or replayed business result | Persistence interaction; replay skips the business callback |

Handle errors from direct method calls. Logger's `WithErrorHandler` observes wrapper instrumentation failures; it does not automatically receive returned `Info` errors. Metrics reports wrapper flush failures and preserves business results by default; `PropagateErrors` changes that policy explicitly. Tracer reports export/instrumentation failures through its callback. Do not recursively invoke the failing utility from its diagnostic callback.

## Migrating TypeScript concepts

| TypeScript pattern | Go approach |
| --- | --- |
| Utility class constructor | `New(options...)`; check errors where returned |
| Decorated/Middy handler | Typed `WrapHandler` and explicit wrapper order |
| Scoped class method | Function/closure plus explicit `context.Context` |
| Custom base class | Implement the documented small provider/compiler/processor interface |
| Promise validator/provider | Synchronous context-aware callback; concurrency is documented per utility |
| JavaScript object/undefined | JSON-tagged structs/maps, nil and utility-specific Undefined markers where provided |
| Shared mutable state | Request scopes and private snapshots; application-owned callbacks/values still need synchronization |

Use [environment variables](ENVIRONMENT_VARIABLES.md) for deployment defaults and explicit options for application configuration. Defaults and precedence are utility-specific. Go JSON/type/error behavior is not a complete JavaScript runtime emulation; each guide states its boundaries.

## Local and deployed execution

Every Go command uses `CGO_ENABLED=0`. Run offline programs from the repository workspace while releases are pending. Programs using `lambda.Start` wait for a Lambda Runtime API; building them succeeds locally, but plain `go run` is not a Lambda invocation. Use [the maintained Docker runner](LOCAL_INTEGRATION.md) for runtime acceptance.

Native Lambda targets are Linux amd64/arm64 on `provided.al2023`. Optional KMS masking currently cannot compile as a native Windows binary; its maintained tests execute in Linux Docker. No TypeScript package, Node.js or Zod process is needed in a deployed Go executable.
