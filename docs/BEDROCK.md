---
description: "Resolve Amazon Bedrock Agent function requests in Go Lambda with Powertools, typed inputs, routing and response contracts."
---

# Bedrock Agent function resolver

The Bedrock function resolver routes function-based Action Group invocations to registered Go tools. Import `github.com/rambow-cloud/powertools-lambda-go/eventhandler/bedrock`. It does not implement an OpenAPI Action Group router or call a model.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Install

Use Go 1.27 or newer and install the module in your own application:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/eventhandler/bedrock@v1.1.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## Complete example

Build the complete example at `./examples/bedrock` with `CGO_ENABLED=0`. Configure a function-based Action Group whose `greeting` function supplies a `name` parameter.

~~~go
--8<-- "examples/bedrock/main.go"
~~~

## Input and output

For the input below, the resolver returns the Bedrock envelope with `messageVersion: "1.0"`, the same `actionGroup` and `function`, and `response.functionResponse.responseBody.TEXT.body` containing `"Hello, Ada"` (including its quote characters). The inner quotes are part of that body because an ordinary Go string result is JSON-encoded. Return `NewFunctionResponse("Hello, Ada")` when the body should be verbatim text. The example writes no successful business log.

~~~json
{
  "messageVersion": "1.0",
  "agent": {
    "name": "orders-agent",
    "id": "AGENT12345",
    "alias": "ALIAS12345",
    "version": "DRAFT"
  },
  "inputText": "Greet Ada",
  "sessionId": "example-session",
  "actionGroup": "orders",
  "function": "greeting",
  "parameters": [
    {
      "name": "name",
      "type": "string",
      "value": "Ada"
    }
  ],
  "sessionAttributes": {},
  "promptSessionAttributes": {}
}
~~~

## Common tasks

- [Register an Action Group tool](#public-mapping).
- [Read parameters and request bodies](#parameters-and-bodies).
- [Return errors to the agent](#errors-and-diagnostics).

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

## Evidence and remaining boundaries

??? info "Reference evidence and compatibility details"

    The generator executes actual pinned public exports in 371 scenarios. Tests
    compare complete response envelopes, body strings, ordered calls and diagnostics;
    they do not parse or reorder body strings to hide wire differences. Non-finite
    values and negative zero use explicit tags only in recorded handler arguments.
    Native tests cover 64 simultaneous invocation contexts, cancellation propagation,
    input ownership, reentrant diagnostics, error identity, nil results, cycle handling
    and ordered parameter mutation.

    Full declaration/native-type mapping, arbitrary object/prototype/Promise behavior,
    typed event adapters, exact circular-error diagnostics, native struct/marshaler
    serialization, uncommon numeric representations and malformed UTF-16/UTF-8 remain
    compatibility gates. Go map insertion order cannot be reconstructed after it is
    lost. Live Bedrock service acceptance, performance and publication remain separate
    requirements in [BEDROCK_PLAN.md](BEDROCK_PLAN.md).

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `app` | Reusable tool registry; `Tool` binds a function name to a callback. |
| `parameters` | Ordered converted parameters; `Get` reads a value, `Has` distinguishes missing from null. |
| `FunctionResponse` | Explicit body, session attributes and `Failure`/`Reprompt` response state. |

## TypeScript feature coverage

??? info "Compare with TypeScript v2.35.0"

    Compared with the [official v2.35.0 bedrock-agents guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/event-handler/bedrock-agents.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

    | TypeScript feature | Go API or approach | Compatibility scope |
    | --- | --- | --- |
    | Tools / parameter conversion | `Tool`, `Parameters` | Function-name routing; covered JavaScript number/boolean conversion. |
    | Context / event access | Callback `ctx` and `Event` | Request-owned parameter objects; input ownership documented below. |
    | Error handling | Execution error bodies, named errors and response states | Not all failures propagate as a Lambda error. |
    | Session attributes | `FunctionResponse` | Explicit session and prompt-session updates. |
    | Logging | `Options.Diagnostic` | Optional sink; successful tool output is not a log record. |

    Executable evidence: [eventhandler/bedrock/reference_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/eventhandler/bedrock/reference_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.
