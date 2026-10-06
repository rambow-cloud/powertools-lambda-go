// Package commons provides dependency-free shared utilities for AWS Lambda.
//
// It includes environment configuration, JSON-shaped cloning and merging, Base64
// decoding, DynamoDB value conversion, bounded LRU caches, and invocation metadata.
// Import this package at github.com/rambow-cloud/powertools-lambda-go/commons;
// the repository root is its module, not an importable package.
//
// # Reuse and compatibility
//
// [NewLRUCache] creates a concurrency-safe bounded cache. [NewUtility] provides
// utility cold-start state, while Lambda wrappers share invocation identity through
// context. The optional commons/awssdk, commons/dynamodb, commons/metadata and
// commons/regex modules supply integrations without adding dependencies here.
//
// Conversion helpers preserve the documented Powertools compatibility contracts,
// which can differ from Go's native type and numeric conventions. Consult the
// guide before replacing standard-library conversions with these helpers.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/COMMONS.md
package commons
