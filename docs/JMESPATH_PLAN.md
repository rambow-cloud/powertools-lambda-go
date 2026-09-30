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
