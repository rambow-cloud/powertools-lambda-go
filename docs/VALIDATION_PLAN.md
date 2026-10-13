# Validation implementation checklist

Post-v1 dialect/extension scope, thirteen executed pinned reference observations
and proposed acceptance are in [the extension design](VALIDATION_EXTENSION_DESIGN.md)
for issue #118. This completes planning only; the implementation gates below remain.

The first implementation slice, [#168](https://github.com/rambow-cloud/powertools-lambda-go/issues/168),
adds `Draft202012Compiler` on `main`. Its native diagnostics, registered references
and measured injected-engine differences are in the
[optional compiler contract](JSON_SCHEMA_VALIDATION.md#optional-draft-2020-12-compiler).
Other dialects and transformations remain separately tracked work.

[#169](https://github.com/rambow-cloud/powertools-lambda-go/issues/169) adds the
opt-in `Draft201909Compiler`, with independently captured tuple/recursive-reference
behavior and native differences. It reuses proven registration/diagnostics without
changing either existing default or 2020-12 contracts.

Reference: Powertools TypeScript v2.35.0, installed AJV v8.20.0. The default Go engine is jsonschema/v6 v6.0.3. All documentation and code comments remain English. Completion of a scoped item does not establish full AJV parity.

- [x] V-01: Implement manual validation, reusable compiled schemas, an injected compiler/validator contract, and typed compilation/validation errors. Verify schema/options snapshots, 32 concurrent callers, cancellation, operational error identity and invalid JSON handling.
- [x] V-02: Implement Draft 7 adaptation for nullable, reference siblings, registered string/numeric formats, standard JMESPath extraction, optional extended query functions and explicitly registered references. All 502 exact TypeScript result/error cases passed, including four invalid-schema regressions that prevent adaptation from hiding an invalid original schema.
- [x] V-03: Implement typed inbound/outbound wrappers with shared invocation context, JSON input snapshots, output envelope bypass, staged errors, handler result/error/panic preservation and the reference's literal-false wrapper behavior. Verify native typed event isolation and handler rejection paths.
- [x] V-04: Verify the final 502 reference cases, all 19 packaged module tests/vet/tidy checks, sixteen standalone consumers, the native Lambda example, both CGO-disabled Linux builds and 316/316 local Docker assertions. All five invocations completed and containers/network were cleaned. The same artifacts passed 14/14 Batch checks. Evidence: MODULE_ACCEPTANCE.json, LOCAL_ACCEPTANCE.json and BATCH_ACCEPTANCE.json (2026-09-16, Asia/Shanghai). Docker executed amd64; arm64 was cross-compiled only. No AWS deployment occurred.
- [ ] V-05: Complete the public export/options/default/error audit, strict-schema rules, custom format/plugin forms, injected-engine options and middleware/decorator behavior mapping.
- [x] V-05d: Control conditional compilation without deleting reference targets, preserve nested dialect annotations, globally index nested resource IDs and expose ordered external schema registration. Verify 1,104 new exact reference cases (23,071 Validation total), snapshots, 32 concurrent callers and exactly-once callbacks. All 19 modules/16 consumers, both CGO-disabled Linux builds, 442/442 Docker assertions and 14/14 Batch checks passed (2026-09-16, Asia/Shanghai). Temporary resources were cleaned; no AWS resources were used. Docker ran amd64, while arm64 was cross-compiled only. Full identity/setup/error compatibility remains open; see [VALIDATION_GRAPHS.md](VALIDATION_GRAPHS.md).
- [x] V-05c: Check reachable keyword types, preserve inactive primitive fragments and annotations, compile nontrivial constant-condition branches, skip inactive conditions and reuse required-entry conversion for property dependencies. Verify 5,764 new reference cases (21,967 Validation total), callback counts, all 19 modules/16 consumers, both CGO-disabled Linux builds, 424/424 Docker assertions and 14/14 Batch checks (2026-09-16, Asia/Shanghai). Temporary resources were cleaned; no AWS resources were used. Docker ran amd64, while arm64 was cross-compiled only. Ignored-reference resolution, dialects and complete setup remain open; see [VALIDATION_SHAPES.md](VALIDATION_SHAPES.md).
- [x] V-05b: Separate original-document structural validation from reachable compilation checks, including unused external documents, definition pointers/aliases/IDs and always-valid pattern optimization. Verify 658 additional reference cases (12,770 Validation total), original-structure and callback regressions, all 19 modules/16 consumers, both CGO-disabled Linux builds, 382/382 Docker assertions and 14/14 Batch artifact checks (2026-09-16, Asia/Shanghai). Temporary resources were cleaned and no AWS resources were used. External registration error identity and the documented unvalidated `$defs` keyword forms remain open; see [VALIDATION_STRICT.md](VALIDATION_STRICT.md).
- [x] V-05a: Use legacy UTF-16 regex matching for strict property/pattern overlap while retaining Unicode payload matching, always-valid/absent-property skips and operational resource-error identity. Verify 3,047 new cases (12,112 Validation total), all 19 modules/16 consumers, both CGO-disabled Linux builds, 364/364 Docker assertions and 14/14 Batch artifact checks (2026-09-16, Asia/Shanghai), with temporary resources cleaned and no AWS deployment. The 24 reference-only unused-definition observations are not counted as passing Go cases. See [VALIDATION_STRICT.md](VALIDATION_STRICT.md); complete strict setup and regex parity remain open.
- [ ] V-06: Complete nested all-errors ordering and every keyword's message/params/schema-path mapping, including conditionals, dependencies, property names, tuples and reference variants.
- [x] V-06a: Implement scoped conditional/dependency/property-name/tuple diagnostics and nested declaration-order collection, retain oneOf failed branches, preserve empty property names and isolate mutable error values through Commons. Verify 1,207 new reference cases (7,794 Validation total), 32 concurrent callers, callback counts and single serialization snapshots. All 19 packaged modules/16 consumers, both CGO-disabled Linux builds, 343/343 Docker assertions and 14/14 Batch artifact checks passed (2026-09-16, Asia/Shanghai). All temporary containers/network were cleaned; no AWS deployment occurred. Exhaustive mixed-type, reference and strict-schema gates remain open. See [VALIDATION_APPLICATORS.md](VALIDATION_APPLICATORS.md).
- [ ] V-07: Match JavaScript regex, Unicode, numeric, malformed input and schema graph boundaries through actual reference cases. Preserve explicit differences until closed.
- [x] V-07b: Preserve referenced empty combinators, fractional/negative/large size limits, non-string required values and the default floating-point/parseInt multipleOf decision. Verify 3,433 new cases (16,203 Validation total), 32 concurrent callers with mutable diagnostics, all 19 modules/16 consumers, both CGO-disabled Linux builds, 403/403 Docker assertions and 14/14 Batch checks (2026-09-16, Asia/Shanghai). The complete run cleaned temporary resources and used no AWS resources; amd64 ran locally, arm64 was cross-compiled only. Complete keyword-shape/numeric/error parity remains open; see [VALIDATION_KEYWORDS.md](VALIDATION_KEYWORDS.md).
- [x] V-06b: Match scoped mixed-type rule-group ordering, integer early checks, nullable array diagnostics and numeric format placement. Discover nested IDs inside $defs and separate canonical locations from inline, compiled, alias-chain and recursive reference paths. Verify 1,271 additional reference cases (9,065 Validation total), concurrent reused targets and caller-owned type arrays. All 19 packaged modules/16 consumers, both CGO-disabled Linux builds, 355/355 Docker assertions and 14/14 Batch artifact checks passed (2026-09-16, Asia/Shanghai), with temporary resources cleaned and no AWS deployment. Exhaustive strict-mode, resource graph and type/keyword gates remain open; see [VALIDATION_REFERENCES.md](VALIDATION_REFERENCES.md).
- [x] V-07a: Verify the pure-Go ECMAScript Unicode regex adapter, 433 pinned property tables/1,683 aliases, original-pattern diagnostics, schema-path encoding, resource errors and concurrent compiled reuse. All 6,587 Validation reference cases, 19 packaged modules/16 consumers, both CGO-disabled Linux builds, 331/331 Docker assertions and 14/14 Batch artifact checks passed (2026-09-16, Asia/Shanghai). The first Docker attempt exposed Windows locale decoding in the test runner; explicit UTF-8 decoding fixed it, and runtime-only continuation reused the verified binaries. See [VALIDATION_REGEX.md](VALIDATION_REGEX.md).
- [ ] V-08: Complete Parser/Validation documentation and composition, Event Handler integration and live Lambda acceptance when requested.
- [ ] V-09: Measure compilation/validation latency, allocations, binary size and schema/payload resource limits; complete dependency/license review and independent release setup.

See [JSON_SCHEMA_VALIDATION.md](JSON_SCHEMA_VALIDATION.md) for current API behavior and known boundaries. The complete utility remains unfinished until all applicable gates pass.

## Finite default boundary audit #189

This finite audit executes the pinned public TypeScript v2.35.0 `validate` factory
with its actual default `new Ajv({ allErrors: true })`, without injecting an engine
or changing strictness. Forty-three observations supplement the existing setup,
strictness, reference, graph, conditional, keyword and regex corpora. Two more
observations retain errors thrown during external registration before AJV's
compilation catch. The generator is `tools/reference/generate-validation-boundaries.mjs`.

| Boundary | Reviewed result / native contract |
| --- | --- |
| Default factory | True/false schemas, nullable, no coercion, no defaults and no additional-property removal retain exact results or all-error arrays. Existing wrappers keep their separate literal-false policy. |
| Reachable strictness | Unknown keywords, unregistered email format, invalid regex, nullable setup and invalid length definitions are compared in unused and referenced forms. Original-document structural checks remain distinct from reachable compilation. |
| Default AJV warnings | Twelve strictTypes warnings are preserved in the fixture. Go has no logger option and deliberately does not emulate AJV's warning side effects; errors and payload diagnostics are compared exactly. |
| Reference identities | Duplicate document IDs, duplicate fragment IDs and missing registered references fail compilation. Existing ordered registration, alias and recursive graph fixtures remain checked; schema loading stays local. |
| External registration failure | Actual AJV emits raw Error before its compile catch. Go consistently returns `SchemaCompilationError` with its native cause for duplicate/invalid unused external documents; both original observations remain unmodified. |
| Conditional/numeric/regex diagnostics | Both conditional branches, floating-point multipleOf/maximum boundaries, astral Unicode, property escapes and invalid patterns retain full keyword/message/params/schema-path arrays in original order. |
| Native ownership/cancellation | Referenced object-valued enums and returned nested allowedValues remain private snapshots. A channel-fenced format callback runs once; cancellation during or before evaluation is retained. Existing compiled concurrent callers and extension error identity remain covered. |
| Native JSON errors | JSON v2 duplicate-key, malformed UTF-8, lone-surrogate and non-finite failures remain operational decode/encode errors, without being converted to schema rejection. |

No production defect was reproduced in these rows. Packaged Validation checks
and its independent consumer use CGO disabled and GOWORK off. Exhaustive V-05,
V-06, V-07 and V-09 resource gates remain open. Optional Draft 2019-09/2020-12
contracts are unchanged; deferred Draft 4/6 work is excluded. No AWS acceptance
is claimed.
