# Independent modules in one repository

The implementation uses twenty-eight public modules and three development modules. Current module/import paths use the selected rambow-cloud repository namespace. Each module has its own `go.mod`, dependency requirements, and version entry in `tools/modules.json`. Versions are initial release targets, not published releases. Go 1.26 is the current supported baseline for all modules; CGO is always disabled. The count includes the deprecated, frozen X-Ray adapter. Current namespace verification is recorded in [MODULE_ACCEPTANCE_MIGRATION.json](MODULE_ACCEPTANCE_MIGRATION.json): all 31 packaged modules and 28 independent public consumers passed with GOWORK=off and CGO disabled on 2026-09-27. The report combines 13 accepted modules with an 18-module continuation. Non-documentation source matched the accepted archives at verification; later publication-only changes are checked separately. Earlier MODULE_ACCEPTANCE, REGEX and KMS records retain their original verification scope with historical module directories identified independently of repository ownership.

## Module boundaries

All module paths start with `github.com/rambow-cloud/powertools-lambda-go`.

| Directory | Responsibility | Initial tag |
| --- | --- | --- |
| `.` | Dependency-free Commons, shared invocation identity, and internal fixture helpers | `v0.1.0` |
| `commons/awssdk` | Smithy request identity adapter | `commons/awssdk/v0.1.0` |
| `commons/dynamodb` | Native SDK DynamoDB conversion; raw helpers delegate to root Commons | `commons/dynamodb/v0.1.0` |
| `commons/metadata` | Optional Metadata HTTP client | `commons/metadata/v0.1.0` |
| `commons/regex` | Optional shared ECMAScript matching and replacement | `commons/regex/v0.1.0` |
| `logger` | Structured logging | `logger/v0.1.0` |
| `metrics` | EMF metrics | `metrics/v0.1.0` |
| `parameters` | Cache/transforms, defaults, and five providers | `parameters/v0.1.0` |
| `signer` | Standalone SigV4 signing and HTTP transport | `signer/v0.1.0` |
| `jmespath` | Query engine, custom functions, and envelopes | `jmespath/v0.1.0` |
| `datamasking` | Field erasure, ordered rules and pluggable provider orchestration | `datamasking/v0.1.0` |
| `datamasking/kms` | Optional AWS Encryption SDK masking provider; Linux verification | `datamasking/kms/v0.1.0` |
| `kafka` | Lazy Kafka primitive/JSON consumer, headers, metadata and parser/codec boundaries | `kafka/v0.1.0` |
| `kafka/avro` | Optional Avro schema and binary decoder | `kafka/avro/v0.1.0` |
| `kafka/protobuf` | Optional Protobuf callback/descriptor and registry-prefix adapter | `kafka/protobuf/v0.1.0` |
| `batch` | Record processing, FIFO ordering, partial responses, and typed Lambda wrappers | `batch/v0.1.0` |
| `idempotency` | Operation lifecycle, key extraction, local response caching, and DynamoDB persistence | `idempotency/v0.1.0` |
| `idempotency/cache` | Optional Redis/Valkey persistence and client dependency | `idempotency/cache/v0.1.0` |
| `parser` | Schema composition, typed parsing and Lambda envelopes | `parser/v0.1.0` |
| `validation` | JSON Schema compilation, manual validation and typed input/output wrappers | `validation/v0.1.0` |
| `eventhandler/appsyncevents` | AppSync Events publish/subscribe routing, errors and size diagnostics | `eventhandler/appsyncevents/v0.1.0` |
| `eventhandler/appsyncgraphql` | AppSync GraphQL single/batch routing, exceptions and scalar helpers | `eventhandler/appsyncgraphql/v0.1.0` |
| `eventhandler/bedrock` | Bedrock Agent function routing, parameter conversion and response envelopes | `eventhandler/bedrock/v0.1.0` |
| `eventhandler/http` | HTTP event adaptation, routes, middleware and optional schema callbacks | `eventhandler/http/v0.1.0` |
| `eventhandler/http/metrics` | Optional request-scoped EMF middleware | `eventhandler/http/metrics/v0.1.0` |
| `eventhandler/http/tracer` | Optional OTel route spans and HTTP metadata | `eventhandler/http/tracer/v0.1.0` |
| `tracer` | OpenTelemetry tracing and instrumentation | `tracer/v0.1.0` |
| `tracer/xray` | Deprecated and frozen legacy SDK adapter | `tracer/xray/v0.1.0` (not recommended for new releases) |
| `examples` | Development examples | Not published |
| `integration` | Lambda and local capture fixtures | Not published |
| `tools` | Packaging and development tooling | Not published |

