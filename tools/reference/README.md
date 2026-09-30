# TypeScript reference fixtures

These `.mjs` files are JavaScript ES modules used only during development. Node.js runs the exact pinned TypeScript Powertools distributions and saves their observable output as JSON. Go tests consume those checked-in JSON files without invoking Node.js.

From this directory:

```text
npm ci
npm run fixtures
```

| Generator | Purpose |
| --- | --- |
| `generate.mjs` | Logger field precedence, removal, filtering, and basic output |
| `generate-sampling.mjs` | Deterministic constructor sampling and first/warm refresh decisions |
| `generate-metrics.mjs` | EMF dimensions, repeated values, automatic flush boundaries, and single metrics |
| `generate-parameters.mjs` | Cache and transform output plus SSM batching/decryption quirks and SDK operation sequences |
| `generate-commons.mjs` | Environment/runtime, Base64, deep merge, LRU/types, DynamoDB values, and Metadata request/cache behavior |
| `generate-signer.mjs` | Thirteen fixed-time SigV4 request/body/header cases, including the duplicate-query difference |
| `generate-jmespath.mjs` | Seventy-eight standard/function/envelope/error cases, including explicitly recorded undefined results |
| `generate-batch.mjs` | Forty-three sync/async sequential, FIFO/group, stream and failure-policy cases, plus four invalid envelopes |
| `generate-idempotency.mjs` | Twenty-five canonical keys, fourteen lifecycle scenarios, and two DynamoDB SDK command sequences |
| `generate-idempotency-cache.mjs` | Sixteen cache persistence, expiry/lock, conflict, custom-field and malformed-record scenarios |
| `generate-validation.mjs` | 502 JSON Schema validation results, ordered AJV diagnostics, compilation errors, formats, references and extraction cases |
| `generate-validation-regex.mjs` | 6,085 JavaScript Unicode regex/property/alias cases with exact results and errors |
| `generate-validation-applicators.mjs` | 1,207 nested applicator, ordering, conditional, dependency and property-name cases |
| `generate-validation-references.mjs` | 1,271 mixed-type, nullable, format and reference-scope cases |
| `generate-validation-strict.mjs` | 3,047 strict property/pattern overlap cases with separate UTF-16 and Unicode semantics |
| `generate-validation-setup.mjs` | 658 original-document structure, unused/referenced definitions, external schemas and pattern-compilation cases |
| `generate-validation-keywords.mjs` | 3,433 referenced keyword, empty combinator, size-limit, required-value and floating-point multipleOf cases |
| `generate-validation-shapes.mjs` | 5,764 reachable/unused keyword types, annotations, primitive fragments, conditionals and property-dependency cases |
| `generate-validation-graphs.mjs` | 1,104 conditional reachability, dialect, external resource identity, relative scope and registration-order cases |
| `generate-http.mjs` | 414 HTTP event, routing, request/response, middleware, error and Standard Schema validation cases |
| `generate-validation-unicode.mjs` | Explicit Unicode 16 table regeneration from official alias files and Node v22.21.1; not required for normal fixture generation |
| `bridge-idempotency-cache.mjs` | Actual TypeScript writes/reads against the disposable Valkey container; invoked by the local Docker runner |

Powertools distributions are pinned to `2.35.0` in `package.json` and `package-lock.json`; the JavaScript SSM SDK peer dependency has its own exact version. Fixtures record the reference version. The sampling generator temporarily replaces Node's random integer function and restores it before exit. The Parameters generator replaces the injected SDK client's send method with deterministic responses, making no AWS requests. Neither generator changes the installed package.

The Parser generator (`node generate-parser.mjs`) executes Parser v2.35.0 with Zod v4.1.12. It writes forty parsing/schema/envelope cases to `parser/testdata/typescript-v2.35.0.json` and inventories exports in `docs/PARSER_EXPORTS.json`. The Go test compares code/message/path/expected issue fields; only JSON syntax diagnostic suffixes are normalized across runtimes. Full Zod error trees and other schema families remain open.

