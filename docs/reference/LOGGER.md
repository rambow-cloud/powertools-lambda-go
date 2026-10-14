# Logger: API and behavior notes

Start with the [Logger guide](../LOGGER.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

## Messages, fields, and errors

Each logging method accepts a message followed by optional attributes or an error:

~~~go
// Reuse appLog from the program above.
if err := appLog.Info("Order accepted", logger.Fields{"order_id": "ORD-123"}); err != nil {
    stdlog.Printf("Logging failed: %v", err)
}
~~~

The first argument is the `message` string. `logger.Fields` is a `map[string]any`; it supplies named JSON fields. Prefer fields for values you want to search or filter. These methods do not perform `Printf` substitution: `appLog.Info("Order %s", "ORD-123")` leaves `message` as `"Order %s"` and writes the second string under `extra`. Use a message and fields, or format the message yourself with `fmt.Sprintf`.

To record a business error, import Go's `errors` package and pass the error after the message:

~~~go
paymentErr := errors.New("card declined")
if logErr := appLog.Error("Payment failed", paymentErr, logger.Fields{
    "order_id": "ORD-123",
}); logErr != nil {
    stdlog.Printf("Logging failed: %v", logErr)
}
~~~

With default configuration, this writes the following JSON line to **stderr**:

~~~json
{"level":"ERROR","message":"Payment failed","timestamp":"2026-10-01T00:00:00.000Z","service":"orders","sampling_rate":0,"error":{"location":"","message":"card declined","name":"*errors.errorString"},"order_id":"ORD-123"}
~~~

The `error` object describes the supplied `paymentErr`: `name` is its concrete Go type, `message` comes from `Error()`, and `location` is empty for this ordinary Go error. Wrapped errors can also include a `cause`.

`logErr` has a different role: it reports a failure to serialize or write the log record. A successfully written `ERROR` record normally returns `nil`. Writing an error-level record does not return the business error from your handler, stop the program, or retry an operation; your application controls those decisions.

At the default `INFO` threshold, `Trace` and `Debug` calls produce no output. Debug sampling can enable `Debug`; `Trace` still requires the `TRACE` threshold. `Info`, `Warn`, `Error`, and `Critical` are eligible to print. A filtered call returns `nil`; it does not mean a record was emitted. Set `logger.WithLevel(logger.DebugLevel)` at construction for debug logs without sampling, or `logger.TraceLevel` for trace logs. Buffering, when enabled, can defer output; see [buffering](#sampling-and-buffering).


## Use Logger in a Lambda handler

Create a root logger once, wrap the handler with `logger.WrapHandler`, and obtain a request logger with `appLog.WithContext(ctx)`. This complete Logger-only example accepts an order and writes two records:

~~~go
package main

import (
	"context"
	stdlog "log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

type Event struct {
	OrderID string `json:"order_id"`
	Items   int    `json:"items"`
}

type Response struct {
	Status string `json:"status"`
}

func main() {
	appLog := logger.New(
		logger.WithServiceName("orders"),
		logger.WithErrorHandler(func(err error) {
			stdlog.Printf("Logger wrapper failed: %v", err)
		}),
	)

	handler := func(ctx context.Context, event Event) (Response, error) {
		requestLog := appLog.WithContext(ctx)
		requestLog.AppendKeys(logger.Fields{"order_id": event.OrderID})

		if err := requestLog.Info("Processing order"); err != nil {
			stdlog.Printf("Logging failed: %v", err)
		}
		if err := requestLog.Info("Order accepted", logger.Fields{
			"items": event.Items,
		}); err != nil {
			stdlog.Printf("Logging failed: %v", err)
		}

		return Response{Status: "accepted"}, nil
	}

	lambda.Start(logger.WrapHandler(appLog, handler))
}
~~~

Invoke the function with:

~~~json
{"order_id":"ORD-123","items":3}
~~~

The handler returns `{"status":"accepted"}` and, with default logging settings, writes two `INFO` records to stdout. For a first invocation of a function named `orders` with 128 MiB of memory, their shape is:

~~~json
{"level":"INFO","message":"Processing order","timestamp":"2026-10-01T00:00:00.000Z","service":"orders","cold_start":true,"function_arn":"arn:aws:lambda:ap-east-1:123456789012:function:orders","function_memory_size":128,"function_name":"orders","function_request_id":"00000000-0000-4000-8000-000000000001","sampling_rate":0,"order_id":"ORD-123"}
{"level":"INFO","message":"Order accepted","timestamp":"2026-10-01T00:00:00.000Z","service":"orders","cold_start":true,"function_arn":"arn:aws:lambda:ap-east-1:123456789012:function:orders","function_memory_size":128,"function_name":"orders","function_request_id":"00000000-0000-4000-8000-000000000001","sampling_rate":0,"items":3,"order_id":"ORD-123"}
~~~

These are illustrative application records with synthetic identifiers. Timestamps, function identity, memory configuration, and request ID come from your actual invocation. Both records share the invocation's `cold_start` value; it becomes `false` on a later warm invocation. Runtime trace context can add `xray_trace_id`; an active OpenTelemetry span adds `trace_id` and `span_id` as well.

`logger.WrapHandler` establishes the invocation scope; `WithContext` binds your object to that scope. Both are needed for invocation-local state. Merely calling `WithContext(context.Background())` outside a wrapped invocation does not create an isolated request scope. Use `requestLog` for handler logs and keep `appLog` for construction and process-level logging.

`AppendKeys` adds `order_id` to every subsequent record written through that request's logger. The `items` field passed to the second `Info` call appears only on that record. A later invocation gets fresh request state, so the previous order's fields do not leak into it. Context-bound loggers reject writes after invocation completion; join background work before the handler returns.

For the Logger/Tracer combination in [Getting Started](GETTING_STARTED.md#add-tracing), put Tracer outside Logger:

~~~go
lambda.Start(tracer.WrapHandler(t, logger.WrapHandler(appLog, handler)))
~~~


## Configuration

| Setting | Default | Behavior |
| --- | --- | --- |
| `WithServiceName` / `POWERTOOLS_SERVICE_NAME` | `service_undefined` | Service field |
| `WithLevel` / `POWERTOOLS_LOG_LEVEL` | `INFO` | Application threshold; `LOG_LEVEL` is the environment fallback |
| `WithSampleRate` / `POWERTOOLS_LOGGER_SAMPLE_RATE` | `0` | Probability of invocation-level debug sampling |
| `POWERTOOLS_LOGGER_LOG_EVENT` | `false` | Emit the incoming event when enabled |
| `POWERTOOLS_DEV` | `false` | Pretty-print output for development |
| `TZ` | UTC | Timestamp timezone |
| `AWS_LAMBDA_LOG_LEVEL` | Unset | Lambda advanced logging control threshold |

Levels are `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`, `CRITICAL`, and `SILENT`. Environment level names are case-sensitive. Lambda advanced logging controls participate in effective filtering; see the [compatibility mapping](../COMPATIBILITY.md).

WARN and higher use stderr; lower levels use stdout. `WithOutput` replaces both sinks. `WithFormatter`, `WithReplacer`, and `WithRecordOrder` customize serialization. Custom callbacks must support concurrent use.


## Attributes and correlation

Use `AppendKeys` for fields reused by subsequent records and `logger.Fields` on a log call for fields used by that record only. Remove temporary fields with `RemoveKeys`; `ResetKeys` clears them. Initialize stable process-wide fields with `WithPersistentKeys`. `AppendPersistentKeys` modifies the persistent set of the logger's current scope; calling it on `requestLog` does not persist a value into later invocations. Remove those keys with `RemovePersistentKeys`. Create an independent child configuration with `Child` when a component needs it.

For the same field name, a call's fields override temporary fields, which override persistent fields. Logger owns the reserved `level`, `message`, `timestamp`, `service`, and `sampling_rate` properties; attempts to override them are dropped with a warning.

Before JSON replacement, Logger removes top-level empty strings and null values
from a map-shaped formatted document, including an empty `message`. Zero, false,
nonnil empty collections, and nested empty/null values remain. A replacer can
still produce empty strings or null values: cleanup has already happened at that
point. Cleanup does not mutate caller or formatter maps. Custom JSON/text marshalers
and struct-shaped formatter results retain their Go encoding rules.
An attribute implementing `json.Marshaler`, `json.MarshalerTo` from `encoding/json/v2`, or `encoding.TextMarshaler` is preserved
even when its underlying string is empty; its custom value or serialization error
is still observed. Explicitly marshaled empty text remains a JSON empty string.
JSON v2 prefers `MarshalJSONTo`, then `MarshalJSON`, then text marshaling. Serialization errors retain their wrapped cause and use v2 error types. Struct attributes use v2 field-tag, zero-value and collection rules. Logger's top-level cleanup and circular/truncated markers remain utility policies.
Text marshalers on maps, slices and pointers keep their scalar output or error;
Logger does not traverse their underlying containers. If JSON normalization for
a replacer fails, the log call returns that first error without invoking the
marshaler again or writing a partial record.

Set `logger.HandlerOptions.CorrelationSource` to a built-in source such as `logger.APIGatewayREST` or `logger.EventBridge`. For custom extraction, supply `CorrelationID`, or use a compiled [JMESPath](../JMESPATH.md) expression as `CorrelationExtractor`. A callback takes precedence over an extractor, which takes precedence over a built-in source.

An active OpenTelemetry context supplies `trace_id`, `span_id`, and X-Ray-formatted `xray_trace_id`. This does not require the legacy X-Ray SDK.


## Sampling and buffering

Sampling uses the reference integer grid and occurs at construction. The first invocation reuses that decision; warm invocations resample independently.

Enable buffering explicitly:

~~~go
appLog := logger.New(
    logger.WithServiceName("orders"),
    logger.WithBuffer(logger.BufferOptions{
        MaxBytes: 20480,
        BufferAt: logger.DebugLevel,
    }),
)
~~~

Buffering needs a valid OpenTelemetry context or a Lambda X-Ray root ID. Unsampled OTel contexts are supported. Errors flush buffered logs by default; successful invocations discard remaining buffered entries. Use `FlushBuffer` or `ClearBuffer` for explicit control and `HandlerOptions.FlushBufferOnError` for handler-error flushing.

The pending buffer belongs to a trace. `FlushBuffer` and `ClearBuffer` operate
only on that active trace and do nothing when the trace is absent or differs.
The first buffered record for a new trace discards the previous trace's leftovers,
including its eviction flag. Span changes within one OTel trace retain pending
records; OTel trace identity takes precedence over runtime headers. Separate
wrapped invocations and child loggers keep independent buffers. Unwrapped root
loggers share process state, so use `WrapHandler` and `WithContext` for concurrent
requests.

With `FlushBufferOnError`, wrapper cleanup flushes the buffers owned by that
invocation even if the handler attached a new OTel trace after wrapper entry.
This also applies to participating child loggers and panic cleanup. The original
handler result, error, or panic is preserved; successful invocations discard
pending buffers.

Capacity counts serialized UTF-8 bytes without the trailing newline. An entry
larger than `MaxBytes` emits a warning containing an `error` with message
`"Item too big"`, followed by the original record even below the application
threshold. Error names and locations follow Go conventions. Sink failures remain
returned errors; wrapper diagnostics preserve the original handler result.

### Source parity fixes

The pinned-source audit identified these independently observable differences:

| Issue | Previous Go behavior | Current behavior |
| --- | --- | --- |
| [#3](https://github.com/rambow-cloud/powertools-lambda-go/issues/3) | Child inherited temporary keys as persistent keys | Separate snapshots preserve reset/removal and temporary precedence |
| [#4](https://github.com/rambow-cloud/powertools-lambda-go/issues/4) | Empty top-level strings/null values were emitted | Shallow cleanup runs after formatting and before replacement |
| [#5](https://github.com/rambow-cloud/powertools-lambda-go/issues/5) | Flush/clear ignored trace changes; oversize warnings omitted the error | Active-trace lifecycle and overflow error details follow the reference |

The development-only corpus is regenerated with `npm run fixtures:logger:parity`
from `tools/reference`. It fixes the clock for byte-capacity checks and normalizes
timestamps and native overflow-error details as described in
[the fixture notes](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/logger/testdata/README.md).


## Error handling and boundaries

Log methods return output or serialization errors. Handle the return from direct calls such as `requestLog.Info(...)` explicitly: `WithErrorHandler` does not automatically handle those returned errors. It receives wrapper instrumentation errors, such as event-log output or correlation-extractor failures, without replacing the business result; its default callback ignores them. Do not recursively log through the same logger from that callback.

The API includes formatter/replacer hooks, timezones, event logging, child configuration, and buffering, but exhaustive TypeScript edge-case parity remains open. Consult the [Logger checklist](../CHECKLIST.md#logger) and [compatibility guide](../COMPATIBILITY.md) before relying on a particular boundary.

## Objects and lifecycle

| Name | What it is | How to use it |
| --- | --- | --- |
| `logger` | The imported Powertools package | Create a logger with `logger.New(...)`; construct fields with `logger.Fields{...}` |
| `appLog` | A `*logger.Logger` object returned by `logger.New` | Write structured records with `appLog.Info(...)`, `appLog.Warn(...)`, or `appLog.Error(...)` |
| `stdlog` | Go's standard `log` package, imported under an explicit alias | Print a plain-text fallback if structured logging fails |



### Event logging and child configuration

Event logging is disabled by default. With `POWERTOOLS_LOGGER_LOG_EVENT=true`, the wrapper emits an additional event record before your business logs; it can include the entire payload. Set this before constructing `appLog`, or select the handler option. It does not change what `Info` means.

`WrapHandler` logs the Go value it receives. When the AWS runtime first decodes
JSON into a struct, members absent from that struct are discarded; serializing
the struct can also omit fields with `omitempty` or `json:"-"` tags. For example,
`{"name":"Alice","age":30}` becomes `{"name":"Alice"}` when the input struct
only declares `name`. The logger cannot reconstruct discarded values.

Register `WrapRawHandler` (available in Logger v1.1.0 and later) at the runtime
entry to log the complete incoming JSON value before decoding a typed business input:

~~~go
type greetingEvent struct {
    Name string `json:"name"`
}

handler := func(ctx context.Context, event greetingEvent) (string, error) {
    return event.Name, appLog.WithContext(ctx).Info("Greeting accepted")
}
enabled := true
lambda.Start(logger.WrapRawHandler(appLog, handler, logger.HandlerOptions{
    LogEvent: &enabled,
}))
~~~

The returned function accepts `encoding/json.RawMessage`. Its event record
contains `"event":{"name":"Alice","age":30}`, while the business handler
receives `greetingEvent{Name: "Alice"}`. Unknown members, nested values, empty
strings, JSON nulls and exact numeric tokens are retained in event logs. JSON
whitespace and member order are not preserved. Existing formatter/replacer
policies can transform or redact the event, and `LogEvent: false` or a level
above INFO suppresses the event record as usual. Review payload contents before
enabling event logging.

Typed decoding uses `encoding/json/v2` defaults, including case-sensitive field
names. Event logging runs before typed decoding; a decoding error
returns without calling the business handler. Invalid JSON rejected by the AWS
runtime never reaches the wrapper. Runtime `WithUseNumber` and
`WithDisallowUnknownFields` options apply to its RawMessage entry, not to the
subsequent typed decoder. If you need custom decoding options, use
`WrapHandler` around a RawMessage function and decode inside that function.
Raw-wrapper correlation callbacks and extractors receive RawMessage; built-in
correlation sources and this project's compiled JMESPath queries support it.

Keep event logging outside Parser/Validation when you need the incoming payload.
For an existing raw-input pipeline, compose
`logger.WrapHandler(appLog, parser.WrapHandler[json.RawMessage](schema, handler), options)`
or the equivalent `validation.WrapHandler[json.RawMessage]` pipeline. Here `json`
refers to `encoding/json`. Placing Logger inside these wrappers records their
parsed or validated value, which may already omit original fields. Tracer can
remain outside Logger; all these wrappers share the invocation identity.
See the executable [raw-event example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/logger/example_test.go)
and [runtime-entry regressions](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/logger/raw_handler_test.go).

Create `componentLog := appLog.Child(logger.WithPersistentKeys(logger.Fields{"component":"payments"}))` when a component needs a stable field and independent settings. In a wrapped handler, bind it with `componentLog.WithContext(ctx)`. Child level/key changes do not mutate the parent's configuration. Treat retained nested values as immutable.

`Child` snapshots persistent and temporary fields separately. Child persistent
options merge into the persistent snapshot; inherited temporary values keep their
usual precedence. For example, if the parent has persistent `shared: "parent"`
and temporary `shared: "request"`, a child configured with persistent
`shared: "child"` initially logs `"request"`. After the child's `ResetKeys`, it
logs `"child"`. Inherited request-only keys disappear on reset and are excluded
from `PersistentKeys`. Resetting a child does not change the parent.

Use `WithRecordOrder("message", "level", "timestamp")` to put selected keys first. A formatter changes the record envelope; a replacer changes individual JSON values. Both callbacks must be concurrency-safe. Serialization failures return from the log call. These hooks do not emulate every JavaScript JSON.stringify value or stack diagnostic.


## Finite boundary audit (#181)

This local audit reuses the existing 48 TypeScript 2.35.0 Logger action cases and
sampling/configuration regressions, then adds 26 actual upstream configuration,
timezone and sink cases. No AWS calls are needed to regenerate or run them.

| Boundary / APIs | Evidence | Classification |
| --- | --- | --- |
| `WithFormatter`, `WithReplacer`, record cleanup/order | Existing formatter, pointer/cycle, native struct, streaming marshaler and 18 cleanup action cases | Verified scoped traversal, precedence, owned snapshots and error identity |
| Native text marshaler containers and formatter documents | `TestNativeTextMarshalerContainers` | Reproduced and fixed ignored map/slice text methods, including discarded errors; retain native JSON v2 method precedence |
| JSON/streaming marshaler failure during replacement | `TestFailedJSONNormalizationDoesNotRetryMarshaler` | Reproduced and fixed duplicate marshaler calls; the first error propagates before output |
| Timezones and environment levels/rates | `TestLoggerConfigurationBoundaryReference`: eight zones in winter/summer and ten environment cases | Verified DST, fractional offsets, invalid-zone fallback, level alias/precedence and ALC `FATAL` |
| Diagnostics and zero offsets | Preserved actual upstream records/diagnostics in the same corpus | Intentional Go differences below; no unexplained mismatch is discarded |
| Level sinks, `WithOutput`, `Child` output and short writes | Existing six-level routing plus `TestSinkShortWriteAndChildRoute` | Verified stdout/stderr thresholds, independent child override and `io.ErrShortWrite` |
| Child key stores and invocation cleanup | Existing twelve child action cases, `TestChildTemporaryKeysInInvocation`, 40 concurrent buffered invocations and raw-wrapper lifecycle tests | Verified persistent/temporary precedence, separate stores, closure and original handler result/error/panic |
| UTF-8 buffer sizes and lifecycle | Existing eighteen buffer action cases, including exact/one-byte-small/two-entry UTF-8 capacities, oversized trace replacement and native OTel tests | Verified byte accounting, eviction, trace ownership, flush/clear and output errors |

Native differences remain explicit: Go's stable `New` API falls back to a zero
sample rate for nonnumeric environment input, while the pinned constructor throws
`TypeError`. Invalid timezone warnings are structured Go WARN records emitted at
construction; the reference emits a plain console warning when formatting.
Go's RFC3339 zero offset is `Z`, whereas Intl uses `+00:00` for London in winter
and invalid-zone fallback. Those three cases have explicit expected Go output;
the original TypeScript values remain in the fixture. Go marshalers, errors and
OTel identity retain their documented native contracts.

The affected packaged module passes tests, vet, tidy/isolation and an independent
consumer with CGO disabled. L-08/L-09 still describe broader exhaustive API,
environment, timezone-database and serialization acceptance; this finite audit
does not claim that every native value, POSIX timezone alias, JavaScript diagnostic
or concurrent callback implementation is equivalent. Resource budgets remain #200,
and native multi-worker/platform acceptance remains #197/#202.
