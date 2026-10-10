---
description: "Validate Go Lambda inputs and responses against JSON Schema with Powertools, compiled validators and structured diagnostics."
---

# JSON Schema Validation

Validation checks events and responses against JSON Schema. Import `github.com/rambow-cloud/powertools-lambda-go/validation`. Use it for schema documents shared with other systems; choose [Parser](PARSER.md) when composing typed Go schemas and event transformations. The reference uses Powertools v2.35.0 with AJV v8.20.0.

JSON snapshots and typed handler conversion use `encoding/json/v2`. Duplicate members and invalid Unicode are rejected; typed fields match JSON names case-sensitively. Numeric callbacks and snapshots retain exact `json.Number` tokens. Use explicit application field tags and [current JSON collection and omission rules](GETTING_STARTED.md#prerequisites-and-installation).

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Complete example

This complete Lambda example compiles inbound and outbound schemas once. Build `./examples/validation` with `CGO_ENABLED=0`. It accepts an order, returns its ID, and validates that the response is a nonempty string.

~~~go
--8<-- "examples/validation/main.go"
~~~

## Input and output

Input `{"id":"ORD-123","amount":42}` returns the JSON string `"ORD-123"`. Input `{"id":"","amount":0}` produces `Inbound schema validation failed` with issues for `id` and `amount`, before business code runs. A successful business result violating the outbound schema produces `Outbound schema validation failed`. The example writes no successful application log record. Compilation errors are handled during initialization, separately from request errors.

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `inbound` / `outbound` | Reusable compiled validators; nil disables a wrapper stage. |
| `input` | Typed order decoded after inbound validation; the wrapper applies JSON snapshot ownership. |
| `SchemaValidationError` | Carries ordered issues; use `errors.As` to inspect paths, keywords and parameters. |

## TypeScript feature coverage

Compared with the [official v2.35.0 validation guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/validation.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| Standalone / decorators / middleware | `Validate`, `Compile`, `WrapHandler` | Compiled typed wrapper; business errors/panics are preserved. |
| Event extraction / envelopes | `Options.Envelope` | JMESPath extraction on input only. |
| Decode query functions | `Options.QueryOptions` | Opt in to Powertools functions explicitly. |
| Custom formats | `Formats`, `NumberFormats` | No implicit email-format support; callbacks must be concurrency-safe. |
| External references | `ExternalRefs`, `ExternalSchemas` | Registered locally; no remote schema loading. |
| Custom AJV instance | `Options.Compiler` interface | Go compiler/validator injection, not an AJV object. |
| Keywords / errors / Unicode regex | Pure-Go Draft 7 adapter and issue mapping | Scoped reference coverage; complete AJV parity remains open. |

Executable evidence: [validation/reference_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/validation/reference_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.

## Usage

```go
schema, err := validation.Compile(ctx, json.RawMessage(`{
  "type": "object",
  "required": ["id"],
  "properties": {"id": {"type": "string", "minLength": 1}}
}`), validation.Options{Envelope: "body"})
if err != nil {
    return err
}
value, err := schema.Validate(ctx, event)
```

`Validate(ctx, payload, schema, options)` compiles for one call. `Compile` retains a schema for concurrent warm-invocation reuse. Schema documents and format/reference registrations are captured during compilation; changing the original maps afterward does not change the compiled validator. Callbacks must be safe for concurrent use. Successful standalone validation returns the original payload when there is no envelope; JMESPath returns its extracted representation. No defaults, coercion or removal of additional fields are enabled by default.

`WrapHandler[I,T,R](inbound, outbound, handler)` accepts the raw event type `I`, validates and converts it to `T`, invokes the business handler, and validates its successful result `R`. Input is snapshotted through JSON before validation, including typed events containing slices or pointers. Only inbound validation applies an envelope. Business results with errors and panic identity are preserved. The wrapper reuses the shared invocation context. See [the native Lambda example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/validation/main.go).

As in the reference middleware, a literal boolean `false` schema passed through `Compile` is skipped by the wrapper. Standalone validation against that same compiled schema rejects all values. A nil wrapper schema disables that stage. Go's wrapper accepts precompiled schemas; compilation failures are therefore normally handled during initialization, before invocation. It does not emulate the TypeScript decorator's un-cloned input variant.

## Extension points

- `CompileOptions.Formats` registers string callbacks returning nil on success. Non-string values do not invoke them. No standard format names, including `email`, are enabled implicitly by the reference's default AJV instance. Register each required format.
- `CompileOptions.NumberFormats` registers numeric callbacks. The default engine supplies `json.Number`, preserving the original numeric token. Defining the same name in both format maps is an error. Built-in engine format names such as `regex` can be overridden through the adapter.
- `CompileOptions.ExternalRefs` maps reference resource URLs to schema documents. Use absolute URLs as keys. Relative `$ref` values use schema `$id` resolution. Unregistered references fail compilation; the loader never fetches files or URLs.
- `CompileOptions.ExternalSchemas` registers schema values by `$id` in slice order after URL-keyed references. Use absolute document IDs for the verified contract. Nested relative IDs and cross-document pointer aliases retain this registration order; URL-keyed references use lexical URL order. See [resource registration](VALIDATION_GRAPHS.md) for remaining identity/error boundaries.
- `CompileOptions.Regex` configures optional match timeout and backtracking stack limits for the default compiler. Matching resource failures return `RegexError` rather than invalid-payload diagnostics. See [regex behavior and limits](VALIDATION_REGEX.md).
- `Options.QueryOptions` configures the existing JMESPath module. The default matches the reference's standard query functions. Pass `jmespath.WithPowertoolsFunctions()` explicitly to enable extended decoding functions.
- `Options.Compiler` implements `Compile(context.Context, any, CompileOptions) (Validator, error)`. A validator returns validation issues separately from operational errors. A nil issue slice means success; a non-nil slice, including an empty one, means rejection. An injected compiler owns its schema dialect, mutation behavior, concurrency and extension semantics.

The default backend is the pure-Go [jsonschema v6.0.3 engine](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6@v6.0.3). Original documents receive Draft 7 structural validation; a private shared assertion core and compiler vocabulary control Draft 7 conditional reachability. The adapter rejects unknown keywords/formats, widens nullable types without widening enum/const, evaluates reference siblings and splits constraints that the engine would otherwise short-circuit. Source locations are retained for diagnostics. Root Commons and Parser gain no third-party dependencies from this module. See [condition graphs](VALIDATION_GRAPHS.md) for the engine configuration and dialect boundaries.

## Errors and current boundaries

`SchemaCompilationError` has the reference message `Failed to compile schema` and unwraps its compilation cause. `SchemaValidationError` carries `Issues` with instance path, schema path, keyword, message, params and an optional property name. An empty property name remains distinguishable from an absent field. Wrapper messages distinguish `Inbound schema validation failed` and `Outbound schema validation failed`. Query and operational errors remain distinct for standalone calls; wrappers retain operational errors as their cause.

The initial reference fixture records complete ordered diagnostics for 502 cases; the Go comparison does not sort away mismatches. Covered cases include scalar/object/array constraints, multiple simultaneous failures, nullable/enum behavior, logical combinations, reference siblings, invalid schemas, custom string/numeric formats, external references and extraction. Its generator uses the actual Powertools utility and an AJV instance with the same `allErrors: true` setting; only AJV's warning logger is disabled during fixture generation. An additional 6,085 regex cases use the default reference utility and cover JavaScript Unicode patterns and all 1,683 supported Unicode property aliases, bringing the total to 6,587.

An additional 1,207 applicator cases bring the Validation corpus to 7,794. Conditionals, dependencies, property names, tuple limits, nested diagnostic ordering and `oneOf` branch retention now have scoped reference coverage. See [applicator diagnostics and object-order semantics](VALIDATION_APPLICATORS.md). The fixture reader compares every reference field, including fields absent from the Go issue type, without sorting expected errors.

Another 1,271 cases cover mixed-type rule groups, nullable arrays, numeric/string formats, nested `$id` resources in `$defs`, relative/absolute references, chains, external schemas and recursion. The combined corpus contains 9,065 cases. Displayed reference paths are scoped separately from canonical engine locations; see [type groups and references](VALIDATION_REFERENCES.md).

An additional 3,047 strict-pattern cases bring the combined corpus to 12,112. Strict property/pattern overlap uses legacy UTF-16 semantics while payload matching retains Unicode behavior. See [strict-schema behavior and remaining setup differences](VALIDATION_STRICT.md).

The schema-setup corpus adds 658 cases, for 12,770 total. Original document structure is checked independently of reachable compilation. Unknown keywords, unknown formats, nullable restrictions and regex syntax in unused definitions do not fail compilation unless reached, while Draft 7 structural errors remain errors. Definitions reached through pointers, aliases or nested IDs use the same checks. Pattern matchers are skipped only when their assertions are always valid and additional-property rules do not require them. See [VALIDATION_STRICT.md](VALIDATION_STRICT.md) for the exact scope and remaining differences.

Another 3,433 keyword cases bring the total to 16,203. Referenced `$defs` fragments now preserve empty combinator behavior, fractional/negative size limits, non-string required diagnostics and default floating-point multipleOf semantics. The latter intentionally follows AJV's `parseInt` comparison, including `0.3` failing `multipleOf: 0.1` and exponential-notation quotients failing the comparison. See [VALIDATION_KEYWORDS.md](VALIDATION_KEYWORDS.md) for examples and remaining numeric boundaries.

The 5,764-case schema-shape corpus brought the total to 21,967. Reachable keyword-type checks remain distinct from original metaschema structure. Scoped primitive fragments, annotation-only schemas, conditional compilation/ignored conditions and mixed property dependencies have reference coverage. Private compilation edges validate unselected constant branches without adding runtime callback calls. See [VALIDATION_SHAPES.md](VALIDATION_SHAPES.md); its remaining ignored-reference resolution cases are covered by the subsequent graph milestone below.

The 1,104-case graph corpus brings the total to 23,071. Ignored conditions do not resolve inactive references, active references retain their original conditional targets, and nested dialect declarations remain annotations. Global nested-resource indexing preserves relative scopes and original diagnostics. Ordered `ExternalSchemas` registrations support the verified cross-document alias replacement behavior; `ExternalRefs` uses lexical URL order. See [VALIDATION_GRAPHS.md](VALIDATION_GRAPHS.md) for scope and remaining identity/setup errors.

Remaining compatibility work is explicit:

- JavaScript Unicode regex adaptation now supports lookarounds, backreferences, strict lexical forms and pinned property tables. Exhaustive regex/capture/quantifier and malformed-string parity remains open; see [VALIDATION_REGEX.md](VALIDATION_REGEX.md).
- Mixed-type grouping, nested IDs and recursive/reference paths have scoped coverage; exhaustive combinations, anchors, duplicate-ID/setup behavior and special JavaScript object names still need further coverage. Go maps use deterministic JSON encoding order; use raw JSON to retain source declaration order. Strict property/pattern overlap now uses a separate legacy matcher; exhaustive legacy capture/native-string behavior remains open.
- Exhaustive AJV strict-mode diagnostics, schema setup errors, arbitrary fragment/conditional reachability, dialect registration, format object/regular-expression forms, custom keyword plugins and injected-instance options require a complete export/options audit. Go wraps invalid external registration in SchemaCompilationError; the reference can throw an ordinary Error before its compilation catch. Referenced `$defs` constraints not covered by Draft 7 structural validation still require numeric/keyword edge coverage.
- JSON numbers retain Go precision. JavaScript floating-point arithmetic, multiple-of rounding, malformed Unicode and non-JSON native values require explicit cross-language boundary tests.
- Input snapshots are JSON snapshots, not arbitrary JavaScript `structuredClone` or a reflection-based Go deep clone. Functions, channels and cyclic values fail before the handler. Standalone validation never mutates the caller's payload by default.
- Context cancellation is checked around compilation/validation, but synchronous engine traversal cannot be interrupted midway. Performance and pathological-schema resource limits are not yet measured.
- Parser/Event Handler composition, complete middleware/decorator mapping, public release and live AWS acceptance remain separate gates.

## Public contract map

| TypeScript public entry | Go equivalent | Remaining boundary |
| --- | --- | --- |
| Root `validate` | `Validate`, reusable `Compile` and `Schema.Validate` | Complete AJV keyword and diagnostic parity |
| `middleware.validator` | `WrapHandler` | JSON snapshot instead of arbitrary structured clone; compilation usually occurs before invocation; clone failures currently receive an inbound stage error |
| `decorator.validator` | Typed handler wrapping | An explicit un-cloned decorator-equivalent path remains open |
| `errors.SchemaCompilationError` | `SchemaCompilationError` with `Unwrap` | Complete native setup-cause and timing mapping |
| `errors.SchemaValidationError` | `SchemaValidationError` with structured `Issues` and operational `Unwrap` | Complete AJV cause payload and keyword metadata |
| `payload`, `schema`, `envelope` | Function arguments and `Options.Envelope` | Runtime-specific native values and malformed queries |
| `formats` | `Formats`, `NumberFormats` | Regex/object/async format forms and custom keyword behavior |
| `externalRefs` | URL-keyed `ExternalRefs`, ordered `ExternalSchemas` | Complete duplicate-ID/relative-document-ID/setup errors |
| `ajv` | `Compiler` and `Validator` interfaces | Explicit adapters for AJV option families rather than a claim of engine equivalence |

This map was inspected against the installed package's export manifest, declarations and implementations. The package exposes four public subpaths: root, middleware, decorator and errors. Its `types.d.ts` is referenced by the declarations but is not a separately exported package subpath.

CGO is disabled for all development, tests and builds. Functional concurrency tests do not replace a race-detector run; the latter is intentionally excluded by repository policy.
