---
description: "Erase or transform selected Go Lambda payload fields with Powertools Data Masking, ordered rules and optional regex or KMS providers."
---

# Data Masking

Data Masking erases selected fields or transforms them through an encryption provider. Import `github.com/rambow-cloud/powertools-lambda-go/datamasking`. Erasure is built in; regex and KMS encryption are separate optional modules.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Complete example

This complete offline program masks a customer SSN without changing the input object. Save it in an empty directory inside the checkout and run `go run main.go` with `CGO_ENABLED=0`. It requires no provider or AWS credentials.

~~~go
package main

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	stdlog "log"

	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
)

func main() {
	payload := map[string]any{
		"customer": map[string]any{"name": "Ada", "ssn": "123-45-6789"},
		"order_id": "ORD-123",
	}
	masker := datamasking.New(datamasking.Config{})
	masked, err := masker.Erase(context.Background(), payload, datamasking.EraseOptions{
		Fields: []string{"customer.ssn"},
	})
	if err != nil {
		stdlog.Fatal(err)
	}
	encoded, err := json.Marshal(masked)
	if err != nil {
		stdlog.Fatal(err)
	}
	fmt.Println(string(encoded))
}
~~~

## Input and output

Stdout contains the following JSON. Only `customer.ssn` changes; the original `payload` remains unchanged. The program prints only the masked copy. Missing selected fields return an error by default; `IgnoreMissing` warns and continues. Erasure is irreversible and does not need KMS. Encryption produces ciphertext and requires a compatible provider; do not treat a mask string as encrypted data.

~~~json
{
  "customer": {
    "name": "Ada",
    "ssn": "*****"
  },
  "order_id": "ORD-123"
}
~~~

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `masker` | Reusable masking policy and optional provider. |
| `EraseOptions` | Select fields, ordered rules and custom/dynamic masks for one operation. |
| `masked` | Returned private result; print/store this value instead of the original payload. |
| `Provider` | Context-aware Encrypt/Decrypt interface; optional real KMS provider is uncached. |

## TypeScript feature coverage

Compared with the [official v2.35.0 data-masking guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/data-masking.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| Erasure / field selection | `Erase`, `Fields`, `Rules` | Whole payload, dot paths, wildcards, custom/dynamic masks. |
| Regex replacement | Optional `commons/regex` replacer | Shared ECMAScript adapter; complete regex boundaries remain open. |
| Encrypt / decrypt | `Provider`, `Encrypt`, `Decrypt` | Caller-supplied provider; field-level and whole-payload paths. |
| Encryption context / multiple keys | `TransformOptions.Context`, KMS provider `Keys` | Real SDK interoperability checked with synthetic local key wrapping. |
| Provider / data-key caching | `datamasking/kms` | Uncached provider implemented; TypeScript data-key caching remains unsupported. |

Executable evidence: [datamasking/masking_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/datamasking/masking_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.

## Rules and selectors

An omitted Fields slice differs from an explicit empty slice. With no fields/rules/strategy, non-array data collapses to the default mask and each array element becomes a mask. Null and Undefined remain unchanged. A top-level Rule without selectors visits all leaves. CustomMask is a *string so an empty mask is representable. DynamicMask is a *bool so explicit false retains upstream rule presence. Dynamic masks count UTF-16 units after string conversion, including two units for an astral character.

EraseOptions.Rules is an ordered []FieldRule. Rules run first; ordinary Fields skip matching concrete paths. Later rules can observe earlier changes. Rule.Replace propagates replacement errors. Bind a compiled commons/regex expression with Regexp.Replacer(format) to use the built-in ECMAScript adapter, or supply an application callback. See [REGEX.md](REGEX.md) for flags, shared lastIndex, UTF-16 handling and remaining boundaries. Precedence is Replace, CustomMask, DynamicMask, then the default mask.

Both Rules and Fields return DataMaskingFieldNotFoundError when a selector matches no field, including unmatched wildcards; IgnoreMissing emits one warning per missing selector and continues. This corrects the pinned TypeScript missing-rule behavior.

Paths support dot properties, numeric dot indices, * and [*]. They are not JMESPath or JSONPath: users[0].secret treats users[0] as a literal property, while users.0.secret selects an array item. Empty path segments are removed. Wildcards exclude __proto__, constructor and prototype object keys; explicit reserved-path behavior follows the covered source cases. An empty expression resolves the root but assignment to an empty path is a no-op.

## Encryption provider boundary

Implement Provider.Encrypt/Decrypt with context.Context, a string and a map[string]string authenticated context. Full encryption serializes data to JSON; full decryption parses returned JSON. Selected fields use the same boundary. A string input to Decrypt always uses the whole-payload path. Missing selected fields are ignored; non-string selected decrypt values warn and remain unchanged. Missing providers report DataMaskingEncryptionError. Provider errors retain identity and provider panics are rethrown on the caller when observed.

All selected values are captured before provider calls start. Operations run concurrently; the caller owns result writes and returns on the first observed failure, matching Promise.all rejection. Already-started sibling providers are not automatically canceled and can finish after the error returns. Providers must honor cancellation. Authenticated-context maps are copied per call. This is a deliberate Go ownership/lifecycle mapping: exact JavaScript invocation scheduling, mutation of shared contexts and overlapping ancestor/descendant writes are not fully equivalent. No unused providerOptions escape hatch is exposed because the pinned runtime ignores it.

## JSON and native boundaries

Use `jsontext.Value` or `encoding/json.RawMessage` when exact source object-key order matters for encrypted plaintext. Commons.SortObjectKeys preserves numeric-key enumeration; native Go maps use deterministic sorted keys. Commons.ParseNumber supplies reference numeric conversion. JSON-tagged native structs use `encoding/json/v2` semantics. Duplicate object members, invalid UTF-8 and invalid escaped Unicode are rejected. Go binary values use their native JSON representation. Fields/rules and callback implementations remain application-owned and must not be mutated during a call.

Selected/rule operations accept acyclic JSON-shaped data. They do not reproduce structuredClone's Dates, Maps, Sets, cyclic graphs or prototype handling. Native functions/channels/cycles that need cloning return DataMaskingUnsupportedTypeError. Undefined cannot be passed as a whole plaintext to a string-only Go provider. Array length changes/holes, lone UTF-16 surrogates, U+2028/U+2029 serialization and exact JSON parser failures remain open. Returned Error names are explicit library metadata, not overrides of native Lambda Runtime API error type names.

The current 240-case corpus checks erasure, provider orchestration, plaintext ordering, errors and diagnostics. Its deterministic provider and local Lambda fixture are explicitly not cryptography. They establish neither confidentiality nor AWS ciphertext/cache compatibility. See [the reference scope](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/datamasking/testdata/README.md).

Current verified acceptance: 30 packaged modules/27 standalone consumers across the initial checkpoint and six-module continuation, both CGO-disabled Linux builds, 856/856 RIE assertions, 95/95 streaming and 14/14 Batch checks. Regex adds 7,671 Node replacement cases, 19 invalid patterns and 30 actual Data Masking compositions. Docker executed amd64; arm64 was cross-compiled. See LOCAL_VALIDATION.md and MODULE_ACCEPTANCE_REGEX.json for exact scopes and the Validation extraction fix.
