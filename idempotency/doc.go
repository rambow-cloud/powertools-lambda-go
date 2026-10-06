// Package idempotency prevents repeated execution using atomic persistence.
//
// [New] constructs a [Manager] with a [Store] and [Options] for event keys,
// payload validation, expiry and optional local response caching. Typed handler
// wrapping and operation execution acquire a record before calling business code
// and replay a persisted response for completed duplicate events.
//
// # Persistence guarantees
//
// Stores must atomically reject live claims and provide consistent snapshots.
// The dynamodb subpackage implements DynamoDB persistence; the independent
// idempotency/cache module supplies Redis/Valkey persistence. Provision shared
// storage for coordination across Lambda environments; the optional local cache
// alone is not a distributed lock.
//
// [Record] stores expiry in seconds and execution lease expiry in milliseconds.
// Concurrent live operations produce [AlreadyInProgressError]. Payload mismatch
// and storage failures remain distinguishable through typed errors.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/IDEMPOTENCY.md
package idempotency
