---
description: "Configure strict schema validation in Powertools for Go and understand compilation checks, compatibility and diagnostics."
---

# Validation strict-schema compatibility

Reference: Powertools TypeScript v2.35.0 and AJV v8.20.0. Strict-schema compatibility remains an open umbrella requirement. The pattern-overlap and scoped schema-setup milestones below are implemented.

## Property and pattern overlap

AJV uses two different regular-expression modes in its default implementation. Payload matching for `patternProperties` uses Unicode mode. The strict check for a declared property matching a pattern uses `new RegExp(pattern)` without `u`. Reusing the payload matcher for that check changes schema compilation results.

The Go adapter now follows the distinction:

- Strict matching preserves UTF-16 code units. An astral property name occupies two units, so `^..$` matches it in the overlap check while `^.$` does not.
- Runtime Unicode matching still treats that character as one code point and uses the existing Unicode property tables.
- Property escapes such as `\p{Letter}` are identity escapes in the strict check. Unicode matching continues to interpret them as property classes.
- Code-point escapes, literal astral characters, surrogate escapes, character classes, quantifiers, lookarounds and named/numeric backreferences follow the applicable mode.
- Dot matching excludes all four JavaScript line terminators.
- Matching is skipped when there are no named properties or when every pattern schema is always valid, following the reference's optimization. If any pattern has a constraint, overlap checks also apply to always-valid pattern entries.
- A pattern valid in Unicode mode can fail to compile in the legacy check, such as an astral character range whose UTF-16 expansion creates a reversed range. That failure remains a schema compilation error when a check is required.

The legacy matcher is private to strict compilation. It uses the existing pure-Go regex dependency, with shared timeout and stack configuration. A match resource failure retains `RegexError` through `SchemaCompilationError.Unwrap`. No JavaScript runtime or new module dependency is introduced.

`tools/reference/generate-validation-strict.mjs` records 3,047 actual reference cases using 29 distinct patterns, 21 property names, constrained/always-valid/annotation-only pattern schemas, and absent/empty named-property maps. Complete diagnostic fields and order are preserved. The combined Validation corpus contains 12,112 cases. A separate unit test checks bounded-stack failure identity. Local Lambda probes cover UTF-16 overlap rejection and Unicode payload validation through dot and property-escape patterns.

Generate this corpus from `tools/reference` with `node generate-validation-strict.mjs`. Use `CGO_ENABLED=0` for `go test ./validation/...`; complete packaged-module, cross-build and Docker acceptance uses `uv run python integration/local/run.py`.

Acceptance (2026-09-16, Asia/Shanghai): all 19 packaged modules passed tests/vet/tidy checks, sixteen standalone consumers built, both Linux architectures compiled with CGO disabled, and Docker passed 364/364 assertions across five invocations. All temporary containers/network were cleaned. The same artifacts passed 14/14 Batch checks without extra invocations. Module checks and builds ran in the same complete acceptance command. Docker executed amd64; arm64 was cross-compiled only. No AWS resources were used.

## Schema setup and reachability

A read-only probe against the pinned reference compared the following definition bodies with and without a root `$ref`. The payload was `"value"`, and the root also declared `type: "string"`.

| Definition body | Unused `definitions` entry | Referenced `definitions` entry | Unused `$defs` entry | Referenced `$defs` entry |
| --- | --- | --- | --- | --- |
| `{"typo":true}` | Accepted | Compilation error | Accepted | Compilation error |
| `{"format":"email"}` | Accepted | Compilation error | Accepted | Compilation error |
| `{"type":"bogus"}` | Compilation error | Compilation error | Accepted | Compilation error |
| `{"nullable":true}` | Accepted | Compilation error | Accepted | Compilation error |
| `{"minLength":-1}` | Compilation error | Compilation error | Accepted | Accepted |
| `{"pattern":"(?i)a"}` | Accepted | Compilation error | Accepted | Compilation error |

These original 24 observations motivated `generate-validation-setup.mjs`, which now records 658 actual reference cases. The corpus covers unused definitions, direct pointers, alias chains, nested resource IDs, unused/referenced external assertions, invalid unused Draft 7 structure, and always-valid patterns with and without additional-property rules. Together with the earlier corpora, 12,770 Validation cases pass. The original probe is not added separately to that total.

