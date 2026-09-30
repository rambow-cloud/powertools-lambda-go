# Validation type groups and references

Reference: Powertools TypeScript v2.35.0 with AJV v8.20.0. The default Go compiler uses jsonschema/v6 v6.0.3. This milestone extends the existing applicator adapter; it does not establish exhaustive AJV equivalence.

## Type diagnostics

AJV evaluates untyped rules, followed by number, string, array and object rule groups. A single declared type with applicable rules can defer its type error until its group. An `integer` declaration uses the early type check, even when numeric constraints exist. A single-element type array follows the same group rule as a scalar declaration. Custom numeric formats belong to the number group; string formats belong to the string group.

The adapter now records that group instead of assigning every delayed type error the same rank. Reference cases combine constraints for multiple value types, constant/enum failures, nullable declarations and both format callback types. Error ordering remains exact and nested scope ordering uses the existing collector.

For an array-valued type declaration, AJV can append `null` while compiling `nullable: true`. Its resulting type diagnostics include that appended member. Go retains caller ownership by changing only its private snapshot and emits the same type parameters and message. The fixture generator snapshots each schema before invoking AJV and uses an independent copy per case, preventing compiler mutation from contaminating later cases or the recorded input.

## Reference locations

Canonical schema locations and displayed error paths serve different purposes. The engine uses canonical locations to resolve resources and the adapter uses them for source metadata and ordered traversal. AJV's displayed paths depend on its compilation strategy:

- A reference-free target is normally inlined and its diagnostic prefix is the literal `$ref`, including a relative URL or an escaped pointer.
- A target containing references is compiled separately and its local diagnostic path starts at `#`.
- A JSON Pointer to a reference-only alias can resolve through an alias chain while retaining the calling reference's diagnostic prefix.
- References inside a compiled target establish their own scope. Recursive instance paths continue to grow while the referenced schema path can restart at `#`.
- AJV's default inline decision scans annotation/data values for reference-named keys as well. The adapter follows that behavior for diagnostic presentation.

The collector passes presentation scope separately from canonical location. This lets two references to the same compiled target retain different error prefixes without mutating shared schema state. Graph edges are indexed once during compilation; request-time collection does not compile or validate targets again.

The compiler also registers `$defs` as a schema-container vocabulary for its Draft 7 engine. This permits nested absolute and relative `$id` resources inside `$defs` to be discovered before resolving their references. It does not change the default dialect, enable later-draft validation keywords or permit unregistered network/file loading.

## Reference evidence

`tools/reference/generate-validation-references.mjs` produces 1,271 cases with the actual pinned utility. The corpus covers seven JSON type declarations, scalar/single-element/nullable type forms, mixed rule groups, custom formats, root and nested IDs, relative/absolute URLs, escaped names, reference chains, external schemas, recursion, reused targets and data-valued annotations. Combined Validation coverage is 9,065 cases: 502 core, 6,085 regex, 1,207 applicator and 1,271 type/reference cases.

Expected errors retain every field and array order. External schema fixtures remain raw JSON so declaration order is not lost during test setup. Additional tests verify 32 concurrent calls through relative and absolute references to one target, nullable-array diagnostic snapshots and caller-data ownership.

Local Lambda probes cover mixed-type error order, nullable-array parameters, two spellings of a nested resource reference, and recursive compiled scope across warm invocations. See the current [module acceptance](MODULE_ACCEPTANCE.json) and [local acceptance](LOCAL_ACCEPTANCE.json) for completed runtime evidence.

Acceptance (2026-09-16, Asia/Shanghai): all 19 packaged modules passed tests/vet/tidy checks, sixteen standalone consumers built, both Linux architectures compiled with CGO disabled, and Docker passed 355/355 assertions across five invocations. All temporary containers/network were cleaned. The same artifacts passed 14/14 Batch checks without additional invocations. Module checks and both builds ran as part of this complete acceptance command. Docker executed amd64; arm64 was cross-compiled only. No AWS resources were used.

## Remaining work

V-05, V-06 and V-07 remain open. Remaining boundaries include exhaustive identifier/anchor/duplicate-ID setup and resource graphs; unused definitions and schema compilation timing; AJV inherited-property and special-name behavior; non-Unicode strict property/pattern overlap checks; pathological cyclic schemas; numerical precision and malformed native values; custom format/plugin and injected-engine options; and operational callback traversal order outside verified cases. Performance/resource limits, Parser/Event Handler integration and public release remain separate gates.

The scoped cases above do not prove all reference graphs or all type/keyword combinations. Existing Go snapshot ownership, map-order and wrapper adaptations remain explicit in [JSON_SCHEMA_VALIDATION.md](JSON_SCHEMA_VALIDATION.md) and [VALIDATION_APPLICATORS.md](VALIDATION_APPLICATORS.md).

The later [strict-pattern milestone](VALIDATION_STRICT.md) implements the separate legacy overlap matcher and adds 3,047 cases (12,112 Validation total). Its schema-setup research retains explicit unused-definition differences for subsequent implementation.

## Inspected sources

- Installed `@aws-lambda-powertools/validation/lib/esm/validate.js`.
- Installed `ajv/dist/compile/validate/{dataType,applicability,index}.js` for type-group selection and ordering.
- Installed `ajv/dist/compile/{resolve,index}.js` and `ajv/dist/vocabularies/core/ref.js` for inline decisions, alias resolution and diagnostic scopes.
- `github.com/santhosh-tekuri/jsonschema/v6@v6.0.3/{compiler,roots,draft,position,vocab}.go` for resource discovery and schema-container vocabulary support.
