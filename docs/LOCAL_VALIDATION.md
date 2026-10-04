# Local Docker validation

Logger source parity acceptance (2026-10-04): fixed separate child attribute stores,
shallow empty-field cleanup and active-trace buffer lifecycle from issues #3/#4/#5.
The corpus executes 48 actual TypeScript v2.35.0 scenarios; native tests cover
typed/nil values, formatter ownership, sampled/unsampled OTel contexts, output
failures and 40 concurrent parent/child invocations. Combined packaged acceptance
passed all 31 modules and 28 standalone consumers with CGO disabled and GOWORK
off. The initial workspace check verified 30 modules before an integration
dependency classification failure; the corrected integration uses the existing
Tracer extraction API. Final Logger and integration checkpoints complete the
scope without repeating unchanged modules. See
[MODULE_ACCEPTANCE_LOGGER.json](MODULE_ACCEPTANCE_LOGGER.json).

Both Linux amd64/arm64 normal and streaming handlers were rebuilt. A constructor
warning from the image's `TZ=:/etc/localtime` interfered with probe record counts;
the fixture now explicitly uses UTC. Runtime-only acceptance reused the completed
binaries and passed 889/889 RIE assertions, including 21 new Logger assertions,
95/95 streaming and 14/14 saved Batch checks. Docker executed amd64; arm64 was
cross-compiled. Containers and networks were cleaned without errors. No AWS
resources were used. Successful runtime evidence accurately records reused
module checks and builds; complete Logger configuration/serialization parity
remains open.

Documentation and feature audit (2026-10-01): compared seventeen official TypeScript v2.35.0 guides with Go contracts and restructured eighteen Go guides, including Commons. Verified nineteen complete programs by compilation and eighteen local programs/direct handler executions; the DynamoDB Idempotency entry is compile-only, with persistence covered by the maintained fixture. All 31 modules and 28 public consumers passed packaged acceptance with CGO disabled and GOWORK off. Dependency download failures required phased continuation; passed modules were reused, and tested Go source bytes match current files. Both Linux builds and 868/868 amd64 RIE, 95/95 streaming and 14/14 saved Batch checks passed, with cleanup. No AWS resources were used. [DOCUMENTATION_ACCEPTANCE.json](DOCUMENTATION_ACCEPTANCE.json) records the combined scope; [LOCAL_ACCEPTANCE.json](LOCAL_ACCEPTANCE.json), [STREAMING_ACCEPTANCE.json](STREAMING_ACCEPTANCE.json) and [BATCH_ACCEPTANCE.json](BATCH_ACCEPTANCE.json) identify runtime evidence. Browser review and exhaustive parity remain open.

## Optional AWS Encryption SDK acceptance (2026-09-24)

Implemented datamasking/kms as an independent, uncached provider backed by the official Go Encryption SDK and Materials Providers Library v0.4.0. It reuses Commons Base64/UTF-8/key ordering and AWS request identity; plain masking remains independent of encryption dependencies.

The integration corpus verifies 39 real TypeScript encrypted messages, five malformed/tampered rejection cases, Base64 variants, secondary-key decryption, 32 concurrent operations, input ownership, uncached request counts and cancellation/deadline/context propagation. The reverse bridge passes 108 assertions with the actual pinned TypeScript provider. Local wrapping fixtures contain plaintext test data keys and establish encrypted-message interoperability, not real KMS wrapping or authorization.

The official SDK dependency cannot compile natively on Windows because it references Unix syscall APIs. The module verifier propagates a Linux target to consumers and uses local Go cross-compilation plus the pinned Lambda Docker image for tests. No upstream files or module caches were patched. The first focused Linux run exposed a test-server cleanup issue; consuming the HTTP request body and explicitly testing server-observed cancellation fixed that fixture.

The scoped packaged run checkpoint-06 passes KMS, integration and tools tests/vet/tidy and the independent Linux consumer. MODULE_ACCEPTANCE_KMS.json combines this with previous accepted module checkpoints: 31 modules and 28 public consumers, with every current non-documentation file checked against its tested archive. This is combined evidence, not a repeated full run. MODULE_ACCEPTANCE_SCOPED.json now records this three-module continuation; older sessions retain their own progress.json.