The adapter validates each original document against Draft 7 structure before rewriting it. This includes registered external documents that are never used. Compilation-only failures are recorded by canonical schema location and rejected when encountered in the compiled graph. The engine's regex-format checks do not compile every source pattern; actual Unicode matchers are bound while adapting the reachable graph, before the validator is published. No runtime lazy compilation, JavaScript runtime, new dependency, or application callback is introduced by this setup pass.

Always-valid pattern schemas require no regex matcher unless `additionalProperties` needs their names. For example, `patternProperties: {"[": true}` compiles by itself, but adding `additionalProperties: false` requires the invalid matcher and fails compilation. Structural errors in unused `definitions`, such as a negative `minLength`, remain rejected. A referenced negative `minLength` in `$defs` remains accepted, matching the reference's separation of Draft 7 metaschema coverage and compilation.

Invalid external registration retains the Go `SchemaCompilationError` wrapper; the reference's `addSchema` call can throw an ordinary `Error` outside its compilation catch. That error-contract difference remains open. Custom formats affecting metaschema format names, malformed keywords inside `$defs`, arbitrary referenced data fragments, conditional optimization and exhaustive resource-graph/dialect behavior remain open. V-05 stays unchecked until the complete setup/default/extension contract is verified.

Setup acceptance (2026-09-16, Asia/Shanghai): all 19 independently packaged modules passed tests/vet/tidy checks and sixteen consumers built. Both Linux architectures compiled with CGO disabled; amd64 Docker passed 382/382 assertions across five invocations, with cleanup. The same artifacts passed 14/14 Batch checks. Module checks and builds ran in the same complete command. No AWS resources were used; arm64 was not executed locally. The Windows blank-import Validation consumer is 5,567,488 bytes, an artifact size rather than a performance baseline.

### Referenced `$defs` keyword findings

A subsequent read-only reference probe tested eleven keyword bodies through `#/$defs/value` with `""`, `"a"`, `"aa"`, `0` and `{}`. These 55 observations motivated the implemented [keyword adapter and its 3,433-case corpus](VALIDATION_KEYWORDS.md). They are not counted separately in the passing total:

| Body | Observed reference behavior |
| --- | --- |
| `{"minLength":1.5}` | String lengths below 1.5 fail with the original fractional limit in diagnostics. |
| `{"maxLength":-1}` | Every tested string fails; non-string values pass. |
| `{"allOf":[]}` | All tested values pass. |
| `{"anyOf":[]}` | All tested values fail with the ordinary anyOf summary. |
| `{"oneOf":[]}` | All tested values fail with `passingSchemas: null`. |
| `{"enum":[]}` | Compilation fails. |
| `{"type":[]}` | All tested values pass. |
| `{"type":["string","string"]}` | Strings pass; other tested values fail with duplicate types retained in diagnostics. |
| `{"required":[1]}` | The empty object fails with numeric `missingProperty: 1`; other tested values pass. |
| `{"multipleOf":0}` | Zero fails validation with divisor zero; non-number values pass. |
| `{"type":"string","nullable":"yes"}` | Compilation fails. |

The default Go adapter now covers these scoped keyword behaviors, including original diagnostic values, while retaining root/`definitions` structural rejection. Empty combinators, fractional limits, required values and floating-point multipleOf have dedicated coverage. Other malformed keyword forms remain open; validating all `$defs` fragments against the metaschema would still reject reference-supported behavior.

## Remaining gates and inspected sources

Exhaustive legacy capture-name and malformed-native-string behavior remains part of V-07. Other strict options, plugins, remaining compilation timing boundaries, special JavaScript object names, anchors/resource graphs, performance limits and Parser/Event Handler integration remain open.

Inspected sources: installed `ajv/dist/vocabularies/applicator/patternProperties.js`, `ajv/dist/vocabularies/code.js`, `ajv/dist/compile/util.js`, the Powertools Validation entry point, and regexp2/v2 v2.8.0 parser and rune-matching APIs. The pattern utility remains Unicode by default; this separate matcher exists solely because the reference's strict overlap check uses a different mode.
