// Package metrics adds CloudWatch EMF middleware to the Lambda HTTP router.
//
// [New] adapts a core Metrics instance to HTTP middleware. Register it with the
// router's Use method during application setup. Route execution records request,
// latency, fault and error metrics using request-scoped metric state.
//
// # Publication lifecycle
//
// Each request owns a scope that publishes when middleware execution finishes,
// before any streaming body transfer. The parent scope remains unchanged.
// Publication failures propagate while retaining business errors. The application
// supplies namespace, service, output and metric policy.
//
// This separate module keeps the core HTTP router free of observability dependencies.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/HTTP_OBSERVABILITY.md
package metrics
