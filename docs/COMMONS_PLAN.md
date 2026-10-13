# Commons and Metadata implementation plan

Status: foundation implemented and verified on 2026-09-14, before JMESPath and Batch. See [COMMONS.md](COMMONS.md) for APIs and explicit compatibility boundaries, and [COMMONS_REUSE.md](COMMONS_REUSE.md) for the inspected upstream call sites and completed migrations.

## Reuse and ownership

| Capability | Existing or planned consumers | Boundary |
| --- | --- | --- |
| Environment string/boolean/number parsing | Logger, Tracer, Metrics, Parameters; future utilities | Share parsing semantics, explicit fallback handling, and error categories. Keep per-utility precedence/defaults in each utility. |
| Invocation identity, cold start, runtime detection, trace context | Existing observability wrappers; future Batch and Idempotency integration | Preserve the existing shared context identity. Separate configuration helpers from invocation lifecycle ownership. |
| Base64 and byte/text conversion | Parameters; planned JMESPath functions and Parser envelopes | Match the pinned Commons validator and encoding modes before migrating callers. |
| SDK user-agent/version helpers | Parameters, Tracer; planned persistence and signing integrations | Isolate SDK-specific code from pure helpers and avoid duplicate markers. Audit upstream import side effects explicitly. |
| LRU and expiry primitives | Planned Idempotency response cache and other proven consumers | LRU eviction and TTL freshness are different contracts. Preserve Parameters cache keys, zero-age behavior, and batch quirks. |
| DynamoDB value conversion | Parameters; planned Parser and persistence code where relevant | Audit numeric precision and type semantics before deciding what can be shared. Provider-specific records remain local. |
| Metadata retrieval | Public Metadata API; possible opt-in Logger/Tracer enrichment | Independent HTTP client with authentication, timeout, caching, and local behavior. No automatic request on each invocation. |

## Concrete findings

- `internal/invocation` retains shared cold-start identity and delegates trace extraction to Commons. Environment/service-name helpers now live in Commons rather than invocation management.
- Parameters max age, SSM decryption, Agent runtime/development configuration, and compatible observability settings now use shared helpers. Utility-specific precedence and fallback policies remain local.
- The shared extended boolean parser now accepts `1/y/yes/t/true/on` and `0/n/no/f/false/off`, trims values, and rejects unsupported values. Consumers select strict/extended parsing and explicitly retain fallback policies where required by their Go APIs.
- Parameters now uses the pinned Commons padding/alphabet validation before Base64 decoding. Whitespace, URL-safe, and incomplete-padding acceptance from the previous private decoder was removed. Differential fixtures cover these changes.
- Service-name defaults differ: logging uses a usable fallback, while AppConfig requires an application. A shared parser must not replace these distinct defaults with one global value.
- The pinned Metadata helper reads `AWS_LAMBDA_METADATA_API` and `AWS_LAMBDA_METADATA_TOKEN`, sends a bearer-authenticated request to `/2026-01-15/metadata/execution-environment`, defaults to a one-second timeout, caches a nonempty result until cleared, and returns an empty object outside Lambda. Metadata does not use the Parameters provider cache or require AWS SDK credentials.

## Work checklist

- [x] C-01: Audit pinned Commons/Metadata exports, configuration modes, encodings, types, defaults, errors, and import side effects; map public Go equivalents in COMMONS.md and actual call sites in COMMONS_REUSE.md.
- [x] C-02: Generate TypeScript differential fixtures for environment/runtime values, Base64, merge/LRU/types, DynamoDB values, and Metadata request/cache behavior.
- [x] C-03: Implement shared configuration/runtime helpers and migrate existing utilities while preserving module-specific policies and public APIs. Verify consumer regressions.
- [x] C-04: Implement shared encoding behavior and integrate it with Parameters; compare 50 reference encoding cases and update compatibility documentation.
- [x] C-05: Implement SDK identity/version helpers with isolated SDK dependencies and no import-time environment mutation. Verify one marker when Tracer and Parameters compose.
- [x] C-06: Implement LRU, indexed/cycle-aware deep merge, raw/native DynamoDB conversion, and exact large integer handling. Verify fixtures and reuse in Logger/Parameters.
- [x] C-07: Implement Metadata cancellation, authentication, timeout, cache clearing, local behavior, unknown fields, isolated snapshots, and concurrent fetch coordination. Verify HTTP and 100-caller tests.
- [x] C-08: Pass full Go tests/vet, both Linux architecture builds, and 100/100 Docker Lambda assertions with CGO disabled; remove disposable containers/network.
- [x] C-08a: Extract raw DynamoDB conversion and numeric parsing into root Commons for Parser reuse. Retain SDK decoding in the optional adapter, existing public delegates and an error alias. Commons/SDK/Parser regression tests, all 18 packaged modules and 15 consumers, both Lambda architectures and 205/205 local Docker assertions passed (2026-09-15). The root and Parser dependency graphs remain free of external modules.
- [ ] C-09: Close remaining native Go/JavaScript type, encoding, malformed-input, and diagnostic boundaries listed in COMMONS.md through additional reference coverage.
- [ ] C-10: Validate live LMDS behavior in AWS and establish allocation/latency/cache-size budgets.

