// Package dynamodb retrieves configuration values from an AWS DynamoDB table.
//
// [New] wraps an application-owned AWS SDK v2 [Client] using [Config] for the
// table and attribute names. [Provider.Get] retrieves one value and
// [Provider.GetMultiple] queries a configuration path, including paginated results.
// Their option types combine SDK requests with Parameters cache and transforms.
//
// # Table and cache policy
//
// The application provisions the table, keys and permissions. This adapter does
// not create tables. Successful retrievals are cached according to Parameters
// policy; [Provider.ClearCache] invalidates local entries. Missing-value and
// transform behavior are explicit options, and errors retain SDK error chains.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/PARAMETERS.md
package dynamodb
