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