Parameters providers remain packages within the Parameters module. Their shared cache and lifecycle implementation stay together, including `parameters/internal/parameterdefaults`. Signer depends on the AWS SDK's core signing package without AWS config loading or OTel. JMESPath depends on the query engine and dependency-free Commons. Logger's query interface imposes no query module dependency. Parser depends only on root Commons; its initial schemas and envelopes share the Parser module. Batch and Idempotency compose with Parser through application callbacks without acquiring its dependency. Subsequent utilities should receive their own modules when implemented.

Root Commons must not import feature modules, including through its tests. DynamoDB tests and Metadata tests own their fixtures inside their respective module archives. The generator writes these fixture subsets from the same upstream execution. Shared fixture normalization remains dependency-free in `internal/testfixture`.

The raw DynamoDB decoder and shared numeric conversion live in root `commons`; the existing `commons/dynamodb` module retains SDK-dependent adapters and API delegates. Parser can decode DynamoDB/Kinesis stream images without importing the SDK module. This extraction adds no module or third-party requirement.

Logger, Metrics, and Tracer use the same root `internal/invocation` package. Its context key and cold-start state are not copied between modules. Sibling module paths remain under the permitted parent import prefix.

## Development

`go.work` lists all modules for local editing and example builds. It also maps the required unpublished internal versions to their local directories, because workspace graph loading can still request their versioned metadata. Keep these development-only mappings in `go.work`; do not add filesystem `replace` directives to `go.mod` files. When introducing another required internal version, update the corresponding workspace mapping. Root `go test ./...` only covers the root module, so use the repository tooling for complete verification:

```powershell
$env:CGO_ENABLED = '0'
uv run python tools/modules.py list
uv run python tools/modules.py check
```

`check` constructs a local file-based Go module proxy, packages every module without nested module source or ignored build/cache directories, and verifies copies extracted from those archives with `GOWORK=off`. It checks dependency-file consistency, tests, and vet for all modules. Each public module also gets a standalone consumer build and dependency isolation checks. Results are written to `docs/MODULE_ACCEPTANCE.json`.

The proxy uses initial target versions only as local fixtures. It does not create Git tags, push code, or test the public Go proxy. A fresh module cache under `dist/modules` prevents these synthetic versions from polluting the normal cache. Existing cached third-party downloads are reused as a fallback before accessing the public Go proxy. Third-party checksum verification stays enabled. Internal fixture checksums are excluded from committed `go.sum` files because source edits change unpublished archives; the isolated check resolves and verifies their checksums in its temporary consumers.

Before starting a tidy/check session, the tool requires at least 2 GiB of free workspace disk space. This is an early guard, not a guaranteed peak-space budget. Historical `dist/modules/check-*` and `tidy-*` directories contain reproducible generated copies and downloads; cleanup remains a separately reviewed action. The verifier never deletes them automatically. During verification, `dist/modules/check-*/progress.json` is atomically updated after each completed module. Its `completed: false` state distinguishes partial evidence from full acceptance. Only a completely successful run replaces the applicable report in `docs/`. Completed reports use `completed: true` and the same module records as the session checkpoint. Python subprocess decoding explicitly uses UTF-8 for Go diagnostics on Windows.

