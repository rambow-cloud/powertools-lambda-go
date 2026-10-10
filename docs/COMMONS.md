---
description: "Reuse dependency-free Powertools Go runtime primitives and one shared Lambda invocation identity across independent utilities."
---

# Commons and Metadata

The foundation is implemented against the installed TypeScript v2.35.0 distribution. Pure helpers live in `commons`; AWS-specific helpers live in `commons/awssdk` and `commons/dynamodb`; HTTP metadata retrieval lives in `commons/metadata`. Existing invocation lifecycle ownership remains in `internal/invocation`. No package performs network requests or modifies AWS environment variables merely because it is imported.

## Complete example

This complete offline program uses the shared Base64 decoder. Save it in an empty directory inside the checkout and run `go run main.go` with `CGO_ENABLED=0`.

~~~go
package main

import (
	"fmt"
	stdlog "log"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

func main() {
	value, err := commons.FromBase64("SGVsbG8=", "base64")
	if err != nil {
		stdlog.Fatal(err)
	}
	fmt.Println(string(value))
}
~~~

## Input and output

Stdout is `Hello` followed by a newline. `FromBase64` returns bytes; select `"base64"` explicitly to decode them, then convert to a string for text output. Its default `"utf8"` mode returns the validated input's UTF-8 bytes, retaining the pinned TypeScript helper's behavior despite its name. Invalid standard-alphabet/padding input returns an error. This is ordinary output from `fmt.Println`, not a structured log. No utility is initialized merely by importing Commons.

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `commons` | Stateless helpers plus explicitly created Utility/LRU objects; root module has no third-party dependencies |
| Invocation identity | Shared private runtime identity reused by handler wrappers; no separate cold-start state per wrapper |
| Optional `commons/*` modules | SDK, DynamoDB, Metadata and regex dependencies installed only when used |

## TypeScript feature coverage

The [public contract map](#public-contract-map) below comes from the pinned [Commons exports](https://github.com/aws-powertools/powertools-lambda-typescript/tree/7bcc27b1574493f9452688673658f52b80c53847/packages/commons/src). See the separate [Metadata guide](METADATA.md) for the user-facing LMDS feature.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| Environment / runtime helpers | StringEnv, NumberEnv, BoolEnv, Utility and context helpers | Strict/extended modes and shared invocation identity |
| Encoding / merge / LRU | FromBase64, DeepMerge, NewLRUCache | Reference indexed merging; Go snapshots and typed generics |
| DynamoDB conversion | SDK-free raw conversion and optional SDK module | Number-preserving native values; Sets become slices |
| SDK identity | Optional API middleware | One marker, no global AWS_SDK_UA_APP_ID mutation |
| Regex / metadata | Optional modules and explicit clients | Documented extensions and ownership differences |

[Shared tests](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/commons/commons_test.go) and independent adapter tests provide scoped evidence. [Reuse audit](COMMONS_REUSE.md) explains which utilities consume these primitives. See [feature comparison](FEATURE_PARITY.md) for remaining native/type/service gates.

## Public contract map

| Reference export | Go equivalent |
| --- | --- |
| `getStringFromEnv`, `getNumberFromEnv`, `getBooleanFromEnv` | `commons.StringEnv`, `NumberEnv`, `BoolEnv` |
| `isDevMode`, `isRunningInLambda`, `getServiceName` | `commons.IsDevMode`, `IsRunningInLambda`, `ServiceName` |
| `getXrayTraceDataFromEnv`, `getXRayTraceIdFromEnv`, `isRequestXRaySampled` | `commons.XRayTraceData`, `XRayTraceID`, `IsRequestXRaySampled`, all accepting context |
| `shouldUseInvokeStore` | `commons.ShouldUseInvokeStore`; Go wrappers always prefer context-owned state |
| `Utility` | `commons.NewUtility`, `GetInitializationType`, `GetColdStart`; standalone `IsValidServiceName` |
| `fromBase64` | `commons.FromBase64` |
| `deepMerge` | `commons.DeepMerge` |
| `LRUCache` | `commons.NewLRUCache[K,V]`, `Add`, `Get`, `Has`, `Remove`, `Size`; Go adds `Clear` |
| `unmarshallDynamoDB` | SDK-free `commons.UnmarshallDynamoDB`; `commons/dynamodb.UnmarshallDynamoDB` delegates for existing consumers |
| SDK attribute decoding | `commons/dynamodb.UnmarshalAttribute`, `UnmarshalItem` for Go SDK values |
| `UnmarshallDynamoDBAttributeError` | `*commons.DynamoDBAttributeError`; `commons/dynamodb.AttributeError` is a type alias |
| Type predicates and `getType`, `isStrictEqual` | Exported `commons.IsRecord`, `IsString`, `IsNumber`, `IsIntegerNumber`, `IsNull`, `IsNullOrUndefined`, `IsStringUndefinedNullEmpty`, `IsRegExp`, `IsTruthy`, `GetType`, `IsStrictEqual` |
| `addUserAgentMiddleware` | `commons/awssdk.UserAgent(feature)` as an SDK API stack option |
| `isSdkClient` | Service-specific Go interfaces checked by the compiler, rather than JavaScript object inspection |
| `PT_VERSION` | `commons.Version` for this Go implementation; `ReferenceVersion` identifies the upstream baseline |
| `getMetadata`, `clearMetadataCache` | `metadata.GetMetadata`, `ClearMetadataCache`; explicit clients also support `Get` and `ClearCache` |
| Middy cleanup, utility keys, Lambda/type interfaces | Existing Go handler wrappers, private typed context keys, and native function/interface contracts; no Middy dependency or JavaScript symbol shim |

## Configuration and runtime helpers

```go
enabled, err := commons.BoolEnv("FEATURE_ENABLED", true, false)
limit, err := commons.NumberEnv("FEATURE_LIMIT", 100)
name, err := commons.StringEnv("FEATURE_NAME", "default")
```

The optional final argument is a default for an absent variable. A present empty string is not absence. String values are trimmed; booleans accept case-insensitive true/false. Extended boolean parsing also accepts `1/y/yes/t/on` and `0/n/no/f/off`. Invalid values return `*commons.EnvironmentError`. Number parsing supports empty-to-zero, decimal/exponent strings, binary/octal/hexadecimal prefixes, and infinity according to the reference conversion. Invalid numeric strings produce a typed error.

`BoolEnvOr` explicitly chooses fallback-on-error for constructors retaining that Go API policy. `IsDevMode` also catches invalid input and returns false, matching upstream. `ServiceName` returns a trimmed environment value or an empty string. `ResolveServiceName` lets the consumer choose a fallback: Logger/Metrics/Tracer use `service_undefined`, while AppConfig still requires an application.

`IsRunningInLambda` follows the pinned initialization-type/development-mode rule, including its unusual treatment of a present empty initialization-type value as running in Lambda. `InitializationType` and `Utility` separately recognize only `on-demand` and `provisioned-concurrency`. A Utility instance reports cold start once for on-demand initialization. Composed wrappers retain shared invocation identity. Metrics additionally reuses Utility for the consumable per-instance manual capture decision and checks the shared invocation before emitting, so a warm bound scope cannot become cold merely by constructing another Metrics instance.

Trace helpers prefer the Lambda runtime value in context, including an explicitly empty header. Process-wide trace fallback is disabled for concurrent invocations. The root-ID-only header form and sampled flag are supported. This is implemented with Go contexts rather than Node's invocation store.

## Encoding, merging, and caches

Raw DynamoDB conversion and numeric parsing live in root Commons. `DynamoDBNumber` returns a safe float64 or an exact large `*big.Int`; the SDK adapter's existing `Number` function delegates to it. Parser schemas reuse the same raw decoder without importing an SDK. Native SDK binary/set conversion stays in `commons/dynamodb`.

`FromBase64` checks padding length and the standard alphabet before conversion. Pass `"base64"` to decode. With no encoding argument, it returns the validated input's UTF-8 bytes, matching the surprising but observable upstream default. Encoding modes cover UTF-8, ASCII/Latin-1, base64/base64url after standard validation, hex, and UTF-16LE aliases. Parameters now uses this helper and rejects whitespace, URL-safe symbols, and incomplete padding covered by the reference fixtures.

`DeepMerge` recursively combines string-keyed objects, merges array elements by index, retains untouched trailing elements, and skips ancestor cycles and `__proto__`/`constructor` keys. It mutates the destination and copies incoming containers. It treats Go nil as null; JavaScript undefined has no distinct Go representation. Logger's merge adapter filters reserved root fields and builds a fresh merged snapshot to preserve parent/child isolation. Serialization callbacks and arbitrary Go values remain Logger concerns.

`LRUCache` uses a map and `container/list` with a mutex. Capacity defaults to 100. Get and overwrite mark an entry recently used; Has does not. Non-positive capacities retain no entries. It has no TTL or background work. Parameters keeps its existing TTL cache because age, transform keys, force fetch, and batching are separate policies.

`CloneValue` shares acyclic decoded-value snapshot logic between Parameters and Metadata. It preserves map keys, byte slices, sets represented by slices, and large integers. It intentionally does not call DeepMerge, whose unsafe-key filtering and indexed arrays would change configuration data. Opaque caller-owned objects are retained rather than reflected into a universal serialization model.

## DynamoDB conversion

The raw helper accepts JSON-shaped DynamoDB attribute objects and supports S, B, BOOL, NULL, N, M, L, SS, NS, and BS. It retains raw B values as supplied. Sets become Go slices. SDK helpers retain the SDK's binary decoding and use its number-preserving decoder before shared numeric conversion.

Numbers within the JavaScript safe-integer magnitude use float64, including ordinary fractions. Larger integer strings become `*big.Int`; large decimal/exponent forms that JavaScript BigInt rejects return an error. This fixes the previous silent precision loss in Parameters. JSON transformation itself still uses normal JSON float64 decoding. Shared snapshots copy big.Int values so a caller cannot modify a cached integer.

## AWS SDK identity

```go
client := ssm.NewFromConfig(cfg, func(o *ssm.Options) {
    o.APIOptions = append(o.APIOptions, awssdk.UserAgent("parameters"))
})
```

Parameters installs the option per operation; Tracer installs it with AWS instrumentation. The stack contains one marker, and the first feature installed wins. Existing SDK user-agent data is retained. Markers use the Go development version, not a claim to be the TypeScript distribution. Unlike the TypeScript root import, Go does not modify `AWS_SDK_UA_APP_ID` globally. No-op markers can be promoted to a feature marker without adding another marker.

## Metadata usage

```go
import (
    "context"
    "time"
    "github.com/rambow-cloud/powertools-lambda-go/commons/metadata"
)

func executionEnvironment(ctx context.Context) (map[string]any, error) {
    return metadata.GetMetadata(ctx, metadata.Options{Timeout: 500 * time.Millisecond})
}
```

The default client reads `AWS_LAMBDA_METADATA_API` and `AWS_LAMBDA_METADATA_TOKEN`, then sends `Authorization: Bearer <token>` to `/2026-01-15/metadata/execution-environment`. The default timeout is one second and is bounded by the caller's context. A nonempty response is cached until cleared; empty results and failures are not cached. Unknown fields are retained. Outside Lambda, the default helper returns an empty map without reading endpoint/token or sending a request.

`metadata.New(metadata.Config{Endpoint: endpoint, Token: token, HTTPClient: client})` creates an explicit independent client. Endpoint explicitly enables local access outside Lambda. An injected instrumented HTTP client supports OTel without importing Tracer into Metadata. The client preserves response snapshots, coalesces concurrent fetches, allows waiting callers to cancel, and prevents pre-clear requests from repopulating the cache. Redirects are rejected to keep the bearer token on the configured endpoint. Response bodies are omitted from status errors. Errors unwrap cancellation/deadline causes.

Metadata is opt-in. Logger, Tracer, and Parameters do not automatically fetch it. The local integration handler explicitly retrieves it to verify composition.

## Remaining boundaries

`SortObjectKeys` sorts a caller-owned key slice in place using JavaScript enumeration order: canonical unsigned indices below 4294967295 first, then other keys in their existing order. The input should contain unique keys. It is reused by Metrics, Parser Kafka envelopes and Validation; it does not recover insertion order from a Go map.

This is the implemented foundation, not a complete JavaScript runtime emulation. Native Go types replace undefined, prototypes, RegExp object identity, Sets, and Middy interfaces. Typed nil values, cyclic equality, obscure Unicode/encoding inputs, malformed raw DynamoDB objects, and all cross-type equality cases are not exhaustively equivalent. `IsStrictEqual` uses Go deep equality outside numeric values; numeric Go kinds compare as numbers. Constructor fallback diagnostics remain utility-specific. Parameters still has duration rounding/overflow and batch-policy differences documented separately.

Metadata uses isolated snapshots, concurrency coalescing, redirect rejection, and object-only responses rather than sharing the reference's mutable global object. Real LMDS availability, authentication and execution-environment semantics still require cloud acceptance. No AWS resources were created for this work.

See [COMMONS_REUSE.md](COMMONS_REUSE.md) for the upstream call-site audit and completed migrations, [COMMONS_PLAN.md](COMMONS_PLAN.md) for checked work, and [LOCAL_VALIDATION.md](LOCAL_VALIDATION.md) for test evidence.
