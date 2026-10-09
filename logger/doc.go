// Package logger writes structured JSON logs for AWS Lambda handlers.
//
// [New] creates a reusable [Logger] with service, level, output and formatting
// options. [Logger.Info] and the other level methods accept additional fields.
// Use [WithPersistentKeys] for stable service metadata rather than request data.
//
// # Invocation lifecycle
//
// [WrapHandler] establishes invocation state. Inside the handler, call
// [Logger.WithContext] with the supplied context to obtain request-scoped fields
// and correlation information. This prevents keys from leaking between invocations.
// Buffering, sampling and event logging are configurable; review their data and
// flush policies before enabling them for production payloads.
// [WrapRawHandler] retains incoming JSON members in event logs before decoding
// a typed business input. [WrapHandler] logs the input value it receives.
//
// The package composes with OpenTelemetry tracing and optional JMESPath extraction.
// Applications own custom writers, formatters, clocks and error callbacks.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/LOGGER.md
package logger
