# Optional utilities and Go publishing

Assessment date: 2026-09-14; publication identity updated 2026-09-27; unified version policy adopted 2026-10-05. Feature modules have independent dependencies and one shared release version in this repository. Module boundaries are implemented; see [MODULES.md](MODULES.md) for the actual layout and maintenance commands. The selected repository path is `github.com/rambow-cloud/powertools-lambda-go`. The 27 maintained public modules were published as `v0.1.0` on 2026-10-04; see [public release acceptance](RELEASE_ACCEPTANCE.json). The frozen X-Ray adapter was excluded.

## Consumer experience

Python's `aws-lambda-powertools[parser]` is installation syntax for optional dependencies, including Pydantic. It does not create a separate Parser distribution or change Python import syntax. TypeScript instead publishes packages such as `@aws-lambda-powertools/parser`.

Go has no bracket-based extras syntax. Consumers import a package path; `go get` selects the module that provides that package. The published Go equivalent is:

```powershell
$env:CGO_ENABLED='0'
go get github.com/rambow-cloud/powertools-lambda-go/parser@v0.1.0
```

```go
import "github.com/rambow-cloud/powertools-lambda-go/parser"
```

The example uses the publicly verified Parser `v0.1.0` release. The existing utilities have separate module/import paths such as `/logger` and `/metrics`; `/parameters/ssm` remains a package within the Parameters module. Lambda applications compile their selected packages into the Go executable; Node.js and Python reference tooling is not a runtime dependency.

## Package selection versus module isolation

| Property | Previous single module | Current separate feature modules in one repository |
| --- | --- | --- |
| Import only Parser in application code | Possible without module isolation | Implemented with a separate Parser module |
| Compile utility packages absent from the application's transitive imports | Not required | Not required |
| Source download boundary | Entire containing module | Selected modules and required dependencies |
| Dependency requirements and minimum Go version | Shared `go.mod` | Each module owns its requirements |
| Release version policy | One version | One shared version across independent modules |
| Version tag for Parser v0.1.0 | `v0.1.0` | `parser/v0.1.0` |
| Maintenance | One release/test boundary | One release cohort with per-module consumer compatibility checks |

A single module does not automatically compile or link every listed dependency into a Lambda binary. Conversely, importing one package does not make the module's dependency metadata, source archive, or version independent. Graph pruning and lazy loading can avoid some unrelated dependency retrieval; they are not an extras mechanism or a guarantee that unrelated dependency metadata is never consulted.

## Selected implementation

The selected structure uses one repository, coarse utility modules, and additional adapter modules where they isolate optional dependencies. It does not turn every helper or source file into a module.

The previous root `go.mod` included Parameters SDK providers, OpenTelemetry exporters, and the legacy X-Ray SDK. The new root module has no third-party requirements. Feature and adapter module files own their respective dependencies. All currently use the existing Go 1.27 baseline.

The split follows these boundaries:

1. Preserve a small shared core containing Commons primitives and the shared invocation implementation. `commons/` and `internal/invocation/` remain in the root module. Metadata, Signer, JMESPath, Parser and existing utilities have separate modules.
2. Separate `commons/awssdk` and `commons/dynamodb` as adapter modules while retaining their paths. SDK-dependent core tests now belong to the DynamoDB adapter, preventing test dependencies from coupling core to the SDK.
3. Keep exactly one shared invocation context key and cold-start state. Feature modules under the existing repository import prefix can still access the root `internal/invocation` package: Go's `internal` rule is based on the parent import path, not simply on module boundaries. Moving it to `commons/internal` would prevent sibling utility imports; copying it would break wrapper composition. The root core must not import feature modules, including through tests.
4. Isolate `tracer/xray` from the OTel Tracer module to keep the deferred legacy SDK optional at the module level. Initially keep the Parameters providers together; consider finer provider modules only if consumer dependency budgets justify them.
5. Move examples and integration programs into development modules so their combined imports do not pull every utility back into the core module. Use `go.work` for local composition. Release verification must also work with `GOWORK=off` and published dependency versions, without local `replace` paths.
6. Release every maintained public module at one shared version, with its required tag (`parser/v0.1.1`, `signer/v0.1.1`, and so on). Publish shared dependencies first and finalize the root unified summary last. A future major v2 requires the corresponding `/v2` module/import suffix. The module split predates public publication; the unified version policy preserves those module paths and boundaries.

Optional parser/schema and Kafka codec integrations should use explicit adapters with small interfaces, not feature build tags or a root package that imports all utilities. Build tags do not offer equivalent dependency isolation because module maintenance considers files across build tags.

## Release checklist