Consumers are seeded with pinned external hashes to avoid redundant checksum lookups; module content is still verified. A generated, ignored `dist/go.mod` prevents Go from scanning downloaded caches as root-module source. This build-artifact boundary is not a library or release module. Development module graphs are checked too; the maintained integration module must not reintroduce the frozen X-Ray SDK.

To update requirements after changing imports:

```powershell
$env:CGO_ENABLED = '0'
uv run python tools/modules.py tidy
```

The command resolves internal modules in dependency order and preserves external requirements/checksums. Older internal requirements may resolve to their existing published versions; modules are not forced to upgrade together. Before any public release exists, internal requirements must name a version available in the local fixture set.

For the complete local Lambda acceptance flow, including module checks, dual-architecture cross-builds, and Docker execution:

```powershell
uv run python integration/local/run.py
```

Run either the full flow or standalone module checks as appropriate; avoid running both consecutively without a relevant change. CI runs the module check and builds both Lambda architectures.

## Independent releases

Use [Releasing modules](RELEASING.md) to accumulate PR summaries, review a
per-module plan, and explicitly dispatch public tags and Releases. Local
fixture verification does not publish or verify public versions.

The module verifier keeps each writable module cache isolated. Its local fixture proxy takes precedence, followed by existing download caches from previous verification runs, and finally the public Go proxy. External checksums remain verified; cached synthetic versions never replace newly generated local fixture versions.

For a future Logger-only release, update only Logger's version entry, for example to `v0.1.1`. Leave its Commons requirement at `v0.1.0` if it does not need newer Commons behavior. Its tag will be `logger/v0.1.1`; Metrics and Tracer do not need new tags. Update dependent module requirements only when they need the newer functionality.

After publication, consumers select module versions normally:

```powershell
$env:CGO_ENABLED = '0'
go get github.com/rambow-cloud/powertools-lambda-go/logger@v0.1.0
go get github.com/rambow-cloud/powertools-lambda-go/metrics@v0.1.0
```

```go
import (
    "github.com/rambow-cloud/powertools-lambda-go/logger"
    "github.com/rambow-cloud/powertools-lambda-go/metrics"
)
```

These commands require a published repository and tags. The selected repository path is `github.com/rambow-cloud/powertools-lambda-go`; upload and tags are pending. Project and source-data notices are included in each module. Before first release, finalize the API compatibility scope and release notes. Publish shared dependencies before their consumers and verify retrieval through the real Go proxy. For a module's incompatible v2 release, use its `/v2` module/import suffix and corresponding subdirectory-prefixed tag.

Splitting modules isolates declared dependency graphs; it does not mean every utility is dependency-free. Logger retains the OTel trace API for correlation, Tracer retains its OTel and AWS instrumentation dependencies, and Parameters retains its SDK providers. The legacy X-Ray SDK is confined to the frozen adapter and its own regression tests. The current integration module no longer depends on it. See [XRAY_MIGRATION.md](XRAY_MIGRATION.md).

## Scoped verification

After a relevant source change, `$env:CGO_ENABLED='0'; uv run python tools/modules.py check --only eventhandler/appsyncevents` in PowerShell rebuilds proxy artifacts and runs the same archive tests, vet, tidy and independent-consumer checks for the selected module. Repeat --only for additional affected modules. Dependencies are packaged but are not claimed as tested by that scoped run. The result is saved separately as MODULE_ACCEPTANCE_SCOPED.json and never replaces MODULE_ACCEPTANCE.json. Full-workspace checks remain the default, including workspace/manifest and dependency-isolation validation.

The datamasking/kms module declares test_os=linux because its official SDK dependency cannot compile natively on Windows. The module tool propagates this requirement to dependents, cross-compiles with the local Go toolchain, runs tests through tools/run-linux-test.py in the pinned Lambda Docker image, and builds Linux standalone consumers. Linux hosts execute the tests natively. No module-cache patches or SDK forks are used. See DATAMASKING_KMS.md.
