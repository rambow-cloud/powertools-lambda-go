---
description: "Handle recursive validation errors and compose Powertools Go Parser operations with explicit paths, results and error contracts."
---

# Parser recursive errors and composition

Reference: Powertools TypeScript v2.35.0 and Zod v4.1.12. This milestone replaces flattened union diagnostics with recursive branch errors and aligns branch selection, continued refinements and array-check ordering. It also propagates safe parsing through nested Go schema composition. Constraint-specific error metadata and complete cross-language compatibility remain separate gates.

## Union selection and errors

`Union` tries validly configured branches in declaration order and returns the first success. If every branch fails, exactly one branch with only continuable check failures retains its own diagnostics. Otherwise the result is an `invalid_union` issue with an `Errors` entry for every branch, recursively including nested unions. A single-branch union delegates directly to that branch.

For example, `Union(positiveNumber, String())` given a negative number reports the positivity check. Given a boolean, it reports an `invalid_union` with separate number/string type errors. Two string branches that both fail refinements remain ambiguous and produce a union error. An operational error or panic stops evaluation immediately; later branches do not run. Cancellation is checked between branches.

`Issue.Errors` serializes as `errors`, an array of branch issue arrays. Paths on the outer union identify its location in the containing event. Paths inside a branch remain relative to that branch; prefixing the outer issue does not prepend the parent path to its children. `Prefix` recursively copies branch arrays and paths, so extending or inspecting one invocation's errors cannot mutate another invocation's shared diagnostics.

The portable error representation now preserves code, message, path, expected type and recursive branch errors. Fields such as Zod's format regex, origin, literal values, minimum/maximum and other constraint-specific metadata are not yet exposed. The TypeScript generators share `parser-issues.mjs` to retain exactly the same recursive projection across every corpus. JSON parser diagnostic suffixes are normalized at every tree depth; no union branches are dropped or sorted to hide differences.

## Continued checks

`Issue.Continuable` is evaluation state and is omitted from serialized errors. Built-in format/range checks and `Refine` set it when the parsed value's type remains valid. Type, literal, strict-object and ambiguous-union failures stop later refinements. Consecutive refinements run after earlier continuable failures and preserve their order. `Transform` and `Pipe` still stop on any upstream validation failure, as the reference pipelines do.

Custom `SchemaFunc` implementations default to non-continuable failures. Set `Continuable: true` only when returning a value of the valid parsed type, allowing later refinements to inspect that value. A non-nil empty issue slice remains a validation failure and stops later checks. `Any` preserves a typed schema's failed output value for branch/refinement evaluation; public `Parse`/`SafeParse` still discard data on overall validation failure.

Array length issues follow element issues. The pinned Zod length check also runs on wrong-type strings and JSON objects with a `length` property. The implementation preserves those diagnostics, counts string length in UTF-16 code units and reuses Commons numeric-string conversion for JSON length values. These values still fail the array type check. No coercion turns them into valid arrays.

The generic union implementation now handles S3's IPv4/service-hostname diagnostic rule; the former S3-specific branch-selection workaround has been removed.

## Nested safe parsing

Object fields, array elements, dictionary values, unions, nullable schemas and JSON/Base64 helpers preserve `SafeSchema` dispatch. A schema remains reusable in either mode. JSON/Base64 decoding uses the existing pipeline primitive, keeping decoding separate from payload validation.

A nested `ParseError` represents validation failure, allowing parent containers to prefix paths, collect sibling issues and let unions try other branches. Its original wrapper message and exception identity are replaced by the containing schema's aggregate diagnostics. Since an envelope `ParseError` has no validated output, later refinements are skipped. Direct top-level `ParseError` handling retains the original error object. Other operational errors and panic identities continue to propagate unchanged.

For example, safe parsing of an object containing a JSON-stringified, nullable SQS envelope can collect invalid payloads from several records plus an invalid sibling field. Ordinary parsing keeps the first invalid SQS payload while still validating the parent object's sibling fields. Both modes retain the original input on validation failure and return no partial data.

## Evidence and remaining work

`generate-parser-unions.mjs` executes sixteen schema combinations against 36 JSON inputs in ordinary and safe modes: 1,152 cases. They cover primitives, sole/ambiguous/single branches, strict objects, nested errors, arrays, dictionaries, JSON failures, transformations, chained refinements, nullable values and length checks. A UTF-16-equivalent predicate is used where the reference application callback reads JavaScript string length. The existing 2,341 cases now compare recursive trees too, bringing the total to 3,493.

Additional Go tests cover safe-mode propagation through every composition layer, nested `ParseError` fallback/aggregation, empty issue slices, constructor slice snapshots, cancellation between branches, callback errors/panics, nil schemas and concurrent error-tree ownership.

Acceptance on 2026-09-16 (Asia/Shanghai) passed all 18 independently packaged modules and 15 standalone consumers, both CGO-disabled Linux builds, 292/292 Docker assertions and 14/14 Batch checks over the same artifacts. Eighteen added runtime assertions verify a sole branch's diagnostic, recursive relative paths, continued refinements, array error order and nested JSON/nullable/SQS aggregation in both modes. Docker executed amd64; arm64 was cross-compiled only. Containers and network were cleaned without AWS access. The machine-readable report uses the artifact's UTC date. See [PARSER_PLAN.md](PARSER_PLAN.md) and [LOCAL_VALIDATION.md](LOCAL_VALIDATION.md).

This does not close every error/primitive gate. Constraint metadata, JavaScript-specific values and coercion/prototype behavior, complete inferred-type mappings, dictionary insertion order, malformed Unicode/encoding, extreme numbers, all custom-schema interactions and performance remain open. General JSON Schema Validation and Event Handler integration are also still pending.
