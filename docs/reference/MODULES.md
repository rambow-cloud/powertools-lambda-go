# Module development and package documentation

Use [module installation](../../MODULES.md) for application dependencies.

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

## Package documentation on pkg.go.dev

Each maintained public package keeps its overview in `doc.go` and public usage
examples in an external-package `example_test.go`. Use Go doc comments for API
links and headings so both `go doc` and pkg.go.dev can display them. Examples with
`Output` comments run during module tests; examples requiring AWS, an Agent or
shared persistence compile without running when they omit `Output`. Keep those
requirements explicit and use synthetic inputs. The frozen `tracer/xray` adapter
retains its deprecation notice and existing regression tests.

pkg.go.dev reads published module source, including package comments, examples
and license files. Merging documentation changes does not update an existing
version's immutable documentation. Publish through the normal
[release process](../RELEASING.md), then verify the version selected in the browser.
Nested modules are indexed separately; the root module's importable public
package is `commons`. See the official [Go doc comment guide](https://go.dev/doc/comment)
and [module publication guide](https://go.dev/doc/modules/developing).

For a published version containing the changes, open these pages and select that
version from the version selector:

- [Commons](https://pkg.go.dev/github.com/rambow-cloud/powertools-lambda-go/commons):
  check the Overview and the `NewLRUCache` example.
- [Logger](https://pkg.go.dev/github.com/rambow-cloud/powertools-lambda-go/logger):
  check invocation lifecycle guidance and the `New` example.
- [HTTP Metrics](https://pkg.go.dev/github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics):
  check request-scope publication guidance and the `New` middleware example.
- [KMS provider](https://pkg.go.dev/github.com/rambow-cloud/powertools-lambda-go/datamasking/kms):
  check the Encryption SDK limitations and compile-only `New` example.

If a published module has not yet been indexed, use pkg.go.dev's request action
for that module/version. Confirm its license, Overview, API links and Examples
after processing. Do not expect unpublished branch changes on the public site.

## Scoped verification

After a relevant source change, `$env:CGO_ENABLED='0'; uv run python tools/modules.py check --only eventhandler/appsyncevents` in PowerShell rebuilds proxy artifacts and runs the same archive tests, vet, tidy and independent-consumer checks for the selected module. Repeat --only for additional affected modules. Dependencies are packaged but are not claimed as tested by that scoped run. The result is saved separately as MODULE_ACCEPTANCE_SCOPED.json and never replaces MODULE_ACCEPTANCE.json. Full-workspace checks remain the default, including workspace/manifest and dependency-isolation validation.

The datamasking/kms module declares test_os=linux because its official SDK dependency cannot compile natively on Windows. The module tool propagates this requirement to dependents, cross-compiles with the local Go toolchain, runs tests through tools/run-linux-test.py in the pinned Lambda Docker image, and builds Linux standalone consumers. Linux hosts execute the tests natively. No module-cache patches or SDK forks are used. See DATAMASKING_KMS.md.
