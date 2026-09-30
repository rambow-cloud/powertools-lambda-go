# Commons reuse audit

Reference: installed `@aws-lambda-powertools/{commons,logger,metrics,parameters}` v2.35.0, baseline commit `7bcc27b1574493f9452688673658f52b80c53847`. This review inspected actual ESM imports and call sites, not only documentation descriptions. Future-module reuse below is a design conclusion, not a claim those packages have been implemented or fully audited.

## Upstream usage and Go migrations

| Observed reference call sites | Existing Go location | Completed reuse |
| --- | --- | --- |
| Logger.js: environment helpers; LogFormatter.js: development mode | `logger/options.go` | Shared service-name, development-mode, strict log-event boolean mode, and sampling-number parsing. Constructor fallback policy remains local. |
| Logger.js: appendPersistentKeys; LogAttributesStore.js: appendKeys; LogItem.js: addAttributes use deepMerge | `logger/encoding.go`, `logger/logger.go` | Removed private recursive merge routines. Persistent and temporary key updates use Commons merge with reserved-field filtering and independent snapshots. Per-record serialization remains local. |
| Metrics.js: getStringFromEnv/getBooleanFromEnv/getServiceName/isDevMode; stores use shouldUseInvokeStore | `metrics/configuration.go`, existing request scopes | Shared environment/development/service parsing. Existing context-bound store ownership and EMF rules are retained. |
| Parameters/base/GetOptions.js: getNumberFromEnv | `parameters/parameters.go` | Shared numeric conversion and typed invalid max-age errors before cache lookup. Public Lifetime retains its legacy fallback behavior for direct callers. |
| Parameters/base/transformValue.js: fromBase64 | `parameters/transform.go` | Shared standard-alphabet/padding validation and byte conversion replace the permissive private decoder. |
| Parameters/ssm/SSMProvider.js: extended getBooleanFromEnv | `parameters/ssm/ssm.go`, `batch.go` | Shared decryption parser with supported boolean forms and errors. Explicit/SDK decryption precedence and batch quirks remain provider-owned. |
| Parameters/appconfig/AppConfigProvider.js: getServiceName | `parameters/appconfig/appconfig.go` | Shared service-name parsing; application validation and token state remain local. |
| Parameters/appconfig-agent/getConfig.js: service/environment/runtime helpers | `parameters/appconfigagent/agent.go` | Shared runtime detection, local-value/port parsing, and service-name resolution. Agent continues to own caching/polling. |
| Parameters/base/BaseProvider.js: addUserAgentMiddleware | Every Parameters AWS adapter | Shared per-request SDK stack option. Tracer also uses it when installing AWS instrumentation; no duplicate markers. |
| Commons/envUtils.js: request trace context with environment fallback | `internal/invocation/context.go` | Trace extraction delegates to public Commons helpers. Invocation identity stays in the existing context primitive. |
| Commons/metadata.js: getStringFromEnv/isRunningInLambda plus its own cache | `commons/metadata` | Independent client reuses environment/runtime and decoded-value snapshots. No dependency on Parameters or observability packages. |

The upstream DynamoDB Parameters provider uses `@aws-sdk/util-dynamodb`, whereas Commons exposes its own unmarshalling helper. Go preserves that distinction with raw-object and native SDK entrypoints, sharing safe-number/big-integer conversion between them. Parameters uses the SDK entrypoint rather than blindly replacing SDK binary decoding with raw Commons conversion.

## Deliberate non-reuse

- Parameters TTL cache, Commons LRU eviction, AppConfig single-use token state, and Metadata clear-until-refetch caching have different contracts. They remain separate owners.
- Logger reserved fields, arbitrary Go values, replacers, output order, and cycle markers are not configuration-value snapshot rules. Only compatible merge behavior is shared.
- Metrics dimensions/metadata/metric stores retain EMF-specific limits and flush boundaries. Commons does not own them.
- The reference Utility class tracks cold start per instance, while composed Go wrappers share one invocation identity. A public Utility equivalent exists, but replacing wrapper state with separate instances would introduce disagreement.
- Root import side effects modifying AWS_SDK_UA_APP_ID are replaced by explicit SDK middleware. Public version information does not require mutating application state.
- Metadata is available for opt-in enrichment; no ordinary logger/tracer/parameter operation triggers a metadata request implicitly.

## Reuse opportunities for subsequent work

HTTP event handling reuses Commons development-mode parsing and whitespace semantics. CORS request-header trimming and compression Cache-Control parsing reuse Commons whitespace rules; compression Content-Length decisions reuse ParseNumber rather than introducing another JavaScript number parser. The existing Parser schema helpers for permissive Buffer Base64 and maximal-subpart UTF-8 decoding now live in Commons as `DecodeBase64Buffer` and `DecodeUTF8`; HTTP conversion and Parser schemas share those implementations. The strict `FromBase64` contract remains separate. HTTP request/shared stores have neither LRU eviction nor a capacity contract and stay with the HTTP module. Parser/Validation callbacks and Logger/OTel context composition do not add feature-module dependencies to HTTP routing.

JMESPath now reuses Commons Base64 validation, bounded LRU syntax caching, and result snapshots. Query argument type contracts remain local to the engine adapter. Signer reuses the SDK signing implementation and has no artificial Commons dependency. Parser reuses Commons Base64 conversion and snapshots for defaults/unknown values, plus shared invocation identity in wrappers. Parser's DynamoDB helper and stream models now use root Commons raw attribute conversion. The existing SDK adapter delegates to that same pure implementation while retaining native SDK decoding; Parser adds no SDK dependency. Idempotency reuses LRU capacity management, extended boolean/string environment parsing, SDK identity, and shared invocation context while keeping expiry, records, hashing, and conditional writes in its own package. Its DynamoDB response decoder preserves JSON numeric tokens rather than applying the Parameters-specific safe-number/BigInt conversion. Batch composes the same invocation primitive and accepts Idempotency handlers without importing that module or owning Logger/Tracer state.

