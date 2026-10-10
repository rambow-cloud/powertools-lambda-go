---
description: "Use shared ECMAScript regular expression matching and replacement in Powertools for Go with documented Unicode behavior."
---

# Shared ECMAScript regular expressions

The optional `commons/regex` module owns the pure-Go regex engine, Unicode 16.0 property tables and pattern adaptation previously embedded in Validation. Validation delegates both Unicode payload matching and legacy strict property-overlap matching to it. Data Masking composes it through `Rule.Replace`; neither root Commons nor ordinary Data Masking depends on the engine.

```go
import (
    "github.com/rambow-cloud/powertools-lambda-go/commons/regex"
    "github.com/rambow-cloud/powertools-lambda-go/datamasking"
)

expression, err := regex.Compile(`(?<account>\d{4})\d+`, "g", regex.Options{})
if err != nil {
    return err
}
masked, err := datamasking.New(datamasking.Config{}).Erase(ctx, payload,
    datamasking.EraseOptions{
        Fields: []string{"account"},
        Rule: datamasking.Rule{Replace: expression.Replacer("$<account>****")},
    })
```

## Matching and replacement

`Compile(pattern, flags, Options)` accepts `d`, `g`, `i`, `m`, `s`, `u` and `y`. The indices flag has no observable effect on string replacement. Duplicate and unknown flags are errors. `MatchString` performs a stateless search from the beginning, requiring a match at zero for sticky patterns; it never changes `lastIndex`.

`Replace` and `Replacer` implement JavaScript replacement tokens: `$$`, `$&`, prefix/suffix tokens, numbered captures including two-digit fallback, and named captures. Captures retain the original input text even when matching uses legacy case canonicalization. No caller callback runs while the Regexp lock is held.

Global replacement starts at zero and resets `lastIndex` after the operation. Sticky non-global replacement starts at the stored UTF-16 offset, updates it after success and resets it after failure. Non-global, non-sticky replacement leaves it unchanged. Zero-length global matches advance a code unit without `u`, or a code point with `u`. A Unicode start offset inside a surrogate pair rewinds to that code point, matching the reference.

`LastIndex` and `SetLastIndex` are synchronized. An individual stateful replacement is atomic with respect to other replacements on the same expression. A separate SetLastIndex/Replace pair is not one atomic operation; use separate instances for independently controlled state. Replacer callbacks deliberately share their expression across fields, matching upstream RegExp reuse.

Legacy matching uses UTF-16 code units. Legacy `i` canonicalization is generated from Node's Unicode 16.0 uppercase rules, including the restriction on non-ASCII characters becoming ASCII. Character classes are canonicalized before matching without changing captures. Unicode property aliases and ranges reuse the pinned Validation tables. Word boundaries and multiline anchors explicitly account for JavaScript's word characters and four line terminators.

## Operational and native boundaries

Options preserve the existing Validation timeout and backtracking-stack limits. Resource failures remain errors; Validation retains its public RegexError wrapping. Go strings preserve lone surrogates as WTF-8 bytes at this module's native boundary. `encoding/json/v2` rejects invalid UTF-8 and lone escaped surrogates, so JSON cannot carry those values as JavaScript UTF-16 strings. Input strings with arbitrary invalid UTF-8 are not a JavaScript string representation.

This is a tested compatibility implementation, not a complete ECMAScript interpreter. Unicode sets (`v`), remaining advanced syntax/case-fold/property/backreference edges, exact SyntaxError wording, JavaScript object coercion and replacement functions remain unverified or unsupported. SetLastIndex accepts a native int rather than arbitrary JavaScript values. Return values do not include match indices/capture arrays. These boundaries remain open under MASK-REGEX and the broader Validation parity gate.

## Evidence

`tools/reference/generate-regex.mjs` records 7,671 exact Node v22.21.1 replacement cases, 19 invalid patterns and 30 actual Powertools TypeScript v2.35.0 Data Masking scenarios. Expected strings are stored as UTF-16 units where lone surrogates matter. Native tests cover 64 concurrent replacements/searches and operational failure identity. All 23,071 existing Validation cases pass after extraction. Combined packaged acceptance covers 30 modules/27 consumers; Linux amd64/arm64 builds, 856/856 RIE assertions, 95/95 streaming and 14/14 Batch checks pass. See LOCAL_VALIDATION.md and MODULE_ACCEPTANCE_REGEX.json for the initial compile failure and six-module continuation. These results do not establish full parity or performance budgets.
