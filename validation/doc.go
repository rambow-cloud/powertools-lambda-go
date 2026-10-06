// Package validation checks Lambda inputs and outputs against JSON Schema.
//
// [Compile] prepares a reusable [Schema]; [Schema.Validate] checks a payload
// against it. [Validate] combines compilation and validation for one-off calls.
// [Options] configures extraction, compilation and error behavior.
// [WrapHandler] validates typed handler inputs and optional outputs.
//
// # Dialect and diagnostics
//
// The default pure-Go backend implements the documented Draft 7 compatibility
// scope, with controlled keyword, format, reference and regular-expression rules.
// Structured issues retain schema and payload locations. It is not a claim of
// complete compatibility with every JSON Schema engine or extension.
//
// Compile stable schemas during initialization and reuse them during invocations.
// Use the separate Parser module for composable typed Go schemas; Validation's
// engine dependencies do not become dependencies of Parser or core Commons.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/JSON_SCHEMA_VALIDATION.md
package validation
