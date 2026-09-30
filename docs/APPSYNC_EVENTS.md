# AppSync Events

The independent `eventhandler/appsyncevents` module implements the pinned TypeScript v2.35.0 AppSync Events router and resolver contracts. It depends only on Commons and the Go standard library. The core uses Commons' 100-entry LRU and environment-string parsing; it does not import Parser, Logger, Tracer, HTTP routing or an AWS SDK.

```go
app := appsyncevents.New(appsyncevents.Options{WarnOnLargePayload: true})
app.OnPublish("/orders/*", func(ctx context.Context, payload any, event appsyncevents.Event) (any, error) {
    return payload, nil
})
app.OnSubscribe("/orders/private", func(ctx context.Context, event appsyncevents.Event) error {
    if event["identity"] == nil {
        return &appsyncevents.UnauthorizedError{Message: "Authentication required"}
    }
    return nil
})
lambda.Start(app.Resolve)
```

The [complete Go Lambda example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/appsyncevents/main.go) includes imports. A Go method value or closure provides the reference's decorated method/scope binding without an additional wrapper or dependency.

## Event and handler mapping

Resolve accepts an ordinary JSON-decoded Lambda event. `Event` aliases `map[string]any`, preserving unknown identity, request, stash and envelope properties. Lambda's decoder supplies nested maps and `[]any` arrays. If using an SDK struct directly, JSON-decode it before calling Resolve; arbitrary native objects and typed collections are not interpreted as JavaScript records/arrays.

The lightweight guard intentionally differs from Parser's complete AppSync Events schemas. It checks required envelope keys, object/array shapes, channel strings and PUBLISH/SUBSCRIBE operations, while permitting values such as non-null result/error/identity. A malformed PUBLISH message array falls through to subscription handling, matching the executable reference. Do not silently strengthen that guard with Parser validation. Applications may run the existing Parser schema explicitly, as the local Lambda integration does.

| Reference | Go |
| --- | --- |
| Router | Router / NewRouter |
| AppSyncEventsResolver | Resolver / New |
| onPublish(path, handler) | OnPublish(path, PublishHandler) |
| onPublish with aggregate | OnPublish with PublishOptions{Aggregate: true} |
| onSubscribe | OnSubscribe / SubscribeHandler |
| resolve(event, context) | Resolve(context.Context, event) |
| UnauthorizedException | UnauthorizedException / UnauthorizedError alias |
| Named Error subclass | NamedError or an error implementing ErrorName() string |
| Immediate undefined payload/result | Undefined{} |
| logger / warnOnLargePayload options | Options.Diagnostic / Options.WarnOnLargePayload |

PublishHandler receives the payload, full event and original context. In aggregate mode it receives the complete decoded `[]any` message list. Aggregate outputs remain caller-controlled, including null or non-array values accepted by the JavaScript runtime. SubscribeHandler returns only an error because the reference discards its return value. Individual publish handlers run concurrently, wait for all items and retain input order in the response. Treat the shared event and shared application state as read-only or synchronize mutations. The resolver does not modify input values.

## Routing and process state

Routes require slash-separated nonempty segments. A wildcard is permitted at the end as `/*`; it matches descendants, not the parent without the slash. Regex metacharacters in a registered path are literal. The longest matching path wins, discounting a trailing wildcard; equal specificity retains registration order. Specificity uses UTF-16 length. Publish and subscribe registries are independent.

Invalid routes warn and are skipped. Re-registering a path warns and replaces its registry entry without changing registration order. The reference's successful-match cache is not invalidated: a previously resolved path retains its old handler until LRU eviction. Missing matches are not cached, but their warnings are deduplicated by path and operation. Register routes before serving requests unless this process-state behavior is intentional. Registration and resolution are synchronized; diagnostic callbacks run outside locks.

An unmatched publish returns the original events, including extra message properties. An unmatched subscription returns the event's events value. Invalid events and successful subscriptions return nil, which Go Lambda encodes as JSON null; JavaScript undefined is not a separate top-level JSON value. A handler returning nil creates a JSON-null payload/events property, whereas Undefined{} omits that immediate property.

## Errors, context and diagnostics

Ordinary handler errors become `Error - message`; NamedError controls the name. Individual publish failures become `{id, error}` items, including UnauthorizedError. Aggregate/subscription UnauthorizedError propagates to the Lambda caller unchanged; ordinary aggregate/subscription errors become a top-level error envelope. The concrete type is UnauthorizedException, retaining the Go Lambda SDK's reflected errorType; UnauthorizedError is an alias. Go error unwrapping is supported for authorization and error names. Non-error panics in business callbacks map to the reference's unknown-error message. Diagnostic callback panics are rethrown on the resolving goroutine after all individual workers finish, so callers can recover without an unhandled worker panic.

The original context reaches each handler and resolution diagnostic, preserving Lambda request identity, cancellation, Logger invocation state and OTel spans. Cancellation is cooperative: handlers inspect context as they would in ordinary Go code. No new invocation identity or tracer backend is created.

Options.Diagnostic receives context, level, message and optional error. Adapt this callback to Logger or another logging implementation. It may run concurrently; synchronize any callback-owned state. Without a callback, warnings/errors go to stderr; debug goes to stdout only when the trimmed AWS_LAMBDA_LOG_LEVEL is exactly DEBUG. Registration diagnostics use context.Background. Exact console stack rendering and JavaScript-native error objects remain compatibility boundaries.

## Event size

WarnOnLargePayload defaults to false. When enabled, handled output items larger than 245760 bytes produce one warning per channel path per Router. Measurement includes the event ID. The returned events are not dropped or truncated by this library, and unmatched passthrough events are not measured. The measurement disables HTML escaping and accounts for JSON.stringify's unescaped U+2028/U+2029 bytes while preserving literal backslash escape text.

The reference fixture covers the exact limit, above-limit values, UTF-8 characters, HTML-sensitive text and Unicode line separators. Full JavaScript native serialization, nested undefined/non-finite/custom-marshaler values, malformed strings and byte-identical Go Lambda output remain separate gates in [the plan](APPSYNC_EVENTS_PLAN.md).

## Verification scope

Verified 100 actual TypeScript scenarios, 64 concurrent invocations, ordered concurrent item results, LRU eviction and authorization/callback error behavior; full verification passed 23 packaged modules/20 standalone consumers, both CGO-disabled Linux builds, 742/742 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.

The full-run AppSync archive preceded the concrete UnauthorizedException type-name correction. MODULE_ACCEPTANCE_SCOPED.json separately verifies the final AppSync source archive, tests/vet/tidy and standalone consumer; the Lambda binaries were built from the corrected source. Unaffected module suites and runtime invocations were not repeated.

The pinned reference generator produces 100 scenarios, many with repeated registration and resolution operations. It compares complete response shapes, warning/error text, cache behavior, handler arguments and context. Long strings are compared by exact UTF-8 length and SHA-256 rather than discarded. Concurrent calls and error logs use multiset comparison; route diagnostics and response order remain exact. Go-specific tests cover 64 concurrent invocations, concurrent item completion, input immutability, authorization identity, 100-entry cache eviction and callback panic propagation. This is scoped evidence, not an exhaustive parity or live AppSync service claim.