The optional cache adapter reuses the Idempotency Store/Record contracts and core key/lifecycle logic. Redis-specific TTL, orphan locks, JSON record fields and compare-and-set recovery remain in the adapter. It does not duplicate key extraction or response-cache policy, and the core does not import the Redis client.

The JSON Schema Validation module reuses `commons.CloneValue` to isolate mutable type, enum and const diagnostic parameters from its compiled schemas. It also reuses shared invocation identity and the independent JMESPath module. Schema traversal, JSON declaration order and AJV diagnostic rules remain local to Validation; they do not add dependencies or policies to Commons.

Metrics diagnostic sanitization reuses `commons.IsStringUndefinedNullEmpty`, as the pinned TypeScript Metrics implementation does. The shared `TrimSpace` now excludes U+0085 NEXT LINE from whitespace while retaining BOM removal, matching JavaScript String.trim. The 203 actual Metrics warning scenarios include U+0085, U+200B, BOM and ordinary Unicode whitespace. Warning buffering, EMF collision precedence and callback delivery remain Metrics-owned; Commons does not gain a logging dependency.

Metrics manual cold-start capture reuses `commons.Utility`, including its on-demand initialization check and atomic one-time consumption. Root instances and derived single metrics own independent helpers; request scopes share the originating instance's helper and consult shared invocation identity. Function-name trimming also reuses Commons. Metrics owns the function-name snapshots, cold-start EMF document and wrapper error policy; no second global invocation tracker is introduced.

Metrics construction uses `commons.BoolEnv` and preserves its typed validation errors, rather than using the fallback-only helper needed by utilities with non-error-returning constructors. `StringEnv`, `ServiceName` and `IsDevMode` provide shared parsing; Metrics owns explicit/custom/environment precedence, including the reference's truthy whitespace service behavior. Single metrics reuse `New` for fresh validation and default reconstruction instead of duplicating constructor logic. The custom configuration interface contains only the two getters actually called by the reference Metrics implementation.

`commons.SortObjectKeys` consolidates canonical array-index ordering previously duplicated in Parser Kafka envelopes and Validation object traversal. Metrics now reuses it for metric definitions and map-based diagnostic traversal. It stably moves indices 0 through 4294967294 ahead of other keys; callers retain ownership of insertion order or Go-map lexical ordering. Numeric EMF serialization and default-prototype metric errors stay in Metrics because those policies are not general Commons behavior.

Validation includes pinned fixtures for environment/runtime, Base64, indexed merges, LRU, types, DynamoDB numbers, and Metadata request/cache behavior. Consumer tests cover Logger parent/child isolation and merge semantics, Parameters configuration validation, exact SDK user-agent composition, and existing module regressions. Functional concurrency tests cover LRU, Metadata, and existing request/provider state. CGO remains disabled; no race-detector equivalence is claimed.

AppSync GraphQL reuses `commons.StringEnv` for ALC configuration. Bedrock additionally reuses `commons.ParseNumber` for parameter conversion and `commons.SortObjectKeys` for ordered parameter serialization. Bedrock's response-field inclusion uses JavaScript language truthiness locally: Commons `IsTruthy` deliberately treats empty containers differently. Tool routing, response envelopes and body encoding remain Bedrock-owned, with no Parser, Logger, Tracer, Metadata or AWS SDK module dependency.

Kafka reuses Commons strict Base64 validation, UTF-8 replacement, numeric conversion for byte coercion and object-key ordering. TextDecoder BOM removal and lazy getter/error policy remain Kafka-owned. Parser Kafka headers intentionally convert code points; consumer headers decode bytes, so that schema is not reused. Parser and Idempotency compose through application callbacks without becoming core dependencies. Metadata is not fetched implicitly.

Optional Kafka Avro and Protobuf adapters reuse the core FieldConfig/Decoder boundary and Commons permissive Buffer-style Base64 conversion. Avro also uses Commons UTF-8 replacement. Primitive/JSON decoding retains strict Base64 validation. Binary schema, prefix and preferred index state remain adapter-owned; neither adapter adds dependencies to Kafka core, and neither depends on the other.

## Data Masking reuse

The independent masking core reuses Commons.ParseNumber and Commons.SortObjectKeys. Its private tree preserves source JSON ordering, sequential masking and captured provider inputs; it does not use CloneValue because that helper does not preserve raw JSON order or reject unsupported native graphs. No JMESPath engine is involved: the pinned source uses only dot/wildcard traversal. Cryptographic key/material/cache ownership belongs to an optional provider, not Commons or Logger.

## Shared optional regex

Validation now delegates Unicode payload patterns and legacy property-overlap checks to commons/regex. Data Masking binds the same implementation with Regexp.Replacer through its existing callback boundary. The Unicode tables and translators have one owner; root Commons and datamasking acquire no regex engine dependencies. The separate module adds ECMAScript replacement state and tokens, preserving Validation error wrapping and limits. See REGEX.md and DATAMASKING_PLAN.md for acceptance and remaining scope.

The optional KMS masking provider reuses Commons Buffer-style Base64 decoding, UTF-8 decoding, key ordering and commons/awssdk request identity. It retains its own cryptographic-message/authenticated-context lifecycle. AWS ESDK/MPL dependencies remain isolated from plain masking and root Commons.
