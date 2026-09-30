# Validation applicator diagnostics

Reference: Powertools TypeScript v2.35.0 and AJV v8.20.0. The Go implementation uses jsonschema/v6 v6.0.3 with private compilation and diagnostic adapters. CGO remains disabled.

## Implemented behavior

- Property dependencies report one issue per missing field, retaining the complete declared dependency list, its count, the triggering property and the reference message. Property dependencies precede schema dependencies.
- Property-name failures retain their child diagnostics and optional `propertyName` field, including an empty property name. The enclosing issue has `params.propertyName`. Nested instance paths are captured before the engine reuses its traversal storage.
- Tuple rejection includes the declared tuple length in `params.limit`. Additional-item diagnostics precede tuple-item diagnostics, following AJV's rule order.
- Failed `then` and `else` branches include an `if` summary after the selected branch's issues. Nested conditionals and repeated references retain separate evaluation scopes.
- `oneOf` retains diagnostics from failed branches before the second matching branch. Each visited branch executes once; branches after the second match are skipped. A single matching branch discards branch failures.
- Ordered error collection follows schema scopes, array indices, property declaration order and payload object-key order. It preserves nested groups instead of sorting unrelated flattened issues together.
- Orphan `if`, `then`, `else` and non-tuple `additionalItems` are rejected under the reference's strict defaults. Matching named properties and nontrivial `patternProperties` are rejected; the all-always-valid pattern optimization remains allowed.
- Schema, reference and payload custom JSON marshalers run once per compilation or validation snapshot. Diagnostic ordering uses the same captured document as validation.
- Mutable type/enum/const diagnostic values use `commons.CloneValue`. Modifying returned diagnostics cannot modify subsequent validations or their error payloads.

Compiled adapters are installed before publication and keep invocation state local. They replace the two affected engine applicators rather than running a second validation pass to recover discarded errors. They introduce no dependency on Node.js or AJV in production.

## Object order in Go

Use `json.RawMessage` for schemas and payloads when source object order matters. Encoded structs retain their JSON field order. Go maps have no insertion order, so the adapter uses their deterministic JSON encoding order. Integer-index object keys precede other keys and are sorted numerically, as in JavaScript. Schema snapshots preserve their original order for reuse.

JMESPath extraction and typed wrapper conversion can produce Go maps and consequently use that representation's order. This is an explicit native-Go boundary; the adapter does not claim to recover insertion order after it has been lost.

## Evidence and reproduction

`tools/reference/generate-validation-applicators.mjs` generates 1,207 cases using the actual pinned utility. The corpus covers top-level, nested-property and array-element placements, mixed dependencies, property names, tuple and array keywords, conditionals, logical combinations, strict compilation failures and repeated references. It preserves all expected error fields and array order.

The Go fixture reader compares complete decoded JSON diagnostics. It does not decode expected errors through the Go `Issue` type, which could silently discard an unsupported reference field. It does not sort expected diagnostics. The combined Validation corpus contains 7,794 cases: 502 core, 6,085 regex and 1,207 applicator cases.

Additional unit tests cover 32 concurrent callers, per-branch callback counts, mutation of returned diagnostics and single serialization snapshots. Local Lambda probes exercise nested conditional/dependency/property-name/tuple diagnostics and `oneOf` branch retention across warm invocations.

Acceptance (2026-09-16, Asia/Shanghai): all 19 packaged modules passed tests/vet/tidy checks, sixteen standalone consumers built, both Linux architectures compiled with CGO disabled, and Docker passed 343/343 assertions across five invocations. All temporary containers/network were cleaned. The same artifacts passed 14/14 Batch checks without additional invocations. The current run executed module checks and both builds; arm64 was cross-compiled only. No AWS resources were used.

Generate fixtures from `tools/reference` with `node generate-validation-applicators.mjs`. From the repository root, use `CGO_ENABLED=0` with `go test ./validation/...`. Packaged-module and local Lambda acceptance use `uv run python integration/local/run.py`; see the recorded acceptance files for the last verified run.

## Remaining gates

This milestone does not close V-05, V-06 or V-07. Remaining work includes exhaustive mixed-type rule-group ordering; nested resource IDs, recursive references and inline/reference schema-path variants; special JavaScript object names and inherited-property behavior; strict-mode setup and unused-schema differences; and remaining regex, numeric and malformed-string boundaries. In particular, AJV checks property/pattern overlap with a non-Unicode JavaScript expression, whereas the current Go strict overlap check uses the compiled Unicode matcher. That distinction requires its own reference corpus and adaptation.

The subsequent [type-group and reference milestone](VALIDATION_REFERENCES.md) adds scoped coverage for mixed groups, nested IDs, alias chains and recursive reference presentation. Its additional 1,271 cases bring the combined Validation corpus to 9,065; exhaustive gates remain open.

The subsequent [strict-pattern milestone](VALIDATION_STRICT.md) replaces the Unicode overlap check described above with legacy UTF-16 matching and adds 3,047 cases, bringing the combined corpus to 12,112. The earlier limitation remains recorded here as historical context; unused-schema setup and exhaustive legacy boundaries stay open.

Operational callback traversal order outside the verified applicators, compiler/plugin options, context interruption during synchronous engine traversal, performance/resource budgets, Parser/Event Handler integration and public release remain separate requirements. Full feature completion stays unchecked in [VALIDATION_PLAN.md](VALIDATION_PLAN.md).

## Inspected sources

- Installed `@aws-lambda-powertools/validation/lib/esm/validate.js` and its AJV configuration.
- Installed `ajv/dist/vocabularies/applicator/{if,dependencies,propertyNames,additionalItems,items,contains,oneOf,patternProperties}.js` and `ajv/dist/compile/{rules,util}.js`.
- `github.com/santhosh-tekuri/jsonschema/v6@v6.0.3/{validator,schema,vocab}.go`: error ownership, applicator execution and extension interfaces.
