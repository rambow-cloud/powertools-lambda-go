---
title: Structured logging for Go Lambda
description: "Write structured JSON logs in Go Lambda with context, correlation IDs, configurable levels and buffering using Powertools Logger."
---

# Logger

Write JSON logs with searchable fields, service names and Lambda request context.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

## Write your first log

This program writes an order ID and amount as JSON fields. The timestamp in the output is an example; your run uses the current time.

**Input**

```json
{
  "order_id": "ORD-123",
  "amount": 42
}
```

**Code**

```go
package main

import (
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))

	if err := appLog.Info("Processing order", logger.Fields{
		"order_id": "ORD-123",
		"amount":   42,
	}); err != nil {
		panic(err)
	}
}
```

**Output**

```json
{
  "level": "INFO",
  "message": "Processing order",
  "timestamp": "2026-10-01T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "amount": 42,
  "order_id": "ORD-123"
}
```

## Add fields to later messages

Use `AppendKeys` for fields shared by several log calls. Per-call fields apply only to that message.

```go
appLog.AppendKeys(logger.Fields{"order_id": "ORD-123"})
if err := appLog.Info("Order accepted", logger.Fields{"items": 3}); err != nil {
    panic(err)
}
```

The next record contains `"order_id":"ORD-123"` and `"items":3`. Use `RemoveKeys("order_id")` to remove a temporary field.

## Log an error

Pass the business error after the message; handle a returned logging error separately. These standalone snippets panic if Logger itself cannot serialize or write. In a handler, return that logging error to the caller rather than trying to report it through the same failed logger.

```go
paymentErr := errors.New("card declined")
if err := appLog.Error("Payment failed", paymentErr); err != nil {
    panic(err)
}
```

The record has `level: "ERROR"`, `message: "Payment failed"` and `error.message: "card declined"`. It is written to stderr. Logging it does not return the business error from your handler.

In **v1.3.0**, error logs also include a stack by default, as shown in the Preview below. Historical v1.2.0 does not automatically collect error stacks or log handler panics.

## Error stacks and panic logging (Preview)

> This Preview is included in v1.3.0 and main. It is not included in v1.2.0.

Install the published Preview in your application:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

