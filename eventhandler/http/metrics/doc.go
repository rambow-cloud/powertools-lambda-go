// Package metrics adds CloudWatch EMF middleware to the Lambda HTTP router.
//
// [New] adapts a core Metrics instance to HTTP middleware. Register it with the
// router's Use method during application setup. Route execution records
// latency, fault and error metrics using request-scoped metric state.
// [Options.CaptureRequestCount] opts in to a request metric with unit Count and
// value 1 per middleware execution. The default follows TypeScript v2.35.0.
//
// # Publication lifecycle
//
// Each request owns a scope that publishes when middleware execution finishes,
// before any streaming body transfer. The parent scope remains unchanged.
// Publication failures propagate while retaining business errors. The application
// supplies namespace, service, output and metric policy.
//
// Aggregate request with Sum for published execution counts, including errors
// and panics. It does not measure unique client requests or completed streams;
// retries and EMF delivery can repeat samples. Reserve the request metric and
// route dimension for this middleware when enabling it. Additional metrics incur
// CloudWatch costs.
//
// This separate module keeps the core HTTP router free of observability dependencies.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/HTTP_OBSERVABILITY.md
package metrics
