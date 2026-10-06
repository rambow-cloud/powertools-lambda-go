// Package parser validates and transforms Lambda payloads with typed schemas.
//
// [Parse] returns a validated value or [ParseError]. [SafeParse] retains the
// original input and structured [Issue] values when validation fails.
// [WrapHandler] and [WrapSafeHandler] adapt schemas to typed Lambda handlers.
//
// # Schema composition
//
// Build schemas with [Object], [Array], primitive constructors, [Transform] and
// [Refine]. [Typed] maps validated JSON-shaped values into application types.
// The schemas subpackage supplies service event contracts; envelopes validates
// the outer event and extracts a payload for further parsing.
//
// Parser has no JSON Schema engine dependency. Use the separate Validation module
// for JSON Schema documents. Review the guide's field, number and error boundaries
// before relying on compatibility with another Powertools implementation.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/PARSER.md
package parser
