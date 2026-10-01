# Logger

Logger emits structured JSON with Lambda context, service identity, correlation IDs, configurable levels, and optional buffering.

Import `github.com/rambow-cloud/powertools-lambda-go/logger`. It is an independent module; it does not bring in Tracer or JMESPath. See [installation](MODULES.md) and the [complete Lambda example](GETTING_STARTED.md#create-utilities-once).

## Write your first log

This complete program runs locally without Lambda or Tracer. Save it as `main.go` in a directory that uses the repository's Go workspace, then run it with CGO disabled as described in [Getting Started](GETTING_STARTED.md).

~~~go
package main

import (
	stdlog "log"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))

	if err := appLog.Info("Processing order", logger.Fields{
		"order_id": "ORD-123",
		"amount":   42,
	}); err != nil {
		stdlog.Printf("Logging failed: %v", err)
	}
}
~~~

With the default `INFO` level, no sampling, and `POWERTOOLS_DEV` disabled, this writes one JSON line to **stdout**:

~~~json
{"level":"INFO","message":"Processing order","timestamp":"2026-10-01T00:00:00.000Z","service":"orders","sampling_rate":0,"amount":42,"order_id":"ORD-123"}
~~~

The timestamp is illustrative; your run uses the current time. `level`, `message`, `timestamp`, `service`, and `sampling_rate` are added by Logger. The fields you supply become top-level JSON properties, so `amount` is a number and `order_id` is a string. Local logging without an invocation context does not add Lambda function or request fields.

## Objects and lifecycle

There are three different names in the example:

| Name | What it is | How to use it |
| --- | --- | --- |
| `logger` | The imported Powertools package | Create a logger with `logger.New(...)`; construct fields with `logger.Fields{...}` |
| `appLog` | A `*logger.Logger` object returned by `logger.New` | Write structured records with `appLog.Info(...)`, `appLog.Warn(...)`, or `appLog.Error(...)` |
| `stdlog` | Go's standard `log` package, imported under an explicit alias | Print a plain-text fallback if structured logging fails |

The variable name `appLog` is your choice. Naming it `log` would still work, but would hide an imported standard-library package with that name. The examples use `appLog` and `stdlog` to make their roles explicit.

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

For the Logger/Tracer combination in [Getting Started](GETTING_STARTED.md#create-utilities-once), put Tracer outside Logger:

~~~go
lambda.Start(tracer.WrapHandler(t, logger.WrapHandler(appLog, handler)))
~~~

## TypeScript feature coverage

Compared with the [official v2.35.0 Logger guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/logger.md). Each capability below exists in Go; exhaustive configuration, diagnostics and serialization parity remain open.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| Structured keys / messages / errors | `Info`, `Warn`, `Error` and other levels | Explicit message and Fields; Go error types/causes instead of JS stacks |
| Lambda context injection | `WrapHandler` plus `WithContext` | Typed functions; invocation state closes at completion |
| Log incoming event | `POWERTOOLS_LOGGER_LOG_EVENT`, wrapper option | Opt-in; separate event record |
| Correlation ID | `SetCorrelationID`, handler source/callback/extractor | Nine sources; compiled JMESPath is optional |
| Append / remove / reset attributes | Temporary and persistent key methods | Scoped state and merge precedence |
| Levels / ALC / suppression | `WithLevel`, `SetLevel`, `SilentLevel` | ALC precedence; filtered calls return nil |
| Buffer logs | `WithBuffer`, `FlushBuffer`, `ClearBuffer` | X-Ray root or OTel context; explicit cleanup policies |
| Reorder keys | `WithRecordOrder(keys...)` | Selected keys first; remaining fields use lexical order |
| Custom timezone | `TZ`, injected `WithClock` | Built-in timezone data and Go time formatting |
| Child loggers | `Child(options...)` | Snapshotted attributes and independent settings |
| Debug sampling | `WithSampleRate`, constructor/invocation decisions | Reference sampling grid; separate from trace sampling |
| Formatter / JSON replacer | `WithFormatter`, `WithReplacer` | Go callbacks and JSON representation |
| Test output | `WithOutput`, `WithClock` | Writer injection instead of console spies |

[JSON reference tests](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/logger/reference_test.go), [sampling tests](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/logger/sampling_test.go) and native regressions provide scoped evidence. See [feature comparison](FEATURE_PARITY.md) and [Logger progress](CHECKLIST.md#logger).

### Event logging and child configuration

Event logging is disabled by default. With `POWERTOOLS_LOGGER_LOG_EVENT=true`, the wrapper emits an additional event record before your business logs; it can include the entire payload. Set this before constructing `appLog`, or select the handler option. It does not change what `Info` means.

Create `componentLog := appLog.Child(logger.WithPersistentKeys(logger.Fields{"component":"payments"}))` when a component needs a stable field and independent settings. In a wrapped handler, bind it with `componentLog.WithContext(ctx)`. Child level/key changes do not mutate the parent's configuration. Treat retained nested values as immutable.

Use `WithRecordOrder("message", "level", "timestamp")` to put selected keys first. A formatter changes the record envelope; a replacer changes individual JSON values. Both callbacks must be concurrency-safe. Serialization failures return from the log call. These hooks do not emulate every JavaScript JSON.stringify value or stack diagnostic.

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

Levels are `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`, `CRITICAL`, and `SILENT`. Environment level names are case-sensitive. Lambda advanced logging controls participate in effective filtering; see the [compatibility mapping](COMPATIBILITY.md).

WARN and higher use stderr; lower levels use stdout. `WithOutput` replaces both sinks. `WithFormatter`, `WithReplacer`, and `WithRecordOrder` customize serialization. Custom callbacks must support concurrent use.

## Attributes and correlation

Use `AppendKeys` for fields reused by subsequent records and `logger.Fields` on a log call for fields used by that record only. Remove temporary fields with `RemoveKeys`; `ResetKeys` clears them. Initialize stable process-wide fields with `WithPersistentKeys`. `AppendPersistentKeys` modifies the persistent set of the logger's current scope; calling it on `requestLog` does not persist a value into later invocations. Remove those keys with `RemovePersistentKeys`. Create an independent child configuration with `Child` when a component needs it.

For the same field name, a call's fields override temporary fields, which override persistent fields. Logger owns the reserved `level`, `message`, `timestamp`, `service`, and `sampling_rate` properties; attempts to override them are dropped with a warning.

Set `logger.HandlerOptions.CorrelationSource` to a built-in source such as `logger.APIGatewayREST` or `logger.EventBridge`. For custom extraction, supply `CorrelationID`, or use a compiled [JMESPath](JMESPATH.md) expression as `CorrelationExtractor`. A callback takes precedence over an extractor, which takes precedence over a built-in source.

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

## Error handling and boundaries

Log methods return output or serialization errors. Handle the return from direct calls such as `requestLog.Info(...)` explicitly: `WithErrorHandler` does not automatically handle those returned errors. It receives wrapper instrumentation errors, such as event-log output or correlation-extractor failures, without replacing the business result; its default callback ignores them. Do not recursively log through the same logger from that callback.

The API includes formatter/replacer hooks, timezones, event logging, child configuration, and buffering, but exhaustive TypeScript edge-case parity remains open. Consult the [Logger checklist](CHECKLIST.md#logger) and [compatibility guide](COMPATIBILITY.md) before relying on a particular boundary.
