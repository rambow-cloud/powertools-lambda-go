// Package tracer adds OpenTelemetry route spans to the Lambda HTTP router.
//
// [New] adapts a core Tracer instance to HTTP middleware; [Options] controls
// response capture. Register it before compression middleware to observe the
// headers left by compression. Handler code receives the route span context.
//
// # Span and provider ownership
//
// The middleware ends route spans but leaves flushing and provider shutdown to
// the Lambda wrapper or application. It skips HTTP streaming; compose the core
// Tracer WrapHandler inside Streamify for the complete streaming invocation.
// Use an OpenTelemetry backend and a collector's awsxray exporter for X-Ray.
//
// This separate module keeps tracing dependencies out of the core HTTP router.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/HTTP_OBSERVABILITY.md
package tracer
