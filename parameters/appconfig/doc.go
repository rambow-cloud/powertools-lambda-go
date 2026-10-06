// Package appconfig retrieves configuration through the AWS AppConfig Data API.
//
// [New] wraps an application-owned SDK [Client] with application and environment
// names in [Config]. [Provider.Get] uses a configuration profile name and
// [GetOptions] for session selectors, cache age and transforms.
//
// # Polling lifecycle
//
// The provider manages configuration session tokens and serializes polling for
// each session. An unchanged service response retains the previous configuration.
// Reuse the provider across Lambda invocations so token and cache state survive.
// [Provider.ClearCache] invalidates local cached values.
//
// For the local AppConfig Agent or Lambda extension HTTP interface, use the
// sibling appconfigagent package instead of this direct SDK adapter.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/PARAMETERS.md
package appconfig