The implemented [release workflow](RELEASING.md) defines release tracking,
per-module PR summaries, automatic version/dependency preparation, reviewed
plans, publication after authorized merges, and manual failure recovery.
Installing it does not close remaining compatibility or real
version-retrieval gates below.

- [x] PUB-00: Compare Python extras, TypeScript package publication, and Go package/module semantics; audit current dependency and invocation boundaries.
- [ ] PUB-01: Finalize the repository owner/path, distribution layout, supported Go versions, and feature dependency budgets before public release.
- [x] PUB-01a: Select github.com/rambow-cloud/powertools-lambda-go and migrate all 31 local modules/import paths. All 31 packaged modules and 28 independent public consumers pass with CGO disabled, alongside fresh basic Lambda static builds/ZIPs for both architectures. See [migration evidence](MODULE_ACCEPTANCE_MIGRATION.json). Remote upload and real version retrieval remain open.
- [x] PUB-02: Implement nine public and three development module boundaries and the workspace, preserving import paths and shared invocation identity. Verified all packaged module tests/vet and 100 Docker assertions; see [MODULE_ACCEPTANCE.json](MODULE_ACCEPTANCE.json) and [LOCAL_ACCEPTANCE.json](LOCAL_ACCEPTANCE.json).
- [x] PUB-03: Configure per-module CGO-disabled tests/vet and Lambda cross-builds; verify standalone Parser and Signer consumers. All 18 packaged modules and 15 public consumers passed; Parser has no external dependencies. Dependency graphs and consumer binary sizes are recorded in MODULE_ACCEPTANCE.json, with both Lambda architectures built (2026-09-15).
- [x] PUB-03a: Complete the original module-split portion of PUB-03: CI definitions, 12 packaged-module checks, nine isolated consumer builds/dependency checks, and Linux amd64/arm64 builds with CGO disabled. Subsequent utilities including Parser extend verification to 18 modules and 15 independent consumers (2026-09-15). Consumer harness sizes are recorded; release performance benchmarks remain open.
- [x] PUB-03c: Verify optional HTTP Metrics/OTel adapter modules without adding dependencies to HTTP core. Verified 176 Metrics and 128 Tracer middleware reference cases (2,066 HTTP cases across the three modules), scope/span concurrency and body lifecycle tests, all 22 packaged modules/19 independent consumers, both CGO-disabled Linux builds, 622/622 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22, Asia/Shanghai). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used. Versions remain local fixtures, not public releases.
- [x] PUB-04: Verify all 27 maintained public consumers with real `v0.1.0` requirements, GOWORK=off, CGO disabled, fresh caches, public proxy/checksum retrieval, and no module replacements. Hosted module CI also passed before publication. See [RELEASE_ACCEPTANCE.json](RELEASE_ACCEPTANCE.json) (2026-10-04).
- [x] PUB-03b: Extend the independent-module harness to 20 packaged modules and seventeen public consumers, including HTTP. Verify tests/vet/tidy, both CGO-disabled Linux builds and 501/501 Docker assertions (2026-09-16). Root Commons, Parser and HTTP have no third-party module dependencies. These checks use local fixture versions; public publication/retrieval remains open.
- [x] PUB-04a: Verify GOWORK=off consumers through a local file-based module proxy with semver requirements and no go.mod replacements. Root Commons has no external dependencies; Metrics and Metadata only require core; OTel Tracer excludes the X-Ray SDK. Public repository/proxy retrieval remains pending under PUB-04.
- [x] PUB-05a: Add the project MIT license and preserved upstream/source-data licenses, record direct dependency provenance, and verify LICENSE/NOTICE contents in all 31 independent module archives. CI verifies generated notices; see [THIRD_PARTY_NOTICES.md](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/THIRD_PARTY_NOTICES.md). This does not claim a complete binary SBOM or public release.
- [ ] PUB-05: Complete license/provenance notices, API docs, release notes, module tag automation, and public compatibility scope.
- [x] PUB-06: Publish all 27 approved maintained modules in dependency order through GoReleaser OSS v2.18.2 and verify independent public consumers before finalizing each Release. Preparation PR #16 froze the source and notes; issue #15 completed after successful publication. See [RELEASE_ACCEPTANCE.json](RELEASE_ACCEPTANCE.json) (2026-10-04).

## Sources

- [Python optional dependencies](https://docs.aws.amazon.com/powertools/python/latest/getting-started/install/#extra-dependencies)
- [Pinned TypeScript Parser manifest](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/parser/package.json)
- [Go module source organization and subdirectory tags](https://go.dev/doc/modules/managing-source)
- [Go modules, graph pruning, workspaces, and version paths](https://go.dev/ref/mod)
- [Go internal package rules](https://pkg.go.dev/cmd/go#hdr-Internal_packages)
