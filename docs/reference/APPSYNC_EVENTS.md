# AppSync Events: API and behavior notes

Start with the [AppSync Events guide](../APPSYNC_EVENTS.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

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

Ordinary handler errors become `Error - message`; NamedError controls the name. Ordinary individual publish failures become `{id, error}` items; ordinary aggregate/subscription errors become a top-level error envelope. UnauthorizedError propagates to the Lambda caller unchanged in all publication modes and subscriptions, including wrapped errors and errors raised by business callback panics. Individual publication waits for every worker before returning an authorization error with no partial response; when several items deny authorization, it returns the first denial in input order. The concrete type is UnauthorizedException, retaining the Go Lambda SDK's reflected errorType; UnauthorizedError is an alias. Go error unwrapping is supported for authorization and error names. Non-error panics in business callbacks map to the reference's unknown-error message. Diagnostic callback panics are rethrown on the resolving goroutine after all individual workers finish, so callers can recover without an unhandled worker panic.

This intentionally corrects the non-aggregate authorization behavior in the pinned TypeScript the upstream baseline reference. The original reference fixture remains intact; its successful per-item authorization envelope is explicitly checked against the corrected invocation-error expectation. Ordinary error envelopes and empty-publication responses retain their original expectations.

The original context reaches each handler and resolution diagnostic, preserving Lambda request identity, cancellation, Logger invocation state and OTel spans. Cancellation is cooperative: handlers inspect context as they would in ordinary Go code. No new invocation identity or tracer backend is created.

Options.Diagnostic receives context, level, message and optional error. Adapt this callback to Logger or another logging implementation. It may run concurrently; synchronize any callback-owned state. Without a callback, warnings/errors go to stderr; debug goes to stdout only when the trimmed AWS_LAMBDA_LOG_LEVEL is exactly DEBUG. Registration diagnostics use context.Background. Exact console stack rendering and JavaScript-native error objects remain compatibility boundaries.


## Event size

WarnOnLargePayload defaults to false. When enabled, handled output items larger than 245760 bytes produce one warning per channel path per Router. Measurement includes the event ID. The returned events are not dropped or truncated by this library, and unmatched passthrough events are not measured. The measurement disables HTML escaping and accounts for JSON.stringify's unescaped U+2028/U+2029 bytes while preserving literal backslash escape text.

The reference fixture covers the exact limit, above-limit values, UTF-8 characters, HTML-sensitive text and Unicode line separators. Full JavaScript native serialization, nested undefined/non-finite/custom-marshaler values, malformed strings and byte-identical Go Lambda output remain separate gates in [the plan](../APPSYNC_EVENTS_PLAN.md).


## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `app` | Reusable route registry and match cache; handlers are registered once. |
| `payload` / `event` | One publish payload and the full invocation event; aggregate mode passes all items. |
| `ctx` | Original Lambda context; pass to optional Logger/Tracer and downstream calls. |

