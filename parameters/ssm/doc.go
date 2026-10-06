// Package ssm retrieves and writes AWS Systems Manager Parameter Store values.
//
// [New] wraps an application-owned AWS SDK v2 [Client]. [Provider.Get] retrieves
// one parameter, [Provider.GetMultiple] traverses a path, and [Provider.Set] writes
// a parameter. [GetOptions] and [MultipleOptions] select SDK request options and
// the shared Parameters cache/transform policy.
//
// # Defaults and isolation
//
// Package-level helpers lazily initialize a default provider; use an explicit
// provider when controlling clients, endpoints or credentials. Effective decrypt
// and path request options isolate cached values, and matching single/batch reads
// can reuse entries. [Provider.ClearCache] invalidates local cached configuration.
// SDK errors retain their error chain through the Parameters error types.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/PARAMETERS.md
package ssm
