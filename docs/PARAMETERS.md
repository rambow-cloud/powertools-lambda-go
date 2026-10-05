# Parameters

Parameters retrieves configuration from SSM, Secrets Manager, DynamoDB, AppConfig Data and AppConfig Agent. Shared caching and JSON/Base64 transforms live in `github.com/rambow-cloud/powertools-lambda-go/parameters`; service adapters are subpackages of that module.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Complete example

This complete offline example demonstrates the shared cache with an application retrieval callback. It makes no AWS request. Save it in an empty directory inside the checkout and run `go run main.go` with `CGO_ENABLED=0`. For an actual SSM client, use [SSM usage](#ssm-usage) below; create the provider once before serving Lambda invocations.

~~~go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	stdlog "log"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

func main() {
	cache := parameters.NewCache(time.Now)
	fetches := 0
	fetch := func(context.Context) (any, error) {
		fetches++
		return `{"enabled":true,"limit":3}`, nil
	}
	options := parameters.Options{
		Transform: parameters.JSON,
		MaxAge:    parameters.Age(30 * time.Second),
	}
	for range 2 {
		value, err := cache.Get(context.Background(), "/orders/config", options, fetch)
		if err != nil {
			stdlog.Fatal(err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			stdlog.Fatal(err)
		}
		fmt.Println(string(encoded))
	}
	fmt.Printf("fetches=%d\n", fetches)
}
~~~

## Input and output

Stdout is exactly the following. Both calls return the decoded object, but the callback runs only once because the second call uses the still-valid cache entry. Setting `options.ForceFetch = true` before the second call would invoke it again. `cache.ClearCache()` removes cached values; it does not change the underlying configuration service. With an AWS provider, SDK errors are returned to your handler rather than logged as successful values.

~~~text
{"enabled":true,"limit":3}
{"enabled":true,"limit":3}
fetches=1
~~~

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `cache` / explicit provider | Keep one instance across warm invocations to retain cached values; no background polling. |
| `options` | Per-call transform, TTL, force-fetch and missing/error policy. |
| `value` | `any`: JSON objects are `map[string]any`, arrays are `[]any`, numbers are `float64`; inspect or decode explicitly. |
| `ctx` | Caller cancellation/deadline reaches retrieval and SDK operations. |

## TypeScript feature coverage

Compared with the [official v2.35.0 parameters guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/parameters.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| SSM read / path / named batch / write | `ssm.New`, provider methods and convenience helpers | Pagination, decryption and reference batch quirks documented below. |
| Secrets / DynamoDB | Explicit providers or Secrets helper | SDK v2 clients; DynamoDB decoding preserves large integers. |
| AppConfig / Agent | Data provider or Agent `GetConfig` | Data sessions retain tokens; Agent has no additional cache. |
| TTL / fresh values / clearing | `MaxAge`, `ForceFetch`, `ClearCache`, `ClearCaches` | Five-second ordinary default; duration/native edges differ. |
| Transforms / missing values | `JSON`, `Binary`, `Auto`, `ThrowOnMissing` | Explicit Go results/errors; snapshots isolate cached objects. |
| Custom provider / SDK arguments | `Cache.Get`, injected service interfaces and inputs | Callbacks replace inheritance; callers supply region/credentials/permissions. |

Executable evidence: [parameters/parameters_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parameters/parameters_test.go), [parameters/providers_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parameters/providers_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.

## API map

| TypeScript API | Go API |
| --- | --- |
| `SSMProvider` | `parameters/ssm.New(client)` |
| `getParameter`, `getParameters`, `getParametersByName`, `setParameter` | `ssm.GetParameter`, `ssm.GetParameters`, `ssm.GetParametersByName`, `ssm.SetParameter` |
| `SecretsProvider`, `getSecret` | `parameters/secrets.New(client)`, `secrets.GetSecret` |
| `DynamoDBProvider` | `parameters/dynamodb.New(client, Config)` |
| `AppConfigProvider`, `getAppConfig` | `parameters/appconfig.New(client, Config)`, `appconfig.GetAppConfig` |
| AppConfig Agent `getConfig` | `parameters/appconfigagent.GetConfig` |
| Custom `BaseProvider` | `parameters.Cache.Get` / `GetMultiple` with retrieval callbacks |
| `clearCaches` | `parameters.ClearCaches()` for default providers |
| Provider `clearCache` | `provider.ClearCache()` for explicit providers |

All retrievals accept `context.Context`. Providers accept AWS SDK for Go v2 clients through service interfaces, allowing custom credentials, region, endpoints, transports, and tracing. Native SDK input structs carry per-call options; adapters copy them before overriding parameter names, table keys, and other provider-owned fields.

Convenience functions load standard AWS configuration lazily on first use. Initialization failure can be retried; concurrent initialization happens once, with cancellable waiters. Each default provider retains its initial configuration. Default AppConfig retains the application/environment from its first successful initialization, matching the reference. Use explicit providers for different regions, accounts, applications, or environments.

## SSM usage

```go
import (
    "context"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    sdk "github.com/aws/aws-sdk-go-v2/service/ssm"
    "github.com/rambow-cloud/powertools-lambda-go/parameters"
    "github.com/rambow-cloud/powertools-lambda-go/parameters/ssm"
)

// Create this closure once before lambda.Start to retain the provider cache.
func configurationReader(cfg aws.Config) func(context.Context) (any, error) {
    provider := ssm.New(sdk.NewFromConfig(cfg))
    return func(ctx context.Context) (any, error) {
        return provider.Get(ctx, "/orders/config", ssm.GetOptions{
            Options: parameters.Options{
                Transform: parameters.JSON,
                MaxAge: parameters.Age(30 * time.Second),
                ThrowOnMissing: true,
            },
            Decrypt: aws.Bool(true),
        })
    }
}
```

For standard Lambda AWS configuration, use the convenience function:

```go
value, err := ssm.GetParameter(ctx, "/orders/config", ssm.GetOptions{
    Options: parameters.Options{Transform: parameters.JSON},
})
```

`GetMultiple(ctx, path, MultipleOptions)` follows all SSM pages and returns names relative to the requested path. `Recursive` defaults to SDK behavior. `Decrypt` overrides SDK `WithDecryption`, which overrides `POWERTOOLS_PARAMETERS_SSM_DECRYPT`. Shared extended boolean parsing accepts `1/y/yes/t/true/on` and `0/n/no/f/false/off`, ignoring case and surrounding whitespace; invalid values return errors.

`Set(ctx, name, value, *sdk.PutParameterInput)` returns the version. Defaults are `Type=String`, `Tier=Standard`, and `Overwrite=false`. Native options cover KMS key, description, policies, tags, tier, and overwrite. Explicit name/value arguments take precedence. Writes preserve cached reads, matching the reference; use `ForceFetch` or `ClearCache` to observe a write immediately.

### Named batches

```go
values, err := provider.GetParametersByName(ctx, map[string]ssm.GetOptions{
    "/orders/config": {Options: parameters.Options{Transform: parameters.JSON}},
    "/orders/feature": {},
}, ssm.ByNameOptions{ThrowOnError: aws.Bool(false)})
```

Per-name max age, transform, and decryption override batch defaults. Pending names are fetched in batches of at most ten. All-encrypted batches use `GetParameters` with decryption. Mixed batches use individual `GetParameter` calls for encrypted names and batches for the remainder. Graceful mode returns failed names as `values["_errors"].([]string)`; `_errors` is reserved in this mode. SDK transport/service failures from a batch still return errors. Graceful transform failures produce nil entries without adding names to `_errors`.

The pinned implementation has observable inconsistencies that are deliberately retained:

- Encrypted names in mixed batches bypass transforms and cache, returning raw values on every call.
- Batched retrieval ignores `ForceFetch`. Clear the cache or use individual `Get` with `ForceFetch` to refresh.
- Batch max age defaults to five seconds independently of the environment. Explicit nonpositive ages bypass batch cache lookup and storage; the pinned zero-age fallback is intentionally corrected.
- Empty batch strings become nil. Transformed false, zero, and null values are returned but not cached by this path.

Reference fixtures cover mixed decryption, repeat requests, empty values, graceful missing names, outputs, and SDK operation sequences. Go leaves input maps unmodified and sorts names for deterministic batching.

## Cache and transformations

`parameters.Options` has `MaxAge *time.Duration`, `ForceFetch`, `Transform`, `ThrowOnMissing`, `ThrowOnTransformError`, and a provider request discriminator `RequestKey`. SSM and Secrets Manager set the discriminator automatically; callers normally leave it empty. Use `parameters.Age(0)` for explicit zero; nil selects the default. Ordinary retrievals read `POWERTOOLS_PARAMETERS_MAX_AGE` per call, defaulting to five seconds only when absent. Invalid numeric input returns a shared typed configuration error before cache lookup. The legacy public `Lifetime()` accessor retains its fallback-on-error behavior. Nonpositive lifetimes bypass existing entries and skip storage for that call, without evicting earlier valid entries. This includes explicit zero and a zero environment default. Low-level `Lookup` misses and `Store` skips storage when lifetime configuration is invalid; ordinary `Get`/`GetMultiple` return the configuration error.

Expiry is checked on access; a value remains valid at the exact expiry boundary. Retrieval/transform errors, missing raw values, and empty collections are not cached. Cache keys include name, transform, single/multiple operation, and the effective request discriminator. SSM single/batch reads isolate decryption; path reads also isolate recursive mode and SDK filter/pagination selectors. Secrets Manager isolates SDK version ID and stage selectors. Provider overrides and environment defaults are resolved before lookup; caller SDK inputs remain unchanged. Other providers retain their existing SDK-option cache boundaries. Strict-error flags remain outside cache keys. Lifetime is fixed on insertion. A strict transform call can reuse a cached permissive result unless forced.

The cache protects maps with a mutex and copies supported mutable decoded values on storage/retrieval. Network calls run outside the lock; concurrent misses may fetch independently. `ClearCache` prevents in-flight `Get`/`GetMultiple` callbacks from repopulating cleared entries. Batched `Lookup`/`Store` operations are individually synchronized but are not an atomic transaction with clearing; join batches before clearing when strict invalidation is required. There is no background refresh or capacity eviction. Each Lambda execution environment has its own cache.

| Transform | Result |
| --- | --- |
| Unset | Original string, bytes, or decoded DynamoDB value |
| `parameters.JSON` | JSON objects as `map[string]any`, arrays as `[]any`, numbers as `float64` |
| `parameters.Binary` | Base64-decoded UTF-8 string after Commons standard-alphabet and padding validation |
| `parameters.Auto` | JSON for `.json`, base64 for `.binary`, otherwise unchanged; suffixes ignore case |

Transform names ignore case. Single-value failures return `*parameters.TransformParameterError`. Multiple retrievals retain failed entries as nil by default; `ThrowOnTransformError` returns the error. Non-string/non-byte values pass through. JSON null and missing values both map to nil; successfully decoded JSON null can still be cached by ordinary `Get`.

Byte input and Base64-decoded binary output follow `TextDecoder`: each maximal malformed UTF-8 subpart becomes a separate replacement character, and exactly one leading BOM is stripped at each decoding boundary. BOM-prefixed byte JSON therefore parses normally. Unset transforms preserve raw bytes. Sixteen pinned reference cases cover these boundaries, including Auto transforms, in `testdata/utf8-v2.35.0.json`.

`ThrowOnMissing` returns `*parameters.ParameterNotFoundError` for absent raw values. SSM/Secrets Manager normalize SDK not-found exceptions only with this option enabled; otherwise those exceptions remain available under `*parameters.GetParameterError`. Use `errors.As` for types and `errors.Is` for cancellation/deadlines. Writes return `*parameters.SetParameterError` on failure.

## Other providers

Secrets Manager uses `GetSecretValue`. Nonempty `SecretString` takes precedence, followed by raw `SecretBinary` bytes. The SDK already decodes wire-level base64 for `SecretBinary`; leaving the transform unset preserves those bytes. Selecting `Binary` requests an additional decoding step, matching the reference. SDK options accept version ID/stage. Empty strings without binary values count as missing. Bulk retrieval is not supported by the reference provider and is not exposed in Go.

DynamoDB requires `TableName`; attributes default to `id`, `sk`, and `value`. `Get` sends a partition-key lookup and value projection. `GetMultiple` queries that partition, follows every page, and indexes values by sort attribute. The SDK decodes native maps, lists, sets, booleans, numbers, nulls, and binary. SDK options support consistent reads, limits, index selection, and compatible fields. Provider-owned key/projection fields take precedence. Composite-key single-item lookups need custom retrieval callbacks; the reference also supplies only its partition key for `Get`.

AppConfig Data requires application/environment; application falls back to `POWERTOOLS_SERVICE_NAME`. Session SDK options accept `RequiredMinimumPollIntervalInSeconds`. Tokens and last known bytes are retained per profile. Tokens rotate after polling and expire locally after 23 hours 45 minutes. Same-profile requests are serialized because tokens are single-use; waiters can cancel. Empty updates return the previous bytes. Clearing the transformed cache keeps session/last-value state. Failed polls discard possibly consumed tokens so the next call starts a new session.

Like the pinned reference, AppConfig uses the caller's cache lifetime rather than scheduling from `NextPollIntervalInSeconds`. Set lifetime to the service's permitted polling interval in production. `ForceFetch` bypasses the cache; no background poller is created.

AppConfig Agent performs one HTTP GET per call with no extra cache. Options include application, environment, transform, timeout, and missing behavior. Timeout defaults to three seconds; `AWS_APPCONFIG_EXTENSION_HTTP_PORT` defaults to 2772. URL identifiers are escaped individually. Application falls back to the service name. HTTP 404 means missing; other non-success statuses return retrieval errors.

Agent Lambda detection follows the reference: `AWS_LAMBDA_INITIALIZATION_TYPE` is set and not `unknown`, with development mode disabled. Outside Lambda it reads `POWERTOOLS_APPCONFIG_AGENT_RETURN_VALUE`; empty means missing. `Endpoint` explicitly enables HTTP to a local agent outside Lambda. `HTTPClient` accepts an instrumented transport. Error messages omit response bodies to avoid copying configuration data into diagnostics.

## Compatibility boundaries

All five providers and the reference convenience-function families are implemented. Complete JavaScript parity is not claimed:

- Go uses contexts, SDK v2 inputs, typed errors, duration pointers, and nil instead of JavaScript-specific values/exceptions.
- Cache results are isolated snapshots; single/multiple entries cannot collide. The reference returns shared objects and uses the same string-key format for both operations.
- Unknown transform names return errors. Base64 validation now uses the shared Commons contract; prior permissive handling was removed. Extreme duration values and fractional TTL boundaries still differ from JavaScript date handling.
- DynamoDB now uses the shared number-preserving SDK decoder: safe values become float64, large integer strings become *big.Int, and unsupported large decimal/exponent forms fail. Set identity, native type representation, and malformed item cases still need exhaustive parity coverage.
- AppConfig serializes session access, restarts sessions after failed polls, and rejects empty application/environment configuration earlier.
- SDK-specific error messages, prototype-only base-provider methods, and exhaustive invalid-configuration diagnostics are not reproduced. The shared SDK middleware now adds one feature marker using the Go version, without global environment mutation.

## Validation

Service tests use separate layers. Fake clients exercise provider inputs, typed
error chains, failed-fetch recovery, cache reuse and cancellation without HTTP.
They return synthetic SDK outputs and do not validate SDK serialization or AWS
behavior. AppConfig fake tests additionally cover single-use token rotation,
session expiry, unchanged values and concurrent callers. Real SDK protocol tests
and stateful local acceptance provide the next layers.

On 2026-09-14, the complete Go test suite, vet, and both Linux architecture builds passed with CGO disabled. Local Docker Lambda acceptance passed **94/94 assertions** across five invocations. All five providers ran with Logger, Metrics, and OTel Tracer. Checks include warm cache reuse, forced refresh, AppConfig token rotation/empty updates, and agent-owned caching. Containers and the internal network were removed.

Commons migration validation on the same date passed the complete regression suite and **100/100 Docker assertions**, including Metadata and shared SDK marker composition. Existing Parameters cases remain in the suite. [COMMONS_REUSE.md](COMMONS_REUSE.md) records the migrations and retained provider-specific policies.

Unit tests execute real SDK requests against loopback fixtures and cover reference output, cache/transforms, writes/batches, pagination, default helpers/global clearing, and functional concurrency. Fixtures do not establish IAM, KMS, service quotas, throttling, or live AppConfig delivery. No AWS resources were deployed for this module. Remaining parity, cloud, and performance gates stay unchecked in [CHECKLIST.md](CHECKLIST.md). See [LOCAL_VALIDATION.md](LOCAL_VALIDATION.md) for runtime evidence.
