---
description: "Resolve AWS AppSync GraphQL requests in Go Lambda with Powertools, including route dispatch, batching and error contracts."
---

# AppSync GraphQL

AppSync GraphQL routes resolver events and batches to Go callbacks. Import `github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncgraphql`. Register queries, mutations and other type/field pairs before serving invocations.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Install

Use Go 1.27 or newer and install the module in your own application:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncgraphql@v1.1.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## Complete example

Build this complete Lambda example at `./examples/appsyncgraphql` with `CGO_ENABLED=0`. Configure AppSync to send its resolver event to the Lambda data source; the example registers `Query.hello`.

~~~go
--8<-- "examples/appsyncgraphql/main.go"
~~~

## Input and output

The full AppSync event below returns `{"arguments":{"name":"Ada"},"message":"Hello"}`. This is the resolver value, not an HTTP proxy response or a log line. The callback receives both the `arguments` object and complete event. A missing route raises `ResolverNotFoundException`. Successful resolution does not automatically write an application log. Sending only `arguments` and the route names is not a valid resolver envelope.

~~~json
{
  "arguments": {"name": "Ada"},
  "identity": null,
  "source": null,
  "prev": null,
  "stash": {},
  "request": {"headers": {}, "domainName": null},
  "info": {"parentTypeName": "Query", "fieldName": "hello", "variables": {}}
}
~~~

## Common tasks

- [Register queries and mutations](#public-mapping).
- [Handle batches and errors](#batch-and-error-behavior).
- [Use scalar helpers](#scalars-and-evidence).

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

## Scalars and evidence

Scalar helpers accept an explicit clock value for reproducible output. Offsets
use hours, including fractions, in the inclusive range -12 to +14. Nonzero offset
suffixes retain the reference's seconds component, for example `+05:30:00`.
Date-only output also includes its suffix. Formatting preserves milliseconds,
un-padded years, JavaScript Date clipping and NaN offset behavior in the tested
range. Timestamp seconds floor negative instants. Go nanosecond inputs, instants
outside the JavaScript Date domain and arbitrary native serialization remain
explicit compatibility audit items.

`tools/reference/generate-appsync-graphql.mjs` executes actual pinned public
exports: 114 resolver scenarios and 91 scalar cases. Go checks ordered calls,
diagnostics, values and errors, plus 64 concurrent contexts, sequential batches,
mutation ownership, callback reentrancy, panic/error identity and UUID structure.
These checks do not establish live AppSync service or complete language parity.
Remaining acceptance requirements are tracked in [APPSYNC_GRAPHQL_PLAN.md](APPSYNC_GRAPHQL_PLAN.md).
## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `app` | Reusable route/exception registry; no service client. |
| Callback arguments | `arguments` is the field argument value; `event` retains identity, source and resolver context. |
| `Router` | Split registrations into modules and include snapshots into the resolver. |

## TypeScript feature coverage

??? info "Compare with TypeScript v2.35.0"

    Compared with the [official v2.35.0 appsync-graphql guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/event-handler/appsync-graphql.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

    | TypeScript feature | Go API or approach | Compatibility scope |
    | --- | --- | --- |
    | Resolver / nested mappings | `OnQuery`, `OnMutation`, `OnResolver` | Explicit type/field keys replace decorators and scope binding. |
    | Split routers | `NewRouter`, `IncludeRouter` | Snapshots included routes. |
    | Batch resolution | `OnBatchResolver`, batch options | Aggregated or sequential individual processing with explicit error policy. |
    | Exception handling | `OnException`, named errors | Errors remain distinct from resolver-not-found/invalid-batch exceptions. |
    | Scalars | `AWSDate`, `AWSTime`, `AWSDateTime`, `AWSTimestamp`, `MakeID` | Explicit Go clock values; native Date boundaries differ. |
    | Lambda context / logging | Callback `ctx`, `Options.Diagnostic` | Optional Logger; no implicit structured business log. |

    Executable evidence: [eventhandler/appsyncgraphql/reference_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/eventhandler/appsyncgraphql/reference_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.