`node generate-parser-streams.mjs` adds 106 cases covering all 22 runtime exports in SNS, DynamoDB, Kinesis, Firehose and CloudWatch schema families, six envelopes, and the DynamoDB helper. Fixtures retain a shared BigInt/non-finite-number representation and compare JavaScript Sets with Go slices. See `docs/PARSER_STREAMS.md` for the scope and remaining error/format boundaries. These reference generators make no AWS requests.

Normalization is intentionally narrow and documented in the corresponding tests. Passing a fixture proves that scenario only, not complete package parity. The scripts, npm packages, and Node.js are not part of the Go Lambda executable or deployment ZIP.

`node generate-http-stream.mjs` adds 272 streaming cases. Only the Lambda-owned metadata/destination hook is substituted; routing, conversion and middleware run from the actual pinned package. Metadata fields and binary/text bytes are exact; JSON body values use the existing structural comparison. The generator records destination end requests, not a promise that Node has synchronously flushed its final callback. See `docs/HTTP_STREAMING.md` for lifecycle, native SDK integration and platform limits.

`node generate-http-middleware.mjs` adds 1,076 actual CORS/compression router cases to the initial 414 HTTP cases. It compares all four event formats, preflight/route policies, encoding negotiation, thresholds and body/header interactions. Compressed output is compared by decoded bytes; any compressed Content-Length is independently validated against that runtime's bytes. See `docs/HTTP_MIDDLEWARE.md` for explicit representation differences and remaining boundaries.

All Parser generators share `parser-issues.mjs`. It preserves code/message/path/expected fields and recursive union branch errors without flattening or sorting branches. Constraint-specific metadata remains outside this projection. The Go comparison normalizes runtime-specific JSON syntax suffixes recursively.

`node generate-parser-http.mjs` writes 458 cases for eighteen HTTP schema exports and five body envelopes. It covers field omissions/type mutations, identity/IP/refinement rules, combined errors and explicit body decoding semantics. See `docs/PARSER_HTTP.md` for the mapping and remaining gates.

`node generate-parser-services.mjs` writes 270 cases for fifteen schema exports from Kafka, CloudFormation, Transfer, Connect, SES and S3, plus the Kafka envelope. It retains source JSON order where significant and observes the three deliberately rejected Zod validation promises without altering the Powertools result. Unexpected rejections fail generation. The Go tests explicitly distinguish these runtime errors from validation failures. See `docs/PARSER_SERVICES.md` for scope, ordering and Unicode boundaries.

`node generate-parser-identity.mjs` writes 1,467 cases for all 29 unique AppSync/shared, AppSync Events and Cognito exports plus the Null primitive. It checks nested field mutations, identity branches, challenge names, fixed trigger sources and input response flags. The generator also maps all 90 public runtime schema names from the pinned export inventory to Go definitions in `docs/PARSER_SCHEMA_MAP.json`; missing symbols fail generation. Complete inferred-type and behavior mapping remains separate. See `docs/PARSER_IDENTITY.md`.

`node generate-parser-unions.mjs` adds 1,152 ordinary/safe cases for recursive unions, continuable refinements, branch selection, strict objects, arrays, dictionaries, nullable values, transforms and JSON parsing. It also observes array length-check order and wrong-type length diagnostics. Combined Parser coverage is 3,493 cases. See `docs/PARSER_ERRORS.md`.

## Metrics store fixtures

Metrics store fixtures are generated separately with `node generate-metrics-stores.mjs`: 104 scenarios use actual public v2.35.0 APIs for selective clearing, timestamp reset, empty-buffer policy and single-metric behavior. Date.now is injected, dimension names are sorted, and the specific empty-metrics error maps to Go ErrEmptyMetrics. Warnings are outside these assertions; warning parity remains a separate gate in `docs/METRICS_PLAN.md`.

## HTTP observability fixtures

