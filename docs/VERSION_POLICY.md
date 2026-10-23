# Documentation and verification versions

Documentation on `main` and the site describes the current source and can include
unreleased APIs. The latest stable cohort is [v1.3.0](RELEASE_NOTES.md#v130).
It requires Go 1.27.3 and uses JSON v2 directly. The earlier v1.0.0-rc.1 candidate
and v1.2.0/v1.1.0/v1.0.0/v0.2.0 stable cohorts remain available as historical versions. Select
`@v1.3.0` explicitly to reproduce the latest stable release.

## Examples and installation

Current `main` source requires Go 1.27.3 or newer, including the standard-library
security fixes released in October 2026. Published v1.2.0 keeps its original
Go 1.27 directive; rebuild applications with a patched toolchain rather than
assuming a dependency update patches an existing Lambda binary.

- Installation commands identify the applicable published version. Avoid
  `@latest` when reproducing a specific behavior.
- Runnable examples record dependencies in `go.mod`/`go.sum`. Development modules
  such as `examples` are not independently published.
- Checkout snippets describe their source revision. Identify a new API as
  unreleased until its module version is published.
- The README example targets v1.3.0. Verify it in an isolated application using
  its pinned installation command rather than the repository workspace.
- Import paths contain no `@version`; selection belongs to `go get` and `go.mod`.
- For release-specific documentation, use the matching Git tag's docs/examples.
  Nested modules have prefixed tags. The site is not a versioned archive.

## Test and acceptance identity

### Logger stacks Preview by source version

> This Preview is included in v1.3.0 and main. It is not included in v1.2.0.

| Source | Error stacks | Automatic handler panic record |
| --- | --- | --- |
| Logger `@v1.2.0` | No automatic stack collection; no `WithStack` or `WithStackTrace` API | Previous cleanup only; no `WithPanicLogging` API |
| Logger `@v1.3.0` / `@main` | `WithStack` plus default logging-site fallback; configurable with `WithStackTrace` | Enabled by default; configurable with `WithPanicLogging` |

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

An omitted version or `@latest` does not request `main`. Branch references resolve
to pseudo-versions; use a commit SHA for repeatability. See
[Go version queries](https://go.dev/ref/mod#version-queries).
The [Preview example](https://github.com/rambow-cloud/powertools-lambda-go/tree/main/examples/logger-stack-preview)
requires a revision containing these APIs; the historical v1.2.0 installation
commands do not provide them. The v1.3.0 unified stable cohort includes these
APIs with their Preview status. Availability follows the selected source revision,
without runtime branch detection or build tags. Published v1.2.0 remains unchanged.

### Evidence boundaries

| Evidence | Identity and meaning |
| --- | --- |
| PR/main tests | Exact source SHA and workflow run; verifies changed source |
| Packaged local checks | Source SHA, fixture versions and archive hashes; not public retrieval |
| Runtime checks | Artifact/source identity, executed architecture and runtime harness |
| Publication consumers | Exact public tag, fresh caches, GOWORK off, public proxy/checksum database |
| Reference comparisons | TypeScript v2.35.0 source, dependency lock and fixture identity |
| Performance | Source SHA, Go version, OS/architecture, payload, flags and scoped results |
| Cloud examples | Original artifact/service scope and sanitized outcomes; not later-source acceptance |

PR/main CI tests the triggered source instead of downloading the previous release.
Publication checks independently verify frozen public versions. A test suite needs
no separate project-style semantic version. Pin tools, dependencies and reference
data separately. Every Go subprocess keeps `CGO_ENABLED=0`; concurrent functional
tests are supported and the CGO-dependent race detector is excluded.

## Record limits accurately

Keep historical reports at their original scope/date. Do not relabel an old result
with a new version. Reuse passing unchanged scope; document skipped phases,
cross-compilation versus execution and local fixtures versus public artifacts.
See the [v1 gates](V1_READINESS.md) for remaining acceptance.
