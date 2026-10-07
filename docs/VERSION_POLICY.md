# Documentation and verification versions

Documentation on `main` and the site describes the current source and can include
unreleased APIs. The latest stable cohort is [v1.0.0](RELEASE_NOTES.md#v100).
It requires Go 1.27 and uses JSON v2 directly. The earlier v1.0.0-rc.1 candidate
and v0.2.0 stable cohort remain available as historical versions. Select
`@v1.0.0` explicitly to reproduce the stable release.

## Examples and installation

- Installation commands identify the applicable published version. Avoid
  `@latest` when reproducing a specific behavior.
- Runnable examples record dependencies in `go.mod`/`go.sum`. Development modules
  such as `examples` are not independently published.
- Checkout snippets describe their source revision. Identify a new API as
  unreleased until its module version is published.
- The README example targets v1.0.0. Verify it in an isolated application using
  its pinned installation command rather than the repository workspace.
- Import paths contain no `@version`; selection belongs to `go get` and `go.mod`.
- For release-specific documentation, use the matching Git tag's docs/examples.
  Nested modules have prefixed tags. The site is not a versioned archive.

## Test and acceptance identity

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