Run `node generate-http-metrics.mjs` before `node generate-http-tracer.mjs`; the second generator reuses the first generator's four event forms. Metrics runs the actual pinned Metrics implementation (176 cases). Tracer runs the actual pinned HTTP middleware with a recording Tracer contract (128 cases), without executing the retired X-Ray SDK. Go compares these contracts with actual EMF output and OTel SDK spans. See `docs/HTTP_OBSERVABILITY.md` for normalization and runtime differences.

## Metrics warning fixtures

Run `node generate-metrics-warnings.mjs` for 203 actual v2.35.0 diagnostic scenarios: invalid and duplicate dimensions, EMF collisions, namespace and empty-buffer warnings, timestamp boundaries and Unicode whitespace. Date.now is injected. Only dimension-name order is normalized; warning text, timestamps and emitted values are exact. Functional Go tests additionally cover callback reentrancy after storage unlock, concurrent scopes, automatic flush and failed output. Full parity remains bounded by `docs/METRICS_PLAN.md`.

## Metrics cold-start fixtures

Run `node generate-metrics-coldstart.mjs` for 302 v2.35.0 cases covering constructor/environment/setter/argument precedence, empty and Unicode names, initialization types, disabled emission, repeated capture, clearing and independent single metrics. It freezes Date.now and sorts only dimension-name arrays; full documents, timestamps and warnings are compared. Go adds concurrent and failed-output checks plus Lambda wrapper/manual composition. Configuration changes after construction and default reconstruction have a separate configuration fixture.

## Metrics configuration fixtures

Run `node generate-metrics-config.mjs` for 532 actual v2.35.0 cases: explicit/custom/environment precedence, exact custom getter order and errors, extended booleans and invalid settings, cached parent configuration, fresh single-metric construction after environment changes, default-service restoration and dimension overflow, and constructor single-metric mode. Date.now is injected; only dimension-name arrays are normalized. Go-specific tests additionally preserve typed environment/custom error identity and failed cold-start construction consumption.

## Metrics value fixtures

Run `node generate-metrics-values.mjs` for 641 actual v2.35.0 scenarios covering numbers, exact diagnostics, UTF-16 metric-name limits, all units/resolutions, numeric property order, reserved-envelope precedence, default prototype names and automatic flush boundaries. Special input tags preserve NaN, infinities and negative zero. Only dimension-name arrays are sorted; metric-definition order, complete JSON values, errors and warnings are compared exactly. The shared key-order helper also runs through existing Parser and Validation reference suites.

## Metrics timestamp fixtures

Run `node generate-metrics-timestamps.mjs` for 968 actual v2.35.0 cases covering numeric and Date inputs, inclusive CloudWatch time windows, non-finite/fractional values, Date range limits, negative epochs, submillisecond Go instants, disabled emission, strict empty policy, selective clearing and single-metric reconstruction. Date.now is injected; only dimension-name arrays are sorted. Full documents, warnings, errors and cumulative clock calls are compared. An out-of-range Go instant represents an invalid JavaScript Date. Numeric-to-Date fixture inputs use JavaScript TimeClip truncation before creating the Go instant.

## Metrics wrapper fixtures

Run `node generate-metrics-wrappers.mjs` for 876 actual v2.35.0 middleware cases. The harness directly executes before/after/onError hooks and the METRICS_KEY early-cleanup callback; it does not emulate or claim to test the Middy framework. Cases cover singleton/empty/reordered/duplicate target arrays, default-dimension merges and limits, inherited/explicit strict policy, disabled emission, business errors, empty/partial metrics and early returns. Only dimension-name arrays are sorted; full documents, warning/error text, business execution and publication order are compared. Go selects PropagateErrors for equivalent publication-error precedence. Provisioned initialization suppresses cold capture in these fixtures; local Lambda composition checks on-demand cold and warm calls separately.

## Bedrock Agent fixtures

Bedrock Agent function fixtures use `node generate-bedrock.mjs` (or
`npm run fixtures:bedrock`; included in `postfixtures`). The 371 scenarios compare
full body strings, response envelopes, ordered calls and diagnostics from the
actual v2.35.0 public resolver and response builder. Body JSON is not decoded or
sorted in the comparison. Special numbers are tagged only in recorded parameters.
Native tests additionally cover concurrent contexts, cancellation, nil results,
cycle errors, parameter ownership and diagnostic reentrancy.

