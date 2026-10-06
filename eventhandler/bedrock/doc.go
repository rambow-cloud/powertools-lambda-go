// Package bedrock handles AWS Bedrock Agent function-based action-group invocations.
//
// [New] constructs a [Resolver]. [Resolver.Tool] registers a named [ToolHandler]
// using [Configuration]. [Resolver.Resolve] selects the tool and builds the
// action-group response envelope. Tool handlers receive invocation context, ordered
// [Parameters] and the complete [Event].
//
// # Integration boundary
//
// This package implements function-based action groups. The application configures
// the Bedrock Agent, action group, function definitions and Lambda permissions.
// It does not invoke a model or implement an OpenAPI-based action-group router.
//
// Register tools during initialization and make callbacks safe for concurrent
// invocations. Review documented response, parameter and named-error behavior
// before returning application values or translating failures.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/BEDROCK.md
package bedrock
