// Package http routes native AWS Lambda HTTP events with isolated request state.
//
// [New] constructs a [Router]. Register method/path handlers with [Router.Get],
// [Router.Post] or [Router.Handle], and compose middleware with [Router.Use].
// [Router.Resolve] accepts API Gateway, Function URL and ALB event envelopes.
// Handlers receive [RequestContext] with a native net/http request and route data.
//
// # Response and lifecycle
//
// Handlers can return application values, explicit responses or native HTTP
// responses; conversion preserves status, headers, cookies and binary encoding.
// The streaming API supports the Lambda response-streaming lifecycle. See the
// guide before mixing buffered responses, owned bodies and streaming operations.
//
// Request stores isolate invocation state; the router's shared store is explicitly
// application-wide. Register routes during initialization. Optional http/metrics
// and http/tracer modules add EMF and OpenTelemetry without bringing those
// dependencies into this core router.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/HTTP.md
package http
