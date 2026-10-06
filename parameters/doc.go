// Package parameters shares caching, transforms and errors for configuration providers.
//
// The ssm, secrets, dynamodb, appconfig and appconfigagent subpackages retrieve
// configuration from their respective services. Construct service providers with
// application-owned clients and reuse them across Lambda invocations.
//
// # Cache and transform policy
//
// [Options] controls age, refresh, missing-value and transform behavior. Use [Age]
// for an explicit lifetime; nonpositive lifetimes bypass lookup and storage, while
// ForceFetch bypasses lookup. Service adapters isolate entries by effective request
// options. [Cache] also supports application-owned retrieval callbacks.
//
// [TransformValue] applies JSON or Base64 decoding. Retrieval and transform errors
// preserve underlying failures for errors.Is and errors.As. Provider ClearCache
// methods invalidate local entries; they do not change the remote configuration.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/PARAMETERS.md
package parameters