Real LMDS availability and execution-environment behavior require a separately authorized cloud test. Loopback fixtures establish protocol/client behavior only. No implementation item is checked merely because this plan was created.

## Finite boundary audit #180

- [x] C-FINITE-180: The matrix below passes four affected packaged modules and
  four independent consumers with CGO disabled/GOWORK off, scoped JSON v2 policy,
  licenses, navigation, eighteen guide contracts, strict documentation build and
  discovery checks (2026-10-13). Hosted runtime/service checks remain separate.

Baseline: the installed TypeScript v2.35.0 Commons exports and existing
environment/encoding/type/merge/LRU/DynamoDB/Metadata fixtures. New generator
`generate-commons-boundaries.mjs` retains 71 actual observations; the optional
DynamoDB module owns a separate 24-number fixture for packaged consumption.

| Public surface | Evidence and decision |
| --- | --- |
| `ParseNumber`, `DynamoDBNumber`, `IsTruthy`, `IsIntegerNumber`, `GetType` | Verify 24 numeric strings and four non-finite/negative-zero type cases. Preserve the pinned Commons truthiness of NaN, exact BigInt integers, overflow infinities and errors for large decimal/exponent forms. Native Go numeric kinds remain supported. |
| `FromBase64` | Thirty malformed/padded/alphabet and Base64/hex/UTF-16 observations supplement the existing fifty encoding cases. Compare exact bytes and success/failure; native diagnostic spelling remains a Go difference. |
| `UnmarshallDynamoDB` | Thirteen actual raw-attribute cases. Reproduce and fix repeated NaN members in numeric sets; retain one member as JavaScript Set does. Native BOOL zero/NaN conversion now applies to every Go numeric kind. The fixture's empty-array M result remains unchanged: Go deliberately rejects non-object M values with its typed attribute error. Malformed multi-tag and non-JSON/native objects are not exhaustive parity claims. |
| `DeepMerge`, `CloneValue`, `LRUCache` | Reuse named-map/slice/mutual-ancestor and sibling-isolation regressions. Add byte/string/number snapshot mutation, Has-without-recency, update/remove/clear and caller-owned LRU values. Opaque Go objects, deep equality and nil remain documented native differences. |
| Optional SDK DynamoDB adapter | Execute the 24 numeric cases through actual SDK AttributeValue decoding as well as shared conversion. Existing nested sets, binary and BigInt clone tests remain passing evidence; raw B retention and native SDK binary representation are distinct contracts. |
| Optional SDK user-agent adapter | Reuse the actual SDK request test proving one first-feature marker, preserved SDK metadata and no global environment mutation. No additional import-time side effects or SDK dependency enters root Commons. |
| `metadata.Client.Get`, `ClearCache` | Reuse 100-caller coalescing, clear-during-fetch, cancelled waiter, timeout/status failures and redirect rejection. Add five strict JSON v2 body failures with successful recovery, cache/snapshot isolation and once-only body closure, plus six invalid endpoints and pre-cancelled requests that never reach the injected transport. |

Go Metadata's strict object-only JSON v2, private snapshots, generation barriers
and redirect rejection intentionally differ from the reference's mutable global
cache. New protocol checks use an injected transport and synthetic values only.
C-09 exhaustive native/encoding/diagnostic equivalence, C-10 resource/LMDS gates
and service acceptance #201/#202 remain open. This finite audit does not authorize
AWS calls, change invocation identity or implement regex v-mode (#167).
