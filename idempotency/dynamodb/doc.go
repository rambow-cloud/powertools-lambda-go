// Package dynamodb persists idempotency records in AWS DynamoDB.
//
// [New] constructs a [Store] from an application-owned AWS SDK v2 [Client] and
// [Options] describing the table and persistence attributes. Supply the store to
// the core Idempotency manager, which owns keys, expiry and business execution.
//
// # Storage contract
//
// Conditional writes acquire claims atomically; reads return consistent record
// snapshots. Completed responses are stored as JSON data. The application
// provisions the table, TTL policy and IAM permissions.
//
// Store methods preserve cancellation and service error chains. The documented
// record format and expiry conventions support the scoped compatibility contract;
// review the guide before sharing a table with other implementations.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/IDEMPOTENCY.md
package dynamodb
