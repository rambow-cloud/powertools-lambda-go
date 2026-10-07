# Additional Validation dialects and extensions

Planning acceptance for [issue #118](https://github.com/rambow-cloud/powertools-lambda-go/issues/118), reviewed on 2026-10-08. The current default remains the documented Draft 7/AJV-compatible subset. No new option, engine or dependency is implemented by this design.

## Support matrix

Powertools TypeScript v2.35.0 creates AJV with `allErrors: true` unless the caller injects an instance. AJV v8.20.0 is pinned in `tools/reference/package-lock.json`. Capabilities available to an injected instance are not defaults of the utility. [AJV's dialect guide](https://ajv.js.org/guide/schema-language.html) distinguishes its default export from the 2019/2020 engines.

| Capability | Current default Go compiler | Proposed boundary |
| --- | --- | --- |
| Draft 7 assertions, nullable, reference siblings | Scoped adaptation and ordered diagnostics; 23,071 recorded reference cases | Keep existing behavior and known limits |
| Root Draft 4/6/2019-09/2020-12 declaration | Unregistered dialect error; underlying jsonschema/v6 engine capability is not the public adapter contract | Explicit compiler selection and pinned dialect corpus |
| Nested `$schema` | Treated as an annotation in the recorded graph scope | Preserve default behavior; a new dialect compiler must define resource boundaries |
| `$defs`, content annotations, `default` | Recognized annotations/reference containers; no automatic content assertion or default insertion | Do not mistake recognized syntax for mutation or newer-draft support |
| `dependentRequired`, `dependentSchemas`, `unevaluatedProperties/Items`, `prefixItems`, dynamic references | Not in the default keyword allowlist | Separate dialect vocabulary, not keyword aliases in Draft 7 |
| Registered string/numeric formats | `Formats`, `NumberFormats`; callbacks must be concurrency-safe | Object/regex/async format forms need a separate explicit contract |
| `useDefaults`, `coerceTypes`, `removeAdditional` | No built-in options; default validator checks a JSON snapshot | Keep caller ownership; any transformation API needs review |
| `$data`, custom keywords, ajv-errors/plugins, JTD | No built-in extension implementation | Defer arbitrary plugin/mutation support; use the existing injected `Compiler` boundary |
| AJV instances/strictness/error ordering | Existing `Compiler`/`Validator` interfaces permit application adapters; no universal equivalence | New adapters must state their option and diagnostic scope |

Unknown reachable keywords and root dialects must fail compilation, never silently disable assertions. Existing unreachable-definition and ignored-condition rules are intentionally scoped; a new compiler must not change them for default users. Inspect `validation/compiler.go`, `schema_setup.go`, `schema_rules.go` and the existing strict/graph corpora before changing traversal.

## Pinned reference observations

Thirteen cases were executed once against the installed v2.35.0 utility and AJV v8.20.0 on 2026-10-08, using explicit injected instances where specified. These are TypeScript observations, not new Go parity acceptance. Inputs below identify reproducible cases; object property schemas use the stated types.

| Case | Schema/input and instance | Observed result |
| --- | --- | --- |
| default-modern-dialect | Root `$schema: https://json-schema.org/draft/2020-12/schema`, `type: string`, input `"ok"`, default | `SchemaCompilationError` |
| default-new-keyword | Object `dependentRequired: {a:[b]}`, input `{a:1}`, default | `SchemaCompilationError` |
| 2019-dependent-required | Same dependency, root 2019-09 URI, input `{a:1}`, injected Ajv2019 | Validation issue `#/dependentRequired`, missing property `b` |
| 2019-unevaluated | Object with integer property `a`, `unevaluatedProperties:false`, input `{a:1,b:2}`, Ajv2019 | Issue `#/unevaluatedProperties`, `unevaluatedProperty:b` |
| 2020-prefix-items | Array `prefixItems:[{type:integer}]`, `items:false`, input `[1,2]`, Ajv2020 with `strictTuples:false` | Issue `#/items`, limit `1` |
| default-annotation | Object string property `id` with default `Ada`, input `{}`, default | Success; input and result remain `{}` |
| injected-defaults | Same schema/input, Ajv with `useDefaults:true` | Input and returned object become `{id:"Ada"}` |
| injected-coercion | Object integer property `age`, input `{age:"42"}`, `coerceTypes:true` | Input and result become `{age:42}` |
| injected-removal | Object string property `id`, `additionalProperties:false`, input `{id:"Ada",extra:true}`, `removeAdditional:true` | Input and result become `{id:"Ada"}` |
| default-data-reference | Number properties `limit` and `value`, `value.minimum: {$data:"1/limit"}`, input `{limit:3,value:2}`, default | `SchemaCompilationError` |
| injected-data-reference | Same schema/input, Ajv with `$data:true` | Issue `/value`, `#/properties/value/minimum`, limit `3` |
| default-custom-keyword | Integer with `even:true`, input `3`, default | `SchemaCompilationError` |
| injected-custom-keyword | Same schema/input, registered numeric boolean `even` predicate | Issue `#/even`, keyword `even` |

Injected engines use `allErrors:true` and `logger:false`; default rows use the utility's own instance. An injected AJV mutates object inputs for the options shown. That is a material contract difference from this project's default snapshots. [AJV's mutation guide](https://ajv.js.org/guide/modifying-data.html) documents that these options are nonstandard and evaluation order can affect results.

## Proposed implementation slices

Prioritize a separately selected 2020-12 compiler through the existing `Options.Compiler` seam, reusing the pinned Go engine where suitable. A dedicated adapter avoids routing newer vocabularies through Draft 7 diagnostic rewrites. Keep the default compiler, public errors, no-network reference loading and reusable schema ownership intact. Specify whether its errors follow JSON Schema engine diagnostics or AJV2020 diagnostics before implementation; do not claim both automatically.

2019-09 can follow only with its own draft and reference-registration cases. Draft 4/6, JTD, arbitrary plugin registration and mutation remain deferred. Defaults/coercion/removal require a returned transformed snapshot, with no caller mutation, or an explicitly approved different ownership API; the existing `Validator` interface returns diagnostics and is not a general transformation interface. No additional speculative wrapper is approved here.

## Required acceptance before adding an adapter

- Generate pinned positive/negative Ajv2019/Ajv2020 cases through the v2.35.0 injection path. Preserve full ordered issue arrays, paths, parameters, messages and compilation-versus-validation timing; include the observations above and successful counterparts.
- Cover evaluated-property/item accounting across allOf/anyOf/oneOf, conditional branches, external references and failed branches; dependency presence and empty lists; tuple bounds; recursive/dynamic anchors and duplicate resource IDs.
- Retain JSON v2 rejection of invalid Unicode/duplicate members. Test astral characters, combining marks, property escapes, lookarounds/backreferences and malformed patterns against the selected dialect's regex semantics.
- Explicitly measure numeric differences: large integers, exponent notation, negative zero, fractional multipleOf and non-finite native input. Do not widen the existing floating-point parity claim to newer engines.
- Compile immutable schema/reference snapshots once. Test overlapping validation, reentrant callbacks, cancellation, independent result/error ownership and no validation-time mutation of shared compiled metadata. Engine traversal remains synchronous unless proven interruptible.
- For any proposed mutation slice, test standalone and inbound/outbound wrappers, failed-validation partial changes, default aliasing, repeated validation, envelope selection and branch order. Never silently enable mutation.
- Run affected packaged tests/vet/tidy and independent consumers with `GOWORK=off`, `CGO_ENABLED=0`; local Handler/Parser composition and both Linux Lambda architecture builds. New dialect support does not require cloud calls.

## Decision

The default/injected-engine inventory and prioritized acceptance proposal complete #118's design scope. New API/dependency or behavioral changes require a reviewed implementation scope. Full AJV parity remains open in [the Validation plan](VALIDATION_PLAN.md); closing this planning issue does not expand the v1 supported subset.
