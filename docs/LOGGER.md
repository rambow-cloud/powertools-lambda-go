# Logger

Logger emits structured JSON with Lambda context, service identity, correlation IDs, configurable levels, and optional buffering.

Import `github.com/rambow-cloud/powertools-lambda-go/logger`. It is an independent module; it does not bring in Tracer or JMESPath. See [installation](MODULES.md) and the [complete Lambda example](GETTING_STARTED.md#create-utilities-once).

## Invocation scope

Construct a root logger once, wrap the handler with `logger.WrapHandler`, and use `l.WithContext(ctx)` inside the handler:

~~~go
requestLog := l.WithContext(ctx)
requestLog.AppendKeys(logger.Fields{"order_id": event.OrderID})
if err := requestLog.Info("Processing order"); err != nil {
    log.Printf("Logging failed: %v", err)
}
~~~

This fragment assumes the logger, invocation context, and an application event with an `OrderID` field already exist. Request attributes, levels, and buffers remain isolated. Calling the root logger directly uses process-level state. Context-bound loggers reject writes after invocation completion; join background work before the handler returns.

With Tracer composition, wrap in this order:

~~~go
lambda.Start(tracer.WrapHandler(t, logger.WrapHandler(l, handler)))
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

Levels are `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`, `CRITICAL`, and `SILENT`. Environment level names are case-sensitive. Lambda advanced logging controls participate in effective filtering; see the [compatibility mapping](COMPATIBILITY.md).

WARN and higher use stderr; lower levels use stdout. `WithOutput` replaces both sinks. `WithFormatter`, `WithReplacer`, and `WithRecordOrder` customize serialization. Custom callbacks must support concurrent use.

## Attributes and correlation

Use `AppendKeys` for temporary fields and `AppendPersistentKeys` for persistent fields. Remove them with `RemoveKeys` or `RemovePersistentKeys`; `ResetKeys` clears temporary state. Initialize process-wide stable fields with `WithPersistentKeys`. Create a child logger with `Child` when a component needs its own configuration.

Set `logger.HandlerOptions.CorrelationSource` to a built-in source such as `logger.APIGatewayREST` or `logger.EventBridge`. For custom extraction, supply `CorrelationID`, or use a compiled [JMESPath](JMESPATH.md) expression as `CorrelationExtractor`. A callback takes precedence over an extractor, which takes precedence over a built-in source.

An active OpenTelemetry context supplies `trace_id`, `span_id`, and X-Ray-formatted `xray_trace_id`. This does not require the legacy X-Ray SDK.

## Sampling and buffering

Sampling uses the reference integer grid and occurs at construction. The first invocation reuses that decision; warm invocations resample independently.

Enable buffering explicitly:

~~~go
l := logger.New(
    logger.WithServiceName("orders"),
    logger.WithBuffer(logger.BufferOptions{
        MaxBytes: 20480,
        BufferAt: logger.DebugLevel,
    }),
)
~~~

Buffering needs a valid OpenTelemetry context or a Lambda X-Ray root ID. Unsampled OTel contexts are supported. Errors flush buffered logs by default; successful invocations discard remaining buffered entries. Use `FlushBuffer` or `ClearBuffer` for explicit control and `HandlerOptions.FlushBufferOnError` for handler-error flushing.

## Error handling and boundaries

Log methods return output or serialization errors. `WithErrorHandler` receives wrapper instrumentation errors without replacing the business result; its default callback ignores them. Do not recursively log through the same logger from that callback.

The API includes formatter/replacer hooks, timezones, event logging, child configuration, and buffering, but exhaustive TypeScript edge-case parity remains open. Consult the [Logger checklist](CHECKLIST.md#logger) and [compatibility guide](COMPATIBILITY.md) before relying on a particular boundary.