The runtime command uses --skip-module-checks and builds both Linux amd64/arm64 binaries with CGO_ENABLED=0. It passes 868/868 RIE assertions and 95/95 streaming checks; saved Batch artifacts pass 14/14. Twelve added assertions cover encrypted round trips and ownership, fresh uncached data keys, authenticated-context rejection and SDK identity. Docker executes amd64; arm64 is cross-compiled only. Temporary resources were cleaned without errors. No AWS resources were used.

Data-key caching and thresholds, broader key/algorithm/error/native parity, actual KMS service acceptance and performance remain open. See DATAMASKING_KMS.md and DATAMASKING_PLAN.md.

## Shared regex and Data Masking replacement acceptance (2026-09-23)

Extracted the existing Validation translators and unchanged Unicode 16.0 property data into the optional commons/regex module. Validation delegates both payload matching and strict property overlap checks, while Data Masking uses its existing replacement callback. Root Commons and plain masking retain dependency isolation.

Verified 7,671 Node v22.21.1 replacement cases, 19 invalid pattern/flag cases, 30 actual TypeScript v2.35.0 Data Masking compositions and all 23,071 existing Validation cases. Added native concurrency and operational-error tests. Differential cases exposed and corrected multiline Unicode line terminators, legacy case canonicalization, word boundaries and backend-only identity escapes.

The initial full packaged run accepted 24 modules before Validation failed to compile: schema_path.go still referenced two private character predicates from the extracted regex file. Its URI encoding now owns the equivalent inline character test. A continuation checked the six affected/remaining modules and four consumers, without rerunning already accepted modules. Combined acceptance covers 30 modules and 27 standalone consumers, with GOWORK=off, tests/vet/tidy and dependency isolation. MODULE_ACCEPTANCE_REGEX.json records sessions checkpoint-10 and checkpoint-01 and verifies every current non-documentation file against its tested archive. Documentation-only differences are enumerated. MODULE_ACCEPTANCE.json remains the previous uninterrupted full run; The six-module continuation is preserved in its session progress.json; MODULE_ACCEPTANCE_SCOPED.json now records the latest scoped run.

The runtime command reused these checks with --skip-module-checks, rebuilt normal/streaming handlers for Linux amd64 and arm64 with CGO_ENABLED=0, and passed 856/856 RIE assertions plus 95/95 real-Go-SDK/local-Runtime-API streaming checks. Saved Batch artifacts pass 14/14. Nine added runtime assertions cover named Unicode captures, global state reset and a sticky offset inside a surrogate pair. The report correctly distinguishes reused module checks from executed builds. Docker executed amd64; arm64 was cross-compiled. Temporary runtime resources were cleaned, and no AWS resources were used.

Unicode sets (v), advanced syntax/case/property boundaries, native JSON preservation of lone surrogates, exact diagnostics and performance remain open. At this historical milestone the AWS Encryption SDK provider and caching interoperability were not implemented; the later uncached-provider acceptance is recorded above. See REGEX.md and DATAMASKING_PLAN.md.

## Data Masking core acceptance (2026-09-23)

Implemented the independent default/dynamic/custom erasure and provider-orchestration module. Its 240 actual TypeScript cases compare complete values/errors/warnings and provider calls, with an additional async-failure timeline. Native tests cover 64 concurrent erasures, ownership, context, concurrent providers, first-rejection timing, error identity and panic propagation. The core has no third-party module dependencies; Commons supplies number parsing and object-key order.

The complete module check passed all 29 packaged modules and 26 standalone consumers: tests/vet/tidy, GOWORK=off and dependency isolation. Session checkpoint-02. During that run, source review corrected provider first-rejection timing. The final Data Masking code was then checked separately in checkpoint-03; all eight files match its accepted archive and the independent consumer has no external modules. Other modules retain the complete-run evidence. Forty-five integration archive files match current sources; only the Python runner differs due to the restored assertion text.

Normal and streaming handlers were rebuilt for Linux amd64/arm64 with CGO_ENABLED=0.

