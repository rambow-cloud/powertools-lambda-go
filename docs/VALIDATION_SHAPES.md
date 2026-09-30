# Validation schema-shape and conditional compilation compatibility

Reference: Powertools TypeScript v2.35.0 with AJV v8.20.0. This milestone extends [schema setup](VALIDATION_STRICT.md) and [keyword behavior](VALIDATION_KEYWORDS.md). Complete AJV compatibility remains open.

## Reachable keyword types

Original-document Draft 7 structural validation remains separate from compilation. Inside a reachable `$defs` fragment, AJV still checks assertion keyword types even though the Draft 7 metaschema does not traverse that container. The adapter records those failures at their schema locations and rejects them when the corresponding compiled schema is reached.

The checks cover numeric limits/divisors, patterns/formats/references, boolean uniqueness/nullable flags, object property/pattern/dependency maps, array-valued required/combinator/enum rules, boolean/object subschemas and the array form of items. Contextual checks still distinguish an additionalItems rule without tuple items, if without branches, an unknown format and an empty enum. Unused `$defs` entries do not acquire eager assertion checks.

Some unvalidated forms have no rules in the reference. Numeric schema fragments, empty strings and empty arrays can act as always-valid fragments through `$defs`; root or ordinary Draft 7 schema positions remain subject to original structural validation. An empty type string is similarly inactive when original structure permits it. These observations must not be generalized into acceptance of arbitrary primitive schemas.

The known annotation keywords `deprecated` and `contentSchema` are accepted without adding payload assertions or recursively compiling their contents. They also participate in the always-valid pattern/branch optimization. An annotation-only schema must not be mistaken for an assertion-bearing schema.

## Conditional compilation and execution

AJV checks nontrivial then/else branches even when if is a literal boolean. For example, `{if: true, then: true, else: {format: "unknown"}}` fails compilation. The Go engine normally omits the unselected branch; a private compiler vocabulary now retains the necessary compilation edges. These temporary edges are removed after the graph is adapted and never cause additional runtime branch validation.

When both branches are always valid, the condition does not affect validation. The adapter skips that compiled condition, including scoped unknown-keyword, unknown-format and regex checks. Unknown keywords in a branch itself remain compilation errors. Actual selected branches still execute once; format callbacks do not run during compilation or in an ignored condition.

This milestone originally left pre-adaptation resolution and dialect handling open. The subsequent [condition-graph implementation](VALIDATION_GRAPHS.md) now covers ignored unresolved references and nested dialect declarations while preserving active reference targets.

A subsequent read-only probe tested four conditions in `{if: condition, then: branch, else: true}` with payload `{}`. These eight observations were excluded from the original shape corpus and are now covered by the separate graph corpus:

| Condition | `then: true` | `then: false` |
| --- | --- | --- |
| `{"$ref":"#/missing"}` | Accepted | Compilation error |
| `{"$ref":"https://example.test/missing"}` | Accepted | Compilation error |
| `{"$schema":"https://example.test/dialect"}` | Accepted | Validation error |
| `{"properties":{"value":{"$ref":"#/missing"}}}` | Accepted | Compilation error |

The graph adapter preserves skipped reference resolution and nested dialect handling without discarding schema locations that another active reference may target. It controls compilation edges instead of deleting ignored conditions.

## Property dependencies

Property dependency arrays reuse required-entry conversion. Non-string names retain the original diagnostic value or the reference's generated array-name string, while `deps`, `depsCount`, declaration order and duplicate failures remain intact. Schema dependencies continue to use the normal compiled schema graph. Returned mutable diagnostic parameters use Commons snapshots.

## Evidence and reproduction

`tools/reference/generate-validation-shapes.mjs` captures 5,764 reference cases, comparing root, referenced and unused definitions, contextual items/conditional/nullable rules, schema-container entries, condition/branch matrices, direct fragments and mixed property dependencies. Complete validation error fields and order are compared. Combined Validation coverage is 21,967 cases. The earlier 126 keyword-shape observations are covered by the expanded corpus and are not counted separately.

A separate callback test proves that compiling both branches does not execute either callback, the selected branch executes exactly once, and an ignored condition does not execute its callback. All previous concurrency, diagnostic ownership and numerical rule tests remain part of the full suite.

Generate fixtures from `tools/reference` with `node generate-validation-shapes.mjs`. Run Go tests with `CGO_ENABLED=0`; complete packaged-module, cross-build and Docker acceptance uses `uv run python integration/local/run.py`. No runtime JavaScript, dependency, public API or new module is required by this change.

Acceptance (2026-09-16, Asia/Shanghai): all 19 independently packaged modules passed tests/vet/tidy checks, sixteen consumers built, and both Linux architectures compiled with CGO disabled. Docker passed 424/424 assertions across five invocations and cleaned all temporary containers/network. The same saved artifacts passed 14/14 Batch checks without additional invocations. Module checks and builds ran in the same complete acceptance command. Docker executed amd64; arm64 was cross-compiled only. No AWS resources were used.

## Remaining requirements

Keep V-05/V-06/V-07 open for complete export/options/schema setup and diagnostic parity. The graph milestone closes the scoped ignored-reference and nested-dialect cases above. Remaining cases include complete resource identities and dialect registration, arbitrary data-fragment references, prototype/inherited-property behavior, native JavaScript object coercion, non-finite numbers, custom plugins/options and errors outside the reference's compile catch. Parser/Event Handler integration, performance and release requirements remain separate gates.
