// Package secrets retrieves AWS Secrets Manager values with local caching.
//
// [New] wraps an application-owned AWS SDK v2 [Client]. [Provider.Get] supports
// string and binary secrets; [GetOptions] combines SDK selectors with Parameters
// cache and transform options. Pass invocation context to retrieval calls.
//
// # Cache and ownership
//
// Effective version ID and stage selectors isolate entries. Forced refresh and
// nonpositive age follow the shared Parameters policy. [Provider.ClearCache]
// invalidates the local cache. Package-level helpers initialize their default
// provider lazily; explicit providers allow custom clients and endpoints.
//
// Treat retrieved values as sensitive application data and avoid logging them.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/PARAMETERS.md
package secrets
