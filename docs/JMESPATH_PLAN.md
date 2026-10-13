# JMESPath implementation checklist

Reference: TypeScript v2.35.0. Keep comprehensive parity separate from the initial implementation. See [JMESPATH.md](JMESPATH.md) for the public contract and differences.

- [x] J-01: Implement standard query evaluation, compiled expressions, typed errors, and a bounded shared syntax cache.
- [x] J-02: Implement typed/variadic custom functions, expression references, and isolated function definitions.
- [x] J-03: Reuse Commons for Powertools Base64 and snapshots; implement JSON and gzip functions and all thirteen exact envelopes.
- [x] J-04: Generate 78 actual TypeScript fixtures covering standard functions, envelopes, Unicode, and error boundaries; explicitly preserve upstream undefined results.
- [x] J-05: Verify typed-event/result isolation, literal immutability, cache clearing, and 100 concurrent searches.
- [x] J-06: Integrate optional compiled expressions into Logger without adding a query dependency to Logger, and verify decoded projections and correlation in local Lambda composition.
- [x] J-07: Pass 14 independent module tests/vet, 11 consumer builds, both Lambda architecture builds, and 114/114 Docker assertions with CGO disabled.
- [ ] J-08: Complete exhaustive grammar/function/serialization/invalid-input parity, malformed UTF-8 edge cases, and performance/allocation budgets.

## Finite boundary audit #187

The local audit executes 51 preserved TypeScript v2.35.0 observations and reuses
the existing standard/envelope, UTF-8, compiled-expression ownership and concurrent
search tests. It covers seven invalid grammar inputs, four function signature
failures, fifteen numeric strings in both conversion and serialization, and ten
native serialization observations. It does not complete the exhaustive J-08 gate.

| Boundary | Result |
| --- | --- |
| Empty/whitespace, radix, BOM, invalid, overflow and underflow numeric strings | Reproduced conversion defects fixed through Commons `ParseNumber`; NaN becomes null and infinity remains a numeric intermediate. |
| HTML, line separators, Unicode, negative zero and intermediate non-finite serialization | Reproduced legacy JSON encoding defects fixed with `encoding/json/v2`; nested non-finite values stringify to null. Native input validation still rejects non-finite numbers. |
| Object key enumeration | Intentional Go difference: deterministic lexical map order; both original insertion/numeric-key observations remain in the fixture. |
| Grammar/function errors | Typed Go errors retain expression and cause; empty source uses the existing `EmptyExpression` kind. Community grammar extensions remain rejected. |
| Null multi-select input | Reproduced interpreter defect separately tracked in [#225](https://github.com/rambow-cloud/powertools-lambda-go/issues/225); the original null observation and current limitation are explicit, not counted as parity. |
| Native invalid values and custom failures | Strict duplicate-key/UTF-8/non-finite/channel rejection and once-only callback error identity verified. |

The generator is `tools/reference/generate-jmespath-boundaries.mjs`. Packaged
checks and the standalone consumer require CGO disabled and GOWORK off. Resource
budgets (#200), exhaustive grammar and the separately linked interpreter defect
remain open; no live AWS acceptance is claimed.