## AppSync GraphQL fixtures

AppSync GraphQL has a separate generator: `node generate-appsync-graphql.mjs`
(also `npm run fixtures:graphql`, and the `postfixtures` hook after the complete
fixture command). It executes 114 resolver scenarios and 91 clock-controlled
scalar cases. Ordered diagnostics, handler calls, values and errors are compared
without sorting; top-level undefined maps to null. Native Go concurrency, context,
panic and UUID checks supplement the fixtures. See `docs/APPSYNC_GRAPHQL.md`.

## AppSync Events fixtures

Run `node generate-appsync-events.mjs` for 100 actual v2.35.0 scenarios. Cases cover publish/subscribe dispatch, literal/wildcard paths, specificity, replacement and stale-cache behavior, invalid envelopes, malformed-publish fallback, individual/aggregate errors, authorization, omitted/null values and size warnings. Each scenario can contain multiple registrations and resolutions. Long strings use complete UTF-8 length and SHA-256; item calls/error logs are compared as multisets because Go runs items concurrently. Response order and ordinary diagnostics remain exact. The Go module also tests cache capacity, concurrent request identity, completion ordering and callback panic recovery.

## Kafka consumer fixtures

Run `node generate-kafka.mjs` or `npm run fixtures:kafka`; it is also included in postfixtures. The 165 cases execute the actual published CommonJS export of Kafka v2.35.0. Accessor values/errors, handler entry, original metadata, parser calls and diagnostic message order are compared. Undefined and non-finite numbers retain explicit tags. The installed protobufjs 7.5.4 cannot satisfy the reference ESM named BufferReader import; the supported CommonJS export reaches decoder logic without patching upstream. Separate optional adapter corpora verify binary decoding; the core corpus retains missing-schema timing cases. See `kafka/testdata/README.md`.

Kafka binary generators are `node generate-kafka-avro.mjs` (370 scenarios) and `node generate-kafka-protobuf.mjs` (220 prefix scenarios plus nine real proto2 message/descriptor cases). Both run through the pinned public CommonJS consumer and are included in postfixtures. Full values and error messages are compared; byte/non-finite tags retain type information. Protobuf prefix cases preserve decoder calls and shared preference order, while native cases use a binary FileDescriptorSet from the actual reference schema. See each adapter's testdata/README.md for scope and exclusions.

Run `node generate-kafka-modes.mjs` for 172 complete mixed SOURCE/JSON events in integration/kafkamodes/testdata. The development module compares the pinned consumer with direct Go wrapping and the real Go Lambda SDK handler invocation, keeping cross-codec test dependencies out of public modules. Both source types, registry metadata, field combinations, parser calls, null/empty/missing fields and full errors are covered. These are constructed events, not service captures.

## Data Masking fixtures

Run `node generate-datamasking.mjs` for 240 cases from the actual v2.35.0 DataMasking export. Erasure values, errors, diagnostics and provider arguments are compared. Calls to the non-cryptographic deterministic provider are compared as multisets to allow Go scheduling; JSON value and key-order contracts remain checked. No ciphertext interoperability or built-in regex equivalence is claimed. See datamasking/testdata/README.md.

Shared regex: run node generate-regex.mjs for 7,671 Node replacement cases, 19 invalid patterns, Unicode 16 legacy folding data and 30 actual Data Masking composition cases. UTF-16 unit arrays preserve lone surrogates. This does not claim complete ECMAScript parity.

KMS masking: generate-datamasking-kms.mjs records 39 actual encrypted-message cases and five rejection cases with the TypeScript v2.35.0 provider and @aws-crypto/client-node v5.0.2. Its --verify-go mode consumes the existing fixture and a Linux Go bridge, then verifies Go ciphertexts in TypeScript. Local KMS wrapping blobs contain plaintext test keys; they are test artifacts, not a KMS security implementation.
