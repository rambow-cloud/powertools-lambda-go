// Package metrics emits CloudWatch Embedded Metric Format documents for AWS Lambda.
//
// [New] configures namespace, service, dimensions and an io.Writer, which defaults
// to standard output. [Metrics.AddMetric] records values with a [Unit] and
// [Resolution]; [Metrics.Flush] publishes the accumulated EMF document.
// CloudWatch ingests EMF from Lambda logs, so publishing does not call its SDK.
//
// # Invocation lifecycle
//
// [WrapHandler] manages metric publication around a typed handler. Use
// [Metrics.WithContext] inside that handler for isolated invocation state.
// Cold-start capture, empty-metric policy and error callbacks are configurable.
// Keep dimensions bounded: values such as request IDs create expensive cardinality.
//
// Metrics uses io.Writer rather than depending on Logger or Tracer. Applications
// own custom writers and configuration callbacks.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/METRICS.md
package metrics