To use development source instead, select `logger@main`. Omitting a version does
not select `main`. Go resolves a branch to a pseudo-version;
use `@<commit-SHA>` instead of `@main` to pin a preview. See
[Go version queries](https://go.dev/ref/mod#version-queries).
If a module proxy still resolves `main` to an older commit, select the intended
main commit SHA explicitly.
The installation above targets v1.3.0, including these Preview defaults.

### Attach the error's source

Use `WithStack` where an error is created or first observed. Ordinary errors do
not need a panic to carry a stack.

```go
func loadOrder() error {
    return logger.WithStack(errors.New("order not found"))
}

// appLog := logger.New(logger.WithServiceName("orders"))
if logErr := appLog.Error("Cannot load order", loadOrder()); logErr != nil {
    return logErr
}
```

**Output** (error excerpt; example path and line numbers):

```json
{"error":{"name":"*errors.errorString","message":"order not found","location":"/app/main.go:11","stack":"main.loadOrder\n\t/app/main.go:11\nmain.main\n\t/app/main.go:16","stack_source":"attached"}}
```

The internal wrapper adds no error name or extra `cause` layer. `%w`,
`errors.Is` and `errors.As` keep working. Repeated wrapping retains the existing
stack along a single error chain; joined branches keep their own origins.

### Get a stack at the log call

With the Preview defaults, an error without an attached stack gets a fallback:

```go
paymentErr := errors.New("card declined")
if logErr := appLog.Error("Payment failed", paymentErr); logErr != nil {
    return logErr
}
```

**Output** (error excerpt):

```json
{"error":{"name":"*errors.errorString","message":"card declined","location":"/app/main.go:19","stack":"main.main\n\t/app/main.go:19","stack_source":"logging"}}
```

This identifies **where the error was logged**, which may differ from where it
occurred. `location` is the first application frame in the selected stack.
Stacks contain at most 64 frames; a longer stack adds `stack_truncated: true`.
Filtered records and records without errors do not collect a stack.

### Record an unhandled handler panic

The outermost Logger wrapper logs one ERROR before cleaning up the request scope,
then rethrows the original panic value. Typed and raw handlers both support it.

```go
handler := func(ctx context.Context, event map[string]any) (string, error) {
    appLog.WithContext(ctx).SetCorrelationID(event["order_id"])
    panic("unexpected payment state")
}
lambda.Start(logger.WrapHandler(appLog, handler))
```

**Input**: `{"order_id":"ORD-123"}`

**Output** (Logger record excerpt):

```json
{"level":"ERROR","message":"Unhandled panic","service":"orders","function_request_id":"request-123","correlation_id":"ORD-123","error":{"name":"string","message":"unexpected payment state","location":"/app/main.go:30","stack":"main.main.func1\n\t/app/main.go:30\n...","stack_source":"panic"}}
```

The `...` above abbreviates caller frames. Nested Logger wrappers do not repeat
this record. The invocation still fails; the Lambda SDK can emit a separate error
record. Ordinary returned handler errors are logged explicitly by your application;
the existing `FlushBufferOnError` opt-in still applies.

Compact JSON escapes stack newlines, so the Logger record occupies one physical
line. Development pretty printing, custom formatters and SDK-generated records
are outside that guarantee. Formatter/replacer hooks still apply. Initialization
panics, other goroutines and runtime fatal errors are outside the wrapper's scope.

### Turn the Preview behavior off

```go
appLog := logger.New(
    logger.WithStackTrace(false),
    logger.WithPanicLogging(false),
)
```

Both options default to `true` in v1.3.0 and on main. Child loggers can override them.
`WithStackTrace(false)` suppresses attached, fallback and panic stack fields;
`WithPanicLogging(false)` suppresses the new automatic panic record. Disable both
to retain the previous logging behavior. These options do not exist in v1.2.0.

Run the [complete Preview example](https://github.com/rambow-cloud/powertools-lambda-go/tree/main/examples/logger-stack-preview)
for source and logging-site stacks; see the [detailed contract](reference/LOGGER.md#stack-and-panic-contract-preview)
for joins, buffering and diagnostic failures.

## Add Lambda request context

Create the logger once. Use the bound logger inside a wrapped handler:

```go
handler := func(ctx context.Context, event map[string]any) (map[string]any, error) {
    requestLog := appLog.WithContext(ctx)
    if err := requestLog.Info("Order received", logger.Fields{"order_id": event["order_id"]}); err != nil {
        return nil, err
    }
    return map[string]any{"status": "accepted"}, nil
}
lambda.Start(logger.WrapHandler(appLog, handler))
```

Logs gain fields such as `function_request_id` and `cold_start`. An input of `{"order_id":"ORD-123"}` returns `{"status":"accepted"}`. See the [complete Lambda](GETTING_STARTED.md) for imports and deployment.

## Choose a log level

| Setting | Example | Effect |
| --- | --- | --- |
| Service | `logger.WithServiceName("orders")` | Adds `service` to every record |
| Level | `logger.WithLevel(logger.DebugLevel)` | Includes debug records |
| Environment | `POWERTOOLS_LOG_LEVEL=INFO` | Sets the default threshold |
| Event logging | `POWERTOOLS_LOGGER_LOG_EVENT=true` | Opts into logging incoming events |
| Sampling | `logger.WithSampleRate(0.1)` | Enables debug logs for a sample of invocations |

## Log the incoming event

For a typed handler, use `WrapRawHandler` when the log should retain fields that are not declared in your event struct:

```go
type Greeting struct { Name string `json:"name"` }
greet := func(ctx context.Context, event Greeting) (string, error) {
    return "Hello, " + event.Name, nil
}
enabled := true
lambda.Start(logger.WrapRawHandler(appLog, greet, logger.HandlerOptions{LogEvent: &enabled}))
```

With input `{"name":"Ada","debug":true}`, the extra log record includes:

```json
{
  "message": "Lambda invocation event",
  "event": {"name": "Ada", "debug": true}
}
```

This is a log excerpt; the typed handler still receives only its declared fields. Event logging is off by default because it can include sensitive payload data.

## Things to know

- Use named fields rather than printf placeholders: `Info("Order %s", id)` does not format the message.
- Use `WrapRawHandler` when event logs must retain JSON fields absent from your typed event. Event logging can expose payload data; it is off by default.
- Request fields belong to the invocation. Join background work before returning; do not retain a bound logger for later requests.
- Advanced Lambda Logging Controls can override the configured threshold.

## Next steps

See the [API and behavior notes](reference/LOGGER.md) for advanced options, and [module installation](MODULES.md) for dependencies.
