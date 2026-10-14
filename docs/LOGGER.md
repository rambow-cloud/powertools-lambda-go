---
title: Structured logging for Go Lambda
description: "Write structured JSON logs in Go Lambda with context, correlation IDs, configurable levels and buffering using Powertools Logger."
---

# Logger

Write JSON logs with searchable fields, service names and Lambda request context.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.2.0
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
	"log"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))

	if err := appLog.Info("Processing order", logger.Fields{
		"order_id": "ORD-123",
		"amount":   42,
	}); err != nil {
		log.Printf("Logging failed: %v", err)
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
    log.Print(err)
}
```

The next record contains `"order_id":"ORD-123"` and `"items":3`. Use `RemoveKeys("order_id")` to remove a temporary field.

## Log an error

Pass the business error after the message; handle a returned logging error separately.

```go
paymentErr := errors.New("card declined")
if err := appLog.Error("Payment failed", paymentErr); err != nil {
    log.Print(err)
}
```

The record has `level: "ERROR"`, `message: "Payment failed"` and `error.message: "card declined"`. It is written to stderr. Logging it does not return the business error from your handler.

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
