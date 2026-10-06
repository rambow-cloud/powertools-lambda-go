// Package tracer instruments AWS Lambda handlers using OpenTelemetry.
//
// [New] creates a [Tracer]. [WrapHandler] traces a typed Lambda invocation,
// [Capture] traces an operation, and [Tracer.StartSpan] supports manual spans.
// Always pass the returned span context to downstream operations and end the span.
//
// # Providers and export
//
// [NewOTelBackend] accepts an application-owned OpenTelemetry provider without
// installing globals or taking ownership of it. The default backend configures
// OTLP export; deliver traces to AWS X-Ray through a collector's awsxray exporter.
// [Tracer.HTTPClient] and [Tracer.InstrumentAWS] instrument outbound calls.
// The application owns shutdown for a supplied provider; review flush behavior
// when composing Lambda wrappers.
//
// [ExtractLambdaHTTPContext] is an opt-in incoming propagation callback. Choose
// whether request headers are trusted and whether responses or errors may contain
// sensitive data before enabling capture. The separate tracer/xray SDK adapter is
// deprecated and frozen; use this OpenTelemetry integration for new applications.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/TRACER.md
package tracer
