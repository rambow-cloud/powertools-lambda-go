# Validation condition graphs and external resource registration

Reference: Powertools TypeScript v2.35.0 with AJV v8.20.0. This extends [schema-shape compatibility](VALIDATION_SHAPES.md); the full Validation compatibility gate remains open.

## Conditional compilation

When both branches are always valid, AJV does not compile the condition. Missing local or external references, unknown formats and invalid regex syntax inside that condition must therefore remain inactive. An active reference to the original `#/if` location must still compile and validate that node. Deleting the condition from the schema would violate this requirement.

Original documents are still checked against the Draft 7 metaschema before adaptation. The private engine uses its Draft 6 assertion core with a vocabulary that controls Draft 7 conditional compilation. The vocabulary queues the condition and nontrivial branches only when required, retains original locations, and installs the engine's conditional validation fields before publication. It does not evaluate format callbacks during compilation. Both nontrivial branches compile even when the condition is a constant; runtime validation selects its branch once.

The private engine selection does not change the public schema dialect to Draft 6. Draft 6 declarations are not newly registered. Draft 7 structural rules, conditional behavior, existing adapters and exact diagnostics remain part of the Go contract.

## Dialect declarations

The registered root document selects the dialect. The default accepts an absent or empty declaration, the HTTP Draft 7 metaschema URL, and AJV's `http://json-schema.org/schema` alias, with an optional trailing hash. Unregistered root dialects fail compilation. Nested `$schema` declarations remain annotations, including nested resources with their own `$id`; their original structural constraints still apply where the Draft 7 metaschema traverses them. Only the private engine documents have declarations removed.

## External resources

The engine discovers nested IDs within a loaded document but does not globally index them across registered documents. The adapter registers address proxies to their original JSON pointers, resolving relative IDs through ancestor resource scopes. Active compiled references are unwrapped before validation so proxies do not add diagnostic path segments or duplicate callbacks. Escaped pointer names retain their source locations.

`CompileOptions.ExternalSchemas` accepts a slice of schema values registered by `$id` in slice order. Use this form to preserve the TypeScript array registration sequence. `ExternalRefs` remains available for explicit URL-keyed registration; it registers in lexical URL order before `ExternalSchemas`. Neither option introduces remote or file loading. References must resolve within explicitly registered resources.

```go
options := validation.Options{CompileOptions: validation.CompileOptions{
    ExternalSchemas: []any{
        json.RawMessage(`{"$id":"https://example.test/document","$defs":{"value":{"$id":"number","type":"number"}}}`),
    },
}}
schema, err := validation.Compile(ctx,
    json.RawMessage(`{"$ref":"https://example.test/number"}`), options)
```

For the verified case where different documents declare the same nested resource ID, AJV replaces the pointer alias in registration order. The adapter matches that behavior. This does not establish complete duplicate-root, root-versus-nested, fragment-ID, or relative-document-ID compatibility; those registration/error boundaries remain open. Go currently wraps registration errors in `SchemaCompilationError`, while TypeScript can throw an ordinary error before its compilation catch.

## Evidence

`generate-validation-graphs.mjs` generates 1,104 exact reference cases:

- 576 condition/branch cases, with and without explicit Draft 7 declarations.
- 240 root/nested dialect cases.
- 72 active references into conditional nodes and their definitions.
- 18 resource IDs inside conditional nodes and 18 external condition targets.
- 144 nested relative-ID, pointer-escaping and alias-chain cases.
- 12 cross-document chain cases and 24 order-dependent duplicate-alias cases.

The combined Validation corpus contains 23,071 cases. Results, error types, messages and ordered issue fields are compared without diagnostic normalization. Tests additionally cover caller-schema immutability, snapshots after caller mutation, 32 concurrent callers, exactly-once active callbacks and ignored-condition callback suppression. The local Lambda fixture adds six checks per successful invocation for skipped/active missing references, nested dialects, preserved condition paths, external nested IDs and registration order.

Generate the corpus from `tools/reference` with `node generate-validation-graphs.mjs`. Use `CGO_ENABLED=0` for every Go command. Full packaged-module, Linux cross-build and Docker verification uses `uv run python integration/local/run.py`.

Acceptance (2026-09-16, Asia/Shanghai): all 19 independently packaged modules passed tests, vet and tidy checks; sixteen independent consumers built. Both Linux architectures compiled with CGO disabled. Docker passed 442/442 assertions across five invocations and cleaned its temporary containers/network. The same artifacts passed 14/14 Batch checks without additional invocations. Module checks and builds executed in the complete acceptance run. Docker ran amd64; arm64 was cross-compiled only. No AWS resources were used. Evidence: [MODULE_ACCEPTANCE.json](MODULE_ACCEPTANCE.json), [LOCAL_ACCEPTANCE.json](LOCAL_ACCEPTANCE.json) and [BATCH_ACCEPTANCE.json](BATCH_ACCEPTANCE.json).

## Remaining scope

Keep V-05/V-06/V-07 open for exhaustive schema graph/setup behavior, registration error identity, anchors, ambiguous IDs, arbitrary data-fragment references, native JavaScript object behavior, plugins/options, and complete keyword/regex/numeric diagnostics. Parser/Event Handler composition, resource budgets, live-service acceptance and public release remain separate requirements.
