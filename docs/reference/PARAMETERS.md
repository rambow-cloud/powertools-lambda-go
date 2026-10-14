# Parameters: API and behavior notes

Start with the [Parameters guide](../PARAMETERS.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

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

Byte input and Base64-decoded binary output follow `TextDecoder`: each maximal malformed UTF-8 subpart becomes a separate replacement character, and exactly one leading BOM is stripped at each decoding boundary. BOM-prefixed byte JSON therefore parses normally. Unset transforms preserve raw bytes. Sixteen pinned reference cases cover these boundaries, including Auto transforms, in `testdata/utf8-the upstream baseline.json`.

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

JSON transforms use `encoding/json/v2`: duplicate object members, invalid UTF-8 and lone escaped surrogates produce `TransformParameterError` with the underlying JSON error as its cause. JSON byte input is validated without repairing malformed text. Binary transforms retain their documented UTF-8 text decoding behavior.


## Validation

### Finite boundary audit #184

- [x] P-FINITE-184: The affected packaged module passes tests/vet/tidy/isolation
  and one independent consumer with CGO disabled/GOWORK off. Scoped JSON v2,
  navigation, eighteen guides, strict site build and discovery checks pass
  (2026-10-13); hosted runtime/service acceptance remains independent.

Baseline: installed TypeScript the upstream baseline BaseProvider/GetOptions/ExpirableValue
and existing Go reference, effective SDK cache-option, fake-client and real SDK
protocol suites. New generator `generate-parameters-boundaries.mjs` retains
nineteen actual duration/cache observations, including original TTL values.

| Public surface | Evidence and classification |
| --- | --- |
| `Options.Lifetime`, `Cache.Get` | Fifteen environment inputs cover absent/empty, signed, hexadecimal, fractional, underflow, invalid/NaN, infinities and duration saturation. Retrieval/call/cache-insertion outcomes match the reference; typed Go configuration causes and the public Lifetime fallback remain native contracts. Fractional precision and bounded Go duration differ from Date.setSeconds/invalid Date TTL and are explicit. |
| Expiry, `ForceFetch`, `ClearCache`, `GetMultiple` | Actual upstream sequences verify exact expiry, just-after expiry, forced refresh, clear and fresh zero-age storage bypass. With existing warm entries, TypeScript can return the old value for zero/negative age; Go deliberately bypasses lookup without evicting earlier valid entries. Both original warm observations and Go's policy are checked explicitly. The fractional sequence retains the reference's rounded expiry while Go keeps 100 ms. A channel-fenced native batch fetch verifies that clearing prevents an older callback from repopulating cache; byte results stay isolated across cached reads. |
| Effective SDK request options | Reuse SSM single/path/batch decryption, recursive/filter/pagination and Secrets version/stage tests. Provider options override SDK values, then environment defaults; caller inputs are unchanged and value-affecting variants use separate cache keys. Invalid environment values are checked before warm lookup. |
| Provider native values and errors | Reuse SSM/Secrets/DynamoDB fake SDK errors/missing-output/cancellation/recovery tests and strict real-SDK protocol corpus for all providers. Preserve native binary/large-number conversion, typed unwrap causes and missing policies. Fake tests and local HTTP tests prove distinct layers, not AWS service behavior. |
| AppConfig and Agent | Reuse single-use token rotation/expiry/unchanged-output/concurrency/restart tests, Agent direct-fetch/development/timeout/error paths and global default-provider clearing. Their provider-specific ownership/cache policies are retained. |
| `TransformValue`, ordinary/safe transforms | Reuse JSON/Binary/Auto, text/BOM/UTF-8 fixtures and strict JSON v2 duplicate/surrogate/invalid-byte rejection. Strict-transform flags remain outside cache keys; cached permissive results follow the documented policy. |

No additional production defect was reproduced in this finite audit. Existing
provider APIs/dependencies and duration, cache, JSON v2 and native error policies
are preserved. Exhaustive native diagnostics/provider options, cache/resource
budgets (#200) and service IAM/throttling/version/platform acceptance (#201/#202)
remain open. The audit makes no cloud calls.

Service tests use separate layers. Fake clients exercise provider inputs, typed
error chains, failed-fetch recovery, cache reuse and cancellation without HTTP.
They return synthetic SDK outputs and do not validate SDK serialization or AWS
behavior. AppConfig fake tests additionally cover single-use token rotation,
session expiry, unchanged values and concurrent callers. Real SDK protocol tests
and stateful local acceptance provide the next layers.

The [HTTP scenario corpus](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/parameters/testdata/README.md) records the official
API sources and synthetic provenance of response samples. Real SDK clients execute
strict request/response sequences covering pagination, wire binary decoding,
AppConfig token headers, service errors, bounded retries and HTTP cancellation.
This verifies local protocol handling; service-side throttling and live delivery
remain outside the fixture's scope.

On 2026-09-14, the complete Go test suite, vet, and both Linux architecture builds passed with CGO disabled. Local Docker Lambda acceptance passed **94/94 assertions** across five invocations. All five providers ran with Logger, Metrics, and OTel Tracer. Checks include warm cache reuse, forced refresh, AppConfig token rotation/empty updates, and agent-owned caching. Containers and the internal network were removed.

Commons migration validation on the same date passed the complete regression suite and **100/100 Docker assertions**, including Metadata and shared SDK marker composition. Existing Parameters cases remain in the suite. [COMMONS_REUSE.md](../COMMONS_REUSE.md) records the migrations and retained provider-specific policies.

Unit tests execute real SDK requests against loopback fixtures and cover reference output, cache/transforms, writes/batches, pagination, default helpers/global clearing, and functional concurrency. Fixtures do not establish IAM, KMS, service quotas, throttling, or live AppConfig delivery. No AWS resources were deployed for this module. Remaining parity, cloud, and performance gates stay unchecked in [CHECKLIST.md](../CHECKLIST.md). See [LOCAL_VALIDATION.md](../LOCAL_VALIDATION.md) for runtime evidence.

## Objects and lifecycle

| Object | How to use it |
| --- | --- |
| Default provider | Created lazily by `ssm.GetParameter` and reused across calls. |
| Explicit provider | Create once when you need a custom SDK client or separate cache. |
| Invocation context | Pass it to reads so cancellation and deadlines reach the SDK. |
| Retrieval options | Choose caching, transformation and missing-value behavior per call. |

