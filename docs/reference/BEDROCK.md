# Bedrock Agent function resolver: API and behavior notes

Start with the [Bedrock Agent function resolver guide](../BEDROCK.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

## Public mapping

| TypeScript | Go |
| --- | --- |
| `BedrockAgentFunctionResolver` | `Resolver`, `New`, `Tool`, `Resolve` |
| `BedrockFunctionResponse` | `FunctionResponse`, `NewFunctionResponse`, `Build` |
| `Configuration` | `Configuration{Name, Description}` |
| `ToolFunction` | `ToolHandler(context.Context, *Parameters, Event)` |
| Parameter dictionary | `Parameters.Get/Has/Set/Delete/Keys/Values` |
| `FAILURE`, `REPROMPT` response states | `Failure`, `Reprompt` constants |
| Optional JavaScript undefined | `Undefined` marker where absence differs from null |

Tools are selected by function name alone. Duplicate registration warns and
replaces the previous handler. The action group is copied into the response, not
used as an additional route key. Description is accepted without generating a
schema or making a service call, matching the reference's runtime behavior.


## Parameters and bodies

Boolean conversion is exactly `value == "true"`; whitespace and capitalization
are not normalized. Number and integer parameters both use JavaScript Number
conversion through `commons.ParseNumber`. Fractional integer inputs stay
fractional. Invalid numbers retain their original strings; infinities remain
numeric values in the handler and become null during JSON body serialization.
Array and unknown parameter types retain their original strings.

`Parameters` preserves insertion order and canonical numeric-key order using
`commons.SortObjectKeys`. Duplicate parameters replace values without moving their
keys. Primitive assignments to the inherited `__proto__` parameter are ignored,
as in the reference. `Get` returns nil for a missing key; `Has` distinguishes
absence. `Values` returns a shallow map copy for application parsing and other Go
APIs. Return the parameter object itself when body key order matters; converting
it to a Go map discards insertion order. `Set` is an explicit Go own-key operation,
not an emulation of JavaScript prototypes.

Ordinary results are serialized into the response's string-valued
`response.functionResponse.responseBody.TEXT.body`. A string result therefore
contains JSON quotes. Nil, native typed nil, `Undefined` and an empty string produce
an empty body. Decoded JSON arrays/objects handle nested undefined, non-finite
numbers, negative zero, HTML characters and literal Unicode separators. Go maps
use deterministic lexical ordering for non-index keys; ordered parameters retain
the original order. Native structs, custom marshalers and raw JSON retain their
Go serialization contracts. Returning an explicit `FunctionResponse` bypasses
ordinary body encoding, allowing a caller-supplied string verbatim.

```go
response := bedrock.NewFunctionResponse("Please supply a valid order ID")
response.ResponseState = bedrock.Reprompt
response.SessionAttributes = map[string]any{"attempt": "2"}
return response, nil
```

Ordinary results, missing-tool responses and execution failures inherit the event's
session, prompt-session and knowledge-base references captured before the handler
runs. Replacing a top-level event field during the handler does not replace these
captured references; mutating the referenced object remains visible. Explicit
responses use their own fields. `NewFunctionResponse` defaults both session maps
to empty objects; nil omits them. Empty non-nil maps and slices remain present
under JavaScript conditional semantics. Root Commons `IsTruthy` intentionally has
different empty-container semantics and is not used here.


## Errors and diagnostics

A missing tool returns an error body without setting `responseState`. Returned
errors, error-valued panics and serialization failures become execution-error
bodies. `ErrorName() string` supplies a custom name; ordinary Go errors use `Error`.
`NamedError` and Go error wrapping are supported. Non-error panics use JavaScript
string conversion for the covered JSON values. Diagnostic callback failures are
not swallowed. A non-error `panic(nil)` is not a JavaScript throw-null equivalent
because modern Go converts it to a runtime error.

`Options.Diagnostic` receives registration callbacks with `context.Background`
and execution callbacks with the invocation context. Callbacks may run
concurrently. The default sink writes errors/warnings to stderr; debug output goes
to stdout only when Commons-trimmed `AWS_LAMBDA_LOG_LEVEL` equals `DEBUG`.


## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `app` | Reusable tool registry; `Tool` binds a function name to a callback. |
| `parameters` | Ordered converted parameters; `Get` reads a value, `Has` distinguishes missing from null. |
| `FunctionResponse` | Explicit body, session attributes and `Failure`/`Reprompt` response state. |

