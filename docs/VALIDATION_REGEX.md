# Validation regular expressions

Validation uses a pure-Go ECMAScript Unicode regex adapter backed by [regexp2/v2 v2.8.0](https://github.com/dlclark/regexp2/tree/v2.8.0), pinned to commit `9d0d2ffe88a8b90012f7979ec85424e46d5ef48f`. It is used for both `pattern` and `patternProperties`. The implementation now lives in the optional commons/regex module and is also used by Data Masking replacements; see [REGEX.md](REGEX.md). CGO remains disabled, and deployed Lambda binaries require neither Node.js nor a JavaScript runtime.

## Verified behavior

The 6,085 cases in `validation/testdata/regex-v2.35.0.json` execute the actual Powertools v2.35.0 Validation utility using its default AJV behavior on Node v22.21.1. They preserve ordered diagnostics, compilation errors and returned values. All 1,683 supported Unicode property aliases are exercised, with non-surrogate member examples where available and empty-string rejection for every alias. Together with the original 502 Validation cases, the module has 6,587 reference scenarios.

The corpus covers positive/negative lookahead and lookbehind, numeric/named/forward/unmatched backreferences, capture numbering, lazy quantifiers, astral code points, paired UTF-16 escapes, JavaScript line terminators and whitespace, exact property names/aliases, complemented Unicode properties, and property-name matching. Invalid Unicode escapes, range endpoints, quantified assertions and non-JavaScript group forms are rejected. Error messages retain the original pattern. Schema paths escape property components like AJV, while instance paths use JSON Pointer escaping.

The adapter is lexical: it handles Unicode escapes, property classes and syntax differences, while the engine remains responsible for matching, groups and quantifiers. Character-class state prevents text inside a class from being interpreted as a group or wildcard. Original schema locations survive the existing all-errors adaptation.

## Unicode data and reproduction

`commons/regex/unicode_properties.json` contains 433 canonical property tables and 1,683 exact aliases for Unicode 16.0. The generator obtains aliases from the official [PropertyAliases.txt](https://www.unicode.org/Public/16.0.0/ucd/PropertyAliases.txt) and [PropertyValueAliases.txt](https://www.unicode.org/Public/16.0.0/ucd/PropertyValueAliases.txt), then asks the pinned Node runtime which names it supports and which code points match. It does not infer support from the broader, loosely matched property names accepted by the underlying engine.

The generated file records the Node/Unicode versions, source URLs and SHA-256 values. Source alias files are retained under `tools/reference/unicode/`; the [Unicode license](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/validation/UNICODE-LICENSE.txt) accompanies the embedded runtime tables in the independent module archive. The generated JSON is 380,994 bytes. Unicode data is initialized once and read concurrently; each compiled regex owns its match configuration.

From `tools/reference`, use the existing Node v22.21.1 toolchain:

```text
node generate-validation-unicode.mjs
node generate-validation-regex.mjs
```

The Unicode generator requires Node's Unicode version to be 16.0. Cached source alias files avoid network requests on subsequent generation. Normal Go tests read the saved reference corpus. Runtime validation uses only the embedded tables and pure-Go matcher.

## Operational limits

The default compiler accepts `CompileOptions.Regex`:

```go
validation.Options{
    CompileOptions: validation.CompileOptions{
        Regex: validation.RegexOptions{
            MatchTimeout:             100 * time.Millisecond,
            MaxBacktrackingStackSize: 100_000,
        },
    },
}
```

Zero timeout preserves the engine's unlimited-time default. A positive timeout enables the engine's approximate timeout checks; it is not an exact deadline guarantee. Zero stack configuration uses the pinned engine's 100,000-entry default; a positive value sets a bound and a negative value removes it. These are operational limits, not changes to the accepted pattern syntax. They are Go configuration options rather than AJV defaults.

A matching failure caused by timeout or stack exhaustion returns `RegexError`, which unwraps the underlying cause. It is not a `SchemaValidationError` in standalone calls. A handler wrapper adds its inbound/outbound stage and preserves that operational cause. The schema engine's boolean-only matcher interface is bridged using a private sentinel; unrelated application panics, including a public `RegexError` panic, retain their identity. Unit tests exercise these boundaries and 32 concurrent calls through the same compiled schema.

## Remaining gates

These cases establish a substantial compatibility milestone, not exhaustive ECMAScript conformance. Unpaired UTF-16 surrogate payloads, Unicode names/escapes in capture identifiers, arbitrary assertion/backtracking combinations, very large quantifiers, native malformed strings and all regex error-cause messages still require an expanded corpus. Go JSON snapshots cannot retain an unpaired JavaScript UTF-16 code unit as an ordinary Unicode scalar.

Context cancellation is still checked around synchronous schema traversal rather than interrupting an in-progress regex. Configured regex timeouts provide a separate approximate bound. Compilation/validation throughput, allocation and resource budgets remain open; the engine's functional concurrency tests do not substitute for the prohibited CGO-dependent race detector.

The isolated Windows consumer harness grew from 5,048,832 to 5,557,248 bytes with this change, an increase of 508,416 bytes. The current size is recorded in MODULE_ACCEPTANCE.json. This blank-import consumer is a dependency/build check, not a representative Lambda workload or a latency/allocation benchmark. Commons and Parser remain free of third-party module dependencies.
