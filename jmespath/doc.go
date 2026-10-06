// Package jmespath queries JSON-shaped values and typed Lambda events.
//
// [Search] evaluates a query once; [Compile] prepares an immutable [Expression]
// for reuse across invocations. Typed inputs are normalized using JSON field tags,
// and JSON numbers are represented as float64.
//
// # Functions and failures
//
// Standard JMESPath functions are enabled by default. Opt in to JSON, Base64 and
// gzip decoding with [WithPowertoolsFunctions]. Custom functions must be safe for
// concurrent calls and must not mutate or retain input values.
//
// [Error] identifies syntax, function and value failures without requiring callers
// to parse engine messages. [PurgeCache] removes cached syntax trees without
// invalidating expressions that have already been compiled.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/JMESPATH.md
package jmespath