A runtime-only retry reused the built binaries and passed 847/847 RIE, 95/95 real-Go-SDK/local-Runtime-API streaming and 14/14 saved Batch checks. Its report correctly records that module checks and builds were reused. Docker executed amd64; arm64 was cross-compiled. Temporary containers/networks were cleaned without errors and no AWS resources were used.

Masking composition verifies rule precedence, caller input ownership, reversible provider round trips including null values, invocation/authenticated context, warning policy and missing-provider errors. The fixture provider is explicitly non-cryptographic. Built-in ECMAScript regex, AWS Encryption SDK interoperability, data-key caching, exhaustive native compatibility and performance remain open in DATAMASKING_PLAN.md.

## Previous Kafka mixed event modes and Protobuf metadata acceptance (2026-09-23)

Verified 172 constructed SOURCE/JSON events directly and through the native Go Lambda SDK: both sources, Glue/Confluent metadata, text/JSON/Avro/Protobuf key/value combinations, repeated lazy reads, parser calls, original fields and null/empty/missing values. Complete reference errors are compared. A separate regression records aws-lambda-go v1.55.0 KafkaRecord's loss of schema metadata and field presence; the RawMessage wrapper preserves both.

Fixed Protobuf schema ID selection for JSON objects with a numerically coerced length. The implementation reuses Commons ParseNumber and retains explicit cyclic/prototype limits. Its additional 97 reference cases bring the prefix corpus to 220, alongside nine native message cases. See KAFKA_MODES.md and KAFKA_BINARY.md.

Only the affected Protobuf and integration modules were repackaged and checked, reusing the previous complete 28-module/25-consumer acceptance for unchanged modules. Tests/vet/tidy and the independent Protobuf consumer passed. Session checkpoint-05 matches all six Protobuf and 45 integration archive files, including the mixed-event corpus and native SDK tests. MODULE_ACCEPTANCE_SCOPED.json records this narrower scope.

The runtime runner reused those completed module checks, rebuilt normal and streaming handlers for Linux amd64/arm64 with CGO_ENABLED=0, and passed 829/829 RIE assertions, 95/95 streaming checks and 14/14 saved Batch checks. Three new runtime assertions exercise a string-valued object length selecting the correct decoder path. Docker executed amd64; arm64 was cross-compiled. The report correctly records reused module checks and executed builds. Runtime resources were cleaned without errors; no AWS resources were used.

These events are constructed fixtures, not service captures. Actual registry delivery/framing removal/retries, complete native schema/error/serialization compatibility and performance remain open.

## Previous Kafka binary adapter acceptance (2026-09-23)

The full run passed all 28 packaged modules and 25 standalone consumers with GOWORK=off, tests/vet/tidy and dependency isolation. Session: checkpoint-08. Both optional adapters match all six files in their accepted archives. Avro isolates hamba/avro; Protobuf isolates the official Go Protobuf library; Kafka core has no third-party module dependencies.

Normal and streaming Lambda handlers were built for Linux amd64/arm64 with CGO_ENABLED=0. Docker amd64 acceptance passed 826/826 RIE assertions, 95/95 real-Go-SDK/local-Runtime-API streaming checks and 14/14 saved Batch checks. Module checks and builds executed in the passing run. Disposable containers/networks were cleaned without errors; no AWS resources were used.

The 18 new runtime assertions exercise Avro records/tagged unions/bytes and precision rejection, native Protobuf plain/Glue/Confluent messages, adaptive index fallback and null-metadata errors across three successful invocations. The adapter corpora contain 370 Avro, 123 Protobuf prefix and nine native message scenarios.

After that run, 22 constructed JSON-mode event scenarios expanded the core corpus to 165. Packaged core tests/vet/tidy and an independent consumer check passed in checkpoint-09; its ten archive files match current core source. Only the generator, core reference test/corpus and documentation changed after the full runtime run, so runtime checks were not repeated. That historical scoped result is retained in its session progress.json; MODULE_ACCEPTANCE_SCOPED.json tracks the latest affected-module check. See KAFKA_MODES.md. Full SOURCE/service, schema/native/error compatibility and performance gates remain open.

## Previous Kafka consumer acceptance (2026-09-23)

