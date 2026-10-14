# AppSync GraphQL: API and behavior notes

Start with the [AppSync GraphQL guide](../APPSYNC_GRAPHQL.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

## Public mapping

| TypeScript export or member | Go mapping |
| --- | --- |
| `AppSyncGraphQLResolver` | `Resolver`, `New`, `Resolve` |
| `Router` | `Router`, `NewRouter`, promoted registration methods |
| `resolver`, `onQuery`, `onMutation` | `OnResolver`, `OnQuery`, `OnMutation` |
| `batchResolver`, `onBatchQuery`, `onBatchMutation` | `OnBatchResolver`, `OnBatchQuery`, `OnBatchMutation` |
| `exceptionHandler` | `OnException`, exact error names and an `ExceptionHandler` |
| `includeRouter` | `IncludeRouter`, ordered variadic routers |
| `ResolverNotFoundException` | Same concrete exported type and Runtime API error name |
| `InvalidBatchResponseException` | Same concrete exported type and Runtime API error name |
| `awsDate`, `awsTime`, `awsDateTime` | `AWSDate`, `AWSTime`, `AWSDateTime` with explicit `time.Time` and optional offset |
| `awsTimestamp` | `AWSTimestamp(time.Time)` |
| `makeId` | `MakeID`, random UUID v4 |
| Decorators and `scope` binding | Go closures and bound method values |

`OnResolver` and `OnBatchResolver` take an explicit type name. Convenience methods
provide Query and Mutation defaults. Route keys retain the reference's literal
`typeName + "." + fieldName` concatenation. Duplicate registrations replace the
previous handler and emit a warning; single and batch routes remain separate.


## Batch and error behavior

Batch handlers aggregate by default. Both handler arguments contain the entire
event slice. Return a non-nil slice or array; response length is not forced to match
input length. Native top-level slices are materialized as JSON arrays, including
byte slices. Nil slices are rejected because they serialize as null.

`BatchOptions{Individual: true}` selects sequential execution. Only the first
event selects a route, even when later fields or type names differ. Each handler
receives that item's arguments and complete event. Failures log diagnostics and
append null, then processing continues. Add `ThrowOnError: true` to stop on the
first failure and send it through the resolver's outer exception handling.

Returned Go errors and error-valued panics participate in exception handling.
`ErrorName() string` supplies a custom name; ordinary errors use `Error`.
`NamedError` is a convenient implementation. Non-error panic values produce
`{"error":"An unknown error occurred"}` without invoking an exception handler.
Exception handlers match exact names. If an exception handler fails, diagnostics
record that failure and the response formats the original error. Missing-resolver
and invalid-batch exceptions always propagate; handlers cannot override them.
Go error wrapping is recognized through `errors.As`.

Invalid event shapes warn and return nil. The shape guard is intentionally lighter
than Parser's AppSync schema. An empty batch reproduces the reference's propagated
TypeError using a concrete Go type with the same Lambda Runtime API error name;
it does not silently succeed. Top-level JavaScript undefined maps to Go
nil. Go cannot reproduce arbitrary JavaScript prototype, Promise, decorator or
mutable scope behavior. Typed AWS SDK structs need explicit conversion to maps;
the resolver does not silently serialize arbitrary native objects.

`Options.Diagnostic` receives the invocation context for resolution and background
context for registration. It receives debug calls regardless of environment. The
default sink writes errors/warnings to stderr; debug output goes to stdout only
when Commons-trimmed `AWS_LAMBDA_LOG_LEVEL` equals `DEBUG`. An individual batch
failure uses an empty message with its error, matching the reference's error-only
diagnostic. Concurrent invocations may call the sink concurrently.


## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `app` | Reusable route/exception registry; no service client. |
| Callback arguments | `arguments` is the field argument value; `event` retains identity, source and resolver context. |
| `Router` | Split registrations into modules and include snapshots into the resolver. |

