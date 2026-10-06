// Package cache persists idempotency records in Redis or Valkey.
//
// [New] wraps an application-owned [Client] with attribute and clock [Options].
// The resulting [Store] implements core Idempotency persistence and can be passed
// to its manager. The application owns connection setup, TLS, credentials and
// client shutdown.
//
// # Atomic acquisition
//
// The adapter uses atomic claim and compare-and-swap behavior to reject live
// records and recover expired claims. A missing or zero execution deadline does
// not make an otherwise unexpired claim recoverable. [ConsistencyError] identifies
// storage failures that prevent the required acquisition guarantees.
//
// Use a shared service for coordination across Lambda environments. This module
// is optional and does not add Redis dependencies to the core Idempotency module.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/IDEMPOTENCY_CACHE.md
package cache
