// Package appsyncgraphql routes AWS Lambda AppSync GraphQL invocations.
//
// [New] constructs a [Resolver]. Register query, mutation or type/field handlers
// through [Router.OnQuery], [Router.OnMutation] and [Router.OnResolver], then call
// [Resolver.Resolve] with decoded single or batch events.
//
// # Batch and errors
//
// [BatchOptions] defaults to aggregate handling; individual mode processes events
// sequentially and can control whether failures propagate. Handler arguments and
// event data are passed through without copying. Configure exception handlers
// with [Router.OnException] and use the scalar helpers for AppSync-specific values.
//
// The application owns the GraphQL schema and Lambda integration. This resolver
// does not execute arbitrary GraphQL documents or provision an AppSync API.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/APPSYNC_GRAPHQL.md
package appsyncgraphql
