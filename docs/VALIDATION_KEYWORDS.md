---
description: "Review Powertools Go JSON Schema keyword validation, numeric compatibility, supported behavior and diagnostic contracts."
---

# Validation keyword and numeric compatibility

Reference: Powertools TypeScript v2.35.0 with AJV v8.20.0. These rules extend the schema-setup work in [VALIDATION_STRICT.md](VALIDATION_STRICT.md); they do not establish complete AJV compatibility.

## Structural validation and referenced definitions

Draft 7 metaschema validation covers `definitions` but does not traverse `$defs` in the reference's default configuration. Constraints that are rejected at the root or inside `definitions` can therefore compile when reached through `$defs`. The adapter preserves original-document structure checks and implements the resulting keyword behavior, rather than imposing an additional metaschema pass on every referenced fragment.

- Empty `allOf` accepts a value; empty `anyOf` and `oneOf` reject it with their normal summary diagnostics. Empty `oneOf` retains `passingSchemas: null`.
- Empty `enum` remains a compilation error even inside a referenced `$defs` fragment.
- Empty `type` arrays accept values where the original document permits them. Duplicate type names remain visible in type diagnostics.
- Numeric length limits apply to strings, arrays and objects of the matching type. Fractional limits retain their original value in messages and `params.limit`; they are not rounded to integers. Negative limits and large limits follow the same numeric comparison.
- Non-string `required` entries use JavaScript property-name conversion while preserving the reference's diagnostic value. The default 200-entry loop threshold is covered: short lists interpolate array names as joined strings, while long lists retain the array in `missingProperty`. Duplicate entries retain duplicate errors.

For example, a referenced `$defs` fragment containing `minLength: 1.5` rejects a one-character string with `limit: 1.5`. The same original fragment under `definitions` fails schema compilation. An empty `anyOf` in `$defs` produces a validation error, rather than a compilation error or unconditional success.

## Floating-point multipleOf

AJV's default `multipleOf` implementation divides JavaScript numbers and compares the quotient to `parseInt(quotient)`. Exact rational divisibility produces different answers, so the Go adapter uses the same floating-point decision:

- A zero divisor rejects numeric values and does not cause a division panic.
- `0.3` fails `multipleOf: 0.1` because the floating quotient differs from its parsed integer.
- Integer quotients whose magnitude reaches `1e21` stringify in exponential notation; `parseInt` reads the initial coefficient, and the comparison fails. Thus `1e21` fails the reference's default `multipleOf: 1` check.
- Negative divisors in unvalidated `$defs` fragments and tiny/large finite divisors follow the same rule.
- Diagnostic number text uses JavaScript-compatible finite-number formatting. Non-number payloads do not execute the numeric rule.

These behaviors reproduce the pinned reference, including its surprising cases. There is no JavaScript runtime or additional dependency in the Lambda binary.

## Implementation and evidence

Private immutable keyword adapters are installed on the compiled schema graph before publication. Original-source lookup excludes generated type/enum/format aliases, preventing duplicate sibling rules. Size rules share one length calculation for the applicable value type. Required entries preserve declaration order, and diagnostic parameters use Commons snapshots so callers cannot mutate compiled state through returned errors.

`tools/reference/generate-validation-keywords.mjs` records 3,433 actual reference cases. It compares root, `definitions`, `$defs` and alias schemas across size limits, empty rules, required values and numeric boundaries, with complete diagnostic fields and ordering. Combined Validation coverage is 16,203 cases. A separate 32-caller test mutates returned object-valued `missingProperty` parameters and verifies subsequent errors remain unchanged. The previous 55 reference-only observations motivated this corpus and are not counted separately.

Run the generator from `tools/reference` with `node generate-validation-keywords.mjs`. Run focused tests with `CGO_ENABLED=0` and `go test ./validation/...`; packaged-module, cross-build and Docker acceptance uses `uv run python integration/local/run.py`.

Acceptance (2026-09-16, Asia/Shanghai): all 19 independently packaged modules passed tests/vet/tidy checks, sixteen consumers built, and both Linux architectures compiled with CGO disabled. Docker passed 403/403 assertions across five invocations and cleaned all temporary containers/network. The same saved artifacts passed 14/14 Batch checks without extra invocations. Module checks and builds ran in the same complete acceptance command. Docker executed amd64; arm64 was cross-compiled only. No AWS resources were used.

## Remaining boundaries

The complete V-05/V-06/V-07 gates remain open. Remaining work includes other malformed keyword shapes, JavaScript prototype/inherited-property behavior, object-to-primitive failures, non-finite/unsafe-number representation, all numeric comparison/equality/type rules, custom AJV precision/options, exhaustive reference graphs and errors outside the reference's compilation catch. Root/`definitions` structural checks remain mandatory. This implementation does not claim that every schema ignored by the Draft 7 metaschema is compatible.

### Source-shape audit

A read-only reference probe compiled 21 individual keywords through `#/$defs/value`, each with six values, and validated `{}`. These 126 observations motivated the implemented [schema-shape adapter and 5,764-case corpus](VALIDATION_SHAPES.md); they are not counted separately. `C` means compilation error, `V` means validation error, and `P` means success for that payload.

| Keyword | null | true | 1 | `"x"` | `[]` | `{}` |
| --- | --- | --- | --- | --- | --- | --- |
| minLength, maximum, multipleOf | C | C | P | C | C | C |
| pattern | C | C | C | P | C | C |
| format | C | C | C | C | C | C |
| required | C | C | C | C | P | C |
| uniqueItems | C | P | C | C | C | C |
| properties, patternProperties, dependencies | C | C | C | C | C | P |
| items | C | P | C | C | P | P |
| additionalItems | C | C | C | C | C | C |
| additionalProperties, contains, propertyNames | C | P | C | C | C | P |
| not | C | V | C | C | C | V |
| if | C | C | C | C | C | C |
| allOf | C | C | C | C | P | C |
| anyOf, oneOf | C | C | C | C | V | C |
| enum | C | C | C | C | C | C |

The format string `"x"` is unknown; `additionalItems` has no tuple `items`, `if` has no then/else branch, and the tested enum array is empty. Those contextual failures must not be mistaken for simple type restrictions. Deferred keyword-shape checks and the expanded contextual corpus now preserve these compilation-versus-validation distinctions; exhaustive graph/shape behavior remains open.