The independent primitive/JSON Kafka consumer passes 143 actual TypeScript v2.35.0 CommonJS scenarios and 64 concurrent contexts. Verified lazy/repeated access, parser transforms/issues/exception identity, strict Base64/UTF-16 length, UTF-8/BOM differences, original metadata and ordered flattening. The core only depends on Commons; binary codec adapters were not part of that milestone.

The complete run verified all 26 packaged modules and 23 standalone consumers, tests/vet/tidy/isolation with GOWORK=off, and built Linux amd64/arm64 normal and streaming Lambda executables with CGO_ENABLED=0. Session: checkpoint-07. All ten Kafka archive files match current source; the standalone Kafka dependency graph has no external modules.

A runtime-only retry passed 808/808 RIE assertions and 95/95 real-Go-SDK/local-Runtime-API streaming checks using the same built binaries. Saved artifacts also pass 14/14 Batch checks. The successful runtime report accurately records that module checks/builds were reused. This corrects Python acceptance bookkeeping only; no library or Go integration source changed after the successful packaged module checks.

Kafka composition verifies Parser rejection before persistence, one execution for duplicate records, context correlation through Logger/OTel, topic order, self-managed metadata, tombstones, empty keys and UTF-8 headers. Docker executed amd64; arm64 was cross-compiled. Containers/networks were cleaned with no cleanup errors. No AWS resources, public release, full binary decoding parity or performance acceptance is claimed.

Previous Bedrock milestone (2026-09-23, Asia/Shanghai): **790/790 RIE assertions** passed across five Lambda invocations, including 24 new Bedrock function resolver checks. All **25 packaged modules** passed tests/vet/tidy with GOWORK off, and **22 public consumers** passed independent builds/dependency checks. Bedrock's 371 actual TypeScript scenarios compare exact body strings; native tests additionally verify 64 concurrent contexts, ownership and errors. Both Bedrock and corrected GraphQL archives match every current module file. Normal and streaming handlers were built for Linux amd64/arm64 with CGO disabled; Docker executed amd64. The separate streaming suite passed **95/95**, and saved Batch artifacts passed **14/14**. Temporary runtime resources were cleaned; no AWS resources were used. Module checks and builds executed in the passing run, and the final module checkpoint matches its complete report. Full compatibility, service and performance gates remain open.

Bedrock checks cover parameter conversion with concrete Parser validation, Lambda/Logger/OTel context, inherited session and knowledge-base fields, isolated explicit REPROMPT responses, missing tools, typed execution errors, malformed events, empty results and JSON-quoted strings.

## Previous AppSync GraphQL milestone

GraphQL milestone (2026-09-22, Asia/Shanghai): **766/766 RIE assertions** passed across five Lambda invocations, including 24 new AppSync GraphQL composition checks. All **24 packaged modules** passed tests/vet/tidy with GOWORK off; **21 public consumers** passed independent builds and dependency checks. GraphQL's only module dependency is root Commons. Its 114 TypeScript resolver scenarios and 91 scalar cases passed, and the verified ZIP matched all module files at that milestone. Both normal and streaming handlers were cross-compiled for Linux amd64/arm64 with CGO disabled; Docker executed amd64. The separate real-Go-SDK/local-Runtime-API streaming suite passed **95/95**, and saved artifacts passed **14/14 Batch checks**. Temporary containers/network were cleaned and no AWS resources were used. Module checks and builds executed in that run. Full type/service/performance/release compatibility remains open.

GraphQL checks cover Parser validation, request/Logger/OTel identity, first-event aggregate routing with an intentionally shorter result, sequential graceful batches, first-error abort, included exception handlers, framework errors/invalid input and date-time scalar output.

## Previous AppSync Events milestone

