# Data Masking

The independent datamasking module depends only on root Commons. It provides selected-field erasure and orchestration around an application-supplied encryption provider. ECMAScript string replacement is available through the optional commons/regex module. An optional uncached AWS Encryption SDK provider is implemented in datamasking/kms; see [its contract and platform requirements](DATAMASKING_KMS.md). Data-key cache parity remains open. See DATAMASKING_PLAN.md for the complete remaining scope.

```go
masker := datamasking.New(datamasking.Config{})
masked, err := masker.Erase(ctx, payload, datamasking.EraseOptions{
    Fields: []string{"customers[*].ssn", "payment.card"},
})
```

The default mask is *****. Missing selected fields return an Error whose ErrorName is DataMaskingFieldNotFoundError. Config.IgnoreMissing converts these failures to Warn callbacks (stderr by default). Warnings and supplied callbacks must support concurrent callers. Erase does not mutate input. Default whole-payload erasure does not inspect contents, including opaque or cyclic objects; selected/rule operations normalize JSON-shaped inputs into an isolated private tree.

## Rules and selectors

An omitted Fields slice differs from an explicit empty slice. With no fields/rules/strategy, non-array data collapses to the default mask and each array element becomes a mask. Null and Undefined remain unchanged. A top-level Rule without selectors visits all leaves. CustomMask is a *string so an empty mask is representable. DynamicMask is a *bool so explicit false retains upstream rule presence. Dynamic masks count UTF-16 units after string conversion, including two units for an astral character.

EraseOptions.Rules is an ordered []FieldRule. Rules run first; ordinary Fields skip matching concrete paths. Later rules can observe earlier changes. Rule.Replace propagates replacement errors. Bind a compiled commons/regex expression with Regexp.Replacer(format) to use the built-in ECMAScript adapter, or supply an application callback. See [REGEX.md](REGEX.md) for flags, shared lastIndex, UTF-16 handling and remaining boundaries. Precedence is Replace, CustomMask, DynamicMask, then the default mask.

Paths support dot properties, numeric dot indices, * and [*]. They are not JMESPath or JSONPath: users[0].secret treats users[0] as a literal property, while users.0.secret selects an array item. Empty path segments are removed. Wildcards exclude __proto__, constructor and prototype object keys; explicit reserved-path behavior follows the covered source cases. An empty expression resolves the root but assignment to an empty path is a no-op. Missing per-field rules are silent.

## Encryption provider boundary

Implement Provider.Encrypt/Decrypt with context.Context, a string and a map[string]string authenticated context. Full encryption serializes data to JSON; full decryption parses returned JSON. Selected fields use the same boundary. A string input to Decrypt always uses the whole-payload path. Missing selected fields are ignored; non-string selected decrypt values warn and remain unchanged. Missing providers report DataMaskingEncryptionError. Provider errors retain identity and provider panics are rethrown on the caller when observed.

All selected values are captured before provider calls start. Operations run concurrently; the caller owns result writes and returns on the first observed failure, matching Promise.all rejection. Already-started sibling providers are not automatically canceled and can finish after the error returns. Providers must honor cancellation. Authenticated-context maps are copied per call. This is a deliberate Go ownership/lifecycle mapping: exact JavaScript invocation scheduling, mutation of shared contexts and overlapping ancestor/descendant writes are not fully equivalent. No unused providerOptions escape hatch is exposed because the pinned runtime ignores it.

## JSON and native boundaries

Use json.RawMessage when exact source object-key order matters for encrypted plaintext. Commons.SortObjectKeys preserves numeric-key enumeration; native Go maps use deterministic sorted keys. Commons.ParseNumber supplies reference numeric conversion. JSON-tagged native structs are accepted, with encoding/json semantics. Go binary values use their native JSON representation. Fields/rules and callback implementations remain application-owned and must not be mutated during a call.

Selected/rule operations accept acyclic JSON-shaped data. They do not reproduce structuredClone's Dates, Maps, Sets, cyclic graphs or prototype handling. Native functions/channels/cycles that need cloning return DataMaskingUnsupportedTypeError. Undefined cannot be passed as a whole plaintext to a string-only Go provider. Array length changes/holes, lone UTF-16 surrogates, U+2028/U+2029 serialization and exact JSON parser failures remain open. Returned Error names are explicit library metadata, not overrides of native Lambda Runtime API error type names.

The current 240-case corpus checks erasure, provider orchestration, plaintext ordering, errors and diagnostics. Its deterministic provider and local Lambda fixture are explicitly not cryptography. They establish neither confidentiality nor AWS ciphertext/cache compatibility. See [the reference scope](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/datamasking/testdata/README.md).

Current verified acceptance: 30 packaged modules/27 standalone consumers across the initial checkpoint and six-module continuation, both CGO-disabled Linux builds, 856/856 RIE assertions, 95/95 streaming and 14/14 Batch checks. Regex adds 7,671 Node replacement cases, 19 invalid patterns and 30 actual Data Masking compositions. Docker executed amd64; arm64 was cross-compiled. See LOCAL_VALIDATION.md and MODULE_ACCEPTANCE_REGEX.json for exact scopes and the Validation extraction fix.
