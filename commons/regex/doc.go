// Package regex implements ECMAScript-style matching and string replacement in Go.
//
// [Compile] prepares a pattern with explicit flags and [Options] matching limits.
// [Regexp.MatchString] checks input, [Regexp.Replace] applies replacement text,
// and [Regexp.Replacer] adapts replacement to a Data Masking field rule.
//
// # Semantics and limits
//
// This optional module implements compatibility behavior beyond Go's regexp
// package, including Unicode handling and JavaScript replacement tokens. The v flag
// is unsupported. Configure a timeout or backtracking limit for complex patterns.
// Global and sticky replacement can update lastIndex; [Regexp] serializes that
// state and is safe for concurrent use, but must not be copied.
//
// Importing core Commons or Data Masking does not load this regex engine.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/REGEX.md
package regex