Date: 2026-09-22 (Asia/Shanghai). Result: **742/742 scoped assertions passed across five Lambda emulator invocations**, with all disposable containers/network cleaned. AppSync Events adds eighteen assertions for Parser validation, ordered items, Logger/OTel/request context, aggregate precedence, passthrough and authorization. Multi-instance wrappers retain eighteen assertions for publication order, defaults, cold-start composition, strict propagation and scope closure. Timestamps retain eighteen assertions for invalid numeric/Date inputs, inclusive windows, lazy/reset clock reads and late writes. Metric values retain fifteen assertions for non-finite JSON, numeric definition order, reserved-envelope precedence, prototype-name behavior and scope cleanup. Prior configuration assertions remain passing. The prior twelve cold-start, fifteen diagnostic and nine store assertions remain passing. Prior HTTP EMF/OTel isolation checks remain passing. The separate native-Go-SDK/local-Runtime-API suite passed **95/95** across ten streaming invocations. **14/14 Batch checks** reuse saved RIE artifacts. No AWS resources or credentials were used.

All **23 packaged modules** passed tests/vet/tidy with CGO disabled and GOWORK off; **20 public consumers** passed independent builds/dependency checks. Commons, Parser, AppSync Events, HTTP core and HTTP Metrics have no third-party module dependencies; the new OTel adapter excludes the retired SDK. Metrics contributes 104 public store lifecycle scenarios, 203 exact diagnostic scenarios, 302 cold-start/name-precedence scenarios, 532 constructor/configuration scenarios, 641 value/error/key scenarios, 968 numeric/Date timestamp scenarios and 876 middleware-hook scenarios. AppSync Events contributes 100 scenarios. HTTP contributes 2,066 scoped cases (1,762 core, 176 Metrics and 128 Tracer middleware contract), Parser 3,493 and Validation 23,071. The Tracer reference uses actual middleware with a recording contract, not legacy SDK acceptance.

Both normal and streaming handlers were built for Linux amd64/arm64; Docker executed amd64 only. This completed run executed all module checks and builds before Docker acceptance; LOCAL_ACCEPTANCE.json records both flags as true. The previous HTTP observability milestone required a runtime-only continuation after starting Docker; those historical results are retained in VALIDATION.md. One earlier attempt stopped in module tests because a new test incorrectly expected Content-Length after compression. Source inspection confirmed the header is removed; corrected tests now verify absence and actual retained lengths without removing fields from the reference comparison. Those corrections belong to the earlier observability milestone. The AppSync Events runtime run passed once. Initial focused fixtures corrected final-line route matching and Unicode separator byte measurement before that run. The full-run AppSync archive preceded the concrete UnauthorizedException type-name correction. MODULE_ACCEPTANCE_SCOPED.json separately verifies the final AppSync source archive, tests/vet/tidy and standalone consumer; the Lambda binaries were built from the corrected source. Unaffected module suites and runtime invocations were not repeated. The Lambda fixture now explicitly sets on-demand initialization, matching the supported cold-start contract. Full parity, cloud service behavior, performance and release gates remain open.

Verified behavior:

Streaming is additionally verified by the separate [95-check Runtime API report](STREAMING_ACCEPTANCE.json): incremental delivery, metadata framing, read/close/panic/deadline error trailers, HTTP and pre-output invocation errors, warm recovery, exactly-once cleanup and Logger/OTel correlation. The current packaged verification covers all 23 modules and twenty consumers. See [HTTP_STREAMING.md](HTTP_STREAMING.md) for the distinction between local SDK transport evidence and cloud integration acceptance.

- Initial and warm success, returned error, an unsampled parent, and panic.
- Runtime request identity, cold-start state, warm process reuse, temporary attribute isolation, and correlated logs.
- Buffer discard on success and flush on error/panic.
- Actual OTLP/HTTP protobuf export from the default private OTel provider.
- Handler/business span hierarchy, supplied parent identity, annotations, and error status.
- AWS SDK v2 and HTTP client spans and downstream propagation against deterministic local fixtures.
- No Logger or Tracer instrumentation error callbacks during the run.
- Five isolated application EMF documents and one separate ColdStart metric, including namespace/dimensions and flush after error/panic; no Metrics instrumentation failure diagnostics.
- All five Parameters providers: SSM, Secrets Manager, DynamoDB, AppConfig Data, and AppConfig Agent. Verify decoded values, warm cache reuse, forced refresh, one AppConfig session with rotating tokens and an unchanged response, and one Agent request per successful retrieval invocation.
- Metadata response fields, bearer authentication, warm cache reuse, and explicit clearing against a local HTTP fixture. Unknown fields remain intact.
- Exactly one Powertools SDK identity marker on composed AWS requests while preserving the AWS SDK's user agent.
- DynamoDB-backed Idempotency warm replay, duplicate record suppression, deletion after business failure, successful retry with original Batch identifiers, changed-payload rejection, and isolated record logs/spans. The fixture retains seven completed records after 18 acquisitions, seven completions, and three deletions. Standalone adapter tests verify the Idempotency marker; the composed client retains the earlier Tracer marker.
- Real Valkey executes 32 concurrent duplicate attempts per successful invocation while one handler owns the key, recovers orphaned records, retains completed validation hashes, and replays warm responses. The actual TypeScript cache adapter writes a record consumed by Go and reads a Go-written record without running its business callback. These bridge scenarios use JSON values without payload validation; the pinned TypeScript validation omission remains a documented boundary.

