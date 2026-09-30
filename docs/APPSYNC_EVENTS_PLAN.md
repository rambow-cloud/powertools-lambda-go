# AppSync Events implementation checklist

Reference: installed `@aws-lambda-powertools/event-handler` v2.35.0, locked by the reference package lockfile. Sources inspected: `appsync-events/index.d.ts`, AppSyncEventsResolver.js, Router.js, RouteHandlerRegistry.js, errors.js, utils.js and `types/appsync-events.d.ts`.

- [x] ASE-CORE: Implement separate publish/subscribe dispatch, literal/wildcard precedence, individual/aggregate handling, typed authorization and exact scoped response/error envelopes. Verify actual TypeScript fixtures, functional concurrency, packaged consumers, both Lambda builds and local runtime composition.
- [x] ASE-CACHE: Reuse Commons LRU and retain pinned replacement/eviction and warning deduplication behavior. Verify cached replacements, missing-route recovery, capacity eviction and concurrent invocations.
- [x] ASE-SIZE: Implement opt-in per-event size warnings with inclusive byte boundary and per-channel deduplication. Verify UTF-8/HTML/Unicode separator and literal-escape size cases without dropping events.
- [ ] ASE-COMPLETE: Close the full public type/export audit, JavaScript-native serialization and malformed Unicode boundaries, arbitrary mutation/async/diagnostic ordering behavior, typed Go adapters and remaining route/input/resource edges. Top-level undefined, native errors and immutable/read-only Go event contracts remain explicit in APPSYNC_EVENTS.md.
- [ ] ASE-SERVICE: Validate actual AppSync publishing, subscriptions, IAM/authorizer context and service-side oversized/unauthorized behavior when cloud testing is requested. Establish throughput, allocation, payload and cache budgets; complete release requirements.

## Reuse decisions

Route lookup uses the existing synchronized Commons LRU with capacity 100. Environment handling reuses Commons.StringEnv. HTTP routing has different parameter/middleware/URL semantics, so importing its router would introduce unnecessary dependency and compatibility work. Existing Parser AppSync Events schemas remain optional: their stricter validation cannot replace the reference resolver's lightweight guards. Existing Logger and OTel state is carried by context; handlers and diagnostics compose explicitly without importing those feature modules.

The module preserves the reference's unusual malformed-publish subscription fallback and cache behavior. Fixing those behaviors without a scope decision would change the requested port. Go concurrency requires callers to synchronize callback-owned mutable state; the library keeps registry/cache/warning state synchronized and waits for all individual handlers before returning.

## Acceptance

Verified 100 actual TypeScript scenarios, 64 concurrent invocations, ordered concurrent item results, LRU eviction and authorization/callback error behavior; full verification passed 23 packaged modules/20 standalone consumers, both CGO-disabled Linux builds, 742/742 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.

The full-run AppSync archive preceded the concrete UnauthorizedException type-name correction. MODULE_ACCEPTANCE_SCOPED.json separately verifies the final AppSync source archive, tests/vet/tidy and standalone consumer; the Lambda binaries were built from the corrected source. Unaffected module suites and runtime invocations were not repeated.
