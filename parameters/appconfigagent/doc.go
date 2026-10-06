// Package appconfigagent reads configuration from the local AWS AppConfig Agent.
//
// [GetConfig] calls the Agent or Lambda extension HTTP endpoint. [Options] supplies
// application, environment, endpoint, HTTP client and transform settings. Pass the
// configuration profile name and invocation context on each call.
//
// # Retrieval boundary
//
// The Agent owns caching and polling; this adapter does not add a second provider
// cache. The application deploys and configures the Agent or extension. Local HTTP
// failures and transform errors are returned to the caller.
//
// Use the sibling appconfig package when calling AppConfig Data directly with an
// application-owned AWS SDK client.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/PARAMETERS.md
package appconfigagent