Metrics unit tests additionally verify 64 concurrent cold-start captures, consumed failed captures, scoped function-name isolation, diagnostic callback reentrancy, 64 concurrent diagnostic scopes, automatic-flush/failed-output delivery, 100 concurrent request scopes, closed-scope rejection, validation, output failures, and eight documents from four actual TypeScript reference scenarios. Parameters tests execute actual SDK requests against loopback fixtures, compare pinned TypeScript cache/transform/batch outputs, and verify defaults, writes, pagination, missing/errors, cancellation, and functional concurrency. The final complete suite includes the Logger sampling fixture. Real CloudWatch metric ingestion and live Parameters service behavior remain unverified.

Logger regression tests also verify pointer-descendant redaction without duplicate property callbacks, caller-data immutability, pointer cycles, formatter replacement/failure, UTC/Hong Kong/New York timestamps, and stdout/stderr level routing. The pointer traversal defect is fixed; complete JavaScript serialization and configuration parity remain open.

Commons fixtures additionally cover environment/runtime parsing, 50 Base64 cases, indexed deep merge, LRU eviction, type helpers, DynamoDB raw values and large integer precision, and Metadata request/cache behavior. Consumer tests verify migrated configuration modes, Logger indexed merge and parent/child isolation, and Parameters configuration validation. Functional concurrency tests exercise 100 LRU users, 100 Metadata callers, cache invalidation during fetch, and waiter timeouts. Current artifact hashes correspond to the multi-module implementation, including the prior Metadata cancellation fix.

Historical Parser union milestone: the completed command ran full module verification, both architecture builds and all 292 runtime assertions. The focused Parser run passed all 3,493 differential cases plus mode/isolation/cancellation tests. That run verified all examples and new runtime probes from independently packaged sources. Union errors retain branch-relative paths, refinements continue only after eligible check failures, and JSON/nullable/SQS composition preserves ordinary versus safe aggregation. The prior S3 union workaround now uses the shared union implementation. Dependency checks confirm no SDK requirement in root Commons or Parser. Its summaries and fourteen Batch checks reused the same runtime artifacts without additional invocations. No cloud resources were created.

The local response fixtures are not DynamoDB Local, LMDS, or real AWS services. Parent headers are explicitly supplied by the fixture extractor. The emulator returned HTTP 502 with `Runtime.ExitError` for panic; the original panic was separately verified in runtime logs, and the error spans were exported before process exit. This result does not verify IAM, real service/LMDS semantics, native AWS parent injection, freeze/thaw, hard timeouts, or collector-to-X-Ray delivery.

Reproduce with `uv run python integration/local/run.py`; see [runner documentation](LOCAL_INTEGRATION.md). Raw evidence remains in ignored `dist/local/report.json`, `dist/local/otlp.json`, and `dist/local/lambda.log`. [LOCAL_ACCEPTANCE.json](LOCAL_ACCEPTANCE.json) preserves assertion results and artifact hashes.

The runner uses sequential package compilation, a low Go heap target, and build/cache paths under D: to limit local resource pressure. Node.js is used to regenerate reference fixtures and execute the local TypeScript/Go cache bridge; the Lambda executable and ordinary Go module tests do not require it.
