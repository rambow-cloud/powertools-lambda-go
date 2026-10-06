// Package metadata retrieves AWS Lambda execution-environment metadata on demand.
//
// [GetMetadata] uses the default client. [New] constructs an application-owned
// [Client] with an explicit endpoint, token and HTTP client for custom environments.
// Importing this package does not fetch metadata.
//
// # Caching and errors
//
// Successful responses are cached and returned as independent snapshots.
// [Client.ClearCache] invalidates the client cache; [ClearMetadataCache] invalidates
// the default cache. Pass invocation context and [Options] to bound retrieval.
// [Error] preserves HTTP status information and underlying errors.
//
// Metadata access remains optional: ordinary logging, tracing and configuration
// retrieval do not require this module or a metadata request.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/METADATA.md
package metadata
