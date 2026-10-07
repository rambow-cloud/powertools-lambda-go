# Release notes

Published versions of Powertools for AWS Lambda (Go) are listed below, newest
first. Maintained public modules share one project version; they keep independent
Go import paths and component tags. Module headings, bump-only entries and the
version table use project-qualified display names.

[Browse all GitHub Releases](https://github.com/rambow-cloud/powertools-lambda-go/releases).
GitHub Releases are the canonical publication record. This page records published
changes; later changes on `main` belong to a future release. See
[compatibility boundaries](COMPATIBILITY.md) before upgrading and
[the release workflow](RELEASING.md) for maintainer instructions.

## v1.0.0-rc.1

Published **2026-10-07** as a prerelease. [GitHub Release and consolidated notes](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.0.0-rc.1).

All 27 maintained modules use one version and independent tags at the same
commit, [`7eefb62fbde9`](https://github.com/rambow-cloud/powertools-lambda-go/tree/7eefb62fbde95a4d85eec1aee484a47e590a7d00).
The publisher creates one project Release. This candidate requires Go 1.27 and
uses JSON v2 directly; stable v0.2.0 remains available.

Both candidate and merged-main Full regression passed. All 27 fresh public
consumers built with CGO disabled, GOWORK off, the public Go proxy and checksum
database, without local replacements. Exact identities and checksums are in
[candidate publication acceptance](RELEASE_ACCEPTANCE_V1_RC1.json).
Browser/pkg.go.dev review and stable v1.0.0 approval remain separate gates.

## v0.2.0

Published **2026-10-05**. [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v0.2.0).

All maintained public modules move to this version. Modules without file changes receive a version bump only.

**Scope:** 4 modules with changes; 23 version bumps only.

**Source:** [74cbc6d74482](https://github.com/rambow-cloud/powertools-lambda-go/tree/74cbc6d74482c5cf48b6fa70a49aeee305bfabe1)

**Publication status:** [Release tracking issue #43](https://github.com/rambow-cloud/powertools-lambda-go/issues/43)

### Updated modules

#### powertools-lambda-go/parameters

##### Fixes

- Preserve reference UTF-8 replacement and remove leading BOMs in byte and binary parameter transforms. ([#30](https://github.com/rambow-cloud/powertools-lambda-go/pull/30))

<details markdown="1">
<summary>Changed files (3)</summary>

- `parameters/testdata/utf8-v2.35.0.json`
- `parameters/transform.go`
- `parameters/utf8_test.go`

</details>

#### powertools-lambda-go/signer

##### Fixes

- Preserve request bodies until the base transport finishes asynchronous uploads when custom signers share the input body. ([#29](https://github.com/rambow-cloud/powertools-lambda-go/pull/29))

<details markdown="1">
<summary>Changed files (2)</summary>

- `signer/transport.go`
- `signer/transport_body_test.go`

</details>

#### powertools-lambda-go/jmespath

##### Fixes

- Decode malformed UTF-8 subparts consistently in Powertools Base64 and gzip functions. ([#30](https://github.com/rambow-cloud/powertools-lambda-go/pull/30))

<details markdown="1">
<summary>Changed files (3)</summary>

- `jmespath/envelopes.go`
- `jmespath/testdata/utf8-v2.35.0.json`
- `jmespath/utf8_test.go`

</details>

#### powertools-lambda-go/parser

##### Fixes

- Match reference UTF-8 replacement and BOM handling when decoding Base64 text and gzip JSON. ([#30](https://github.com/rambow-cloud/powertools-lambda-go/pull/30))

<details markdown="1">
<summary>Changed files (3)</summary>

- `parser/helpers.go`
- `parser/testdata/utf8-v2.35.0.json`
- `parser/utf8_test.go`

</details>

### Version bumps only

No module file changes; internal requirements will follow the unified version.

- `powertools-lambda-go`: bumped to `v0.2.0`.
- `powertools-lambda-go/commons/awssdk`: bumped to `v0.2.0`.
- `powertools-lambda-go/commons/dynamodb`: bumped to `v0.2.0`.
- `powertools-lambda-go/commons/metadata`: bumped to `v0.2.0`.
- `powertools-lambda-go/commons/regex`: bumped to `v0.2.0`.
- `powertools-lambda-go/logger`: bumped to `v0.2.0`.
- `powertools-lambda-go/metrics`: bumped to `v0.2.0`.
- `powertools-lambda-go/datamasking/kms`: bumped to `v0.2.0`.
- `powertools-lambda-go/datamasking`: bumped to `v0.2.0`.
- `powertools-lambda-go/kafka/protobuf`: bumped to `v0.2.0`.
- `powertools-lambda-go/kafka/avro`: bumped to `v0.2.0`.
- `powertools-lambda-go/kafka`: bumped to `v0.2.0`.
- `powertools-lambda-go/batch`: bumped to `v0.2.0`.
- `powertools-lambda-go/idempotency`: bumped to `v0.2.0`.
- `powertools-lambda-go/idempotency/cache`: bumped to `v0.2.0`.
- `powertools-lambda-go/validation`: bumped to `v0.2.0`.
- `powertools-lambda-go/eventhandler/http`: bumped to `v0.2.0`.
- `powertools-lambda-go/eventhandler/appsyncevents`: bumped to `v0.2.0`.
- `powertools-lambda-go/eventhandler/appsyncgraphql`: bumped to `v0.2.0`.
- `powertools-lambda-go/eventhandler/bedrock`: bumped to `v0.2.0`.
- `powertools-lambda-go/eventhandler/http/metrics`: bumped to `v0.2.0`.
- `powertools-lambda-go/eventhandler/http/tracer`: bumped to `v0.2.0`.
- `powertools-lambda-go/tracer`: bumped to `v0.2.0`.

### Repository

#### Features

- Release all maintained components at one shared version with a grouped project changelog and automatic dependency alignment. ([#24](https://github.com/rambow-cloud/powertools-lambda-go/pull/24))

#### Fixes

- Activate native PR checks for prepared releases so successful checks satisfy protected-branch merge rules without duplicate manual dispatches. ([#18](https://github.com/rambow-cloud/powertools-lambda-go/pull/18))
- Discover and reuse matching GitHub Release drafts during GoReleaser publication recovery instead of treating them as missing published releases. ([#18](https://github.com/rambow-cloud/powertools-lambda-go/pull/18))
- Wait briefly for newly written Releases to become visible before continuing publication. ([#21](https://github.com/rambow-cloud/powertools-lambda-go/pull/21))

#### Documentation

- Record the first verified public v0.1.0 release and available installation commands. ([#22](https://github.com/rambow-cloud/powertools-lambda-go/pull/22))
- Use consistent project-qualified module names in unified release notes. ([#42](https://github.com/rambow-cloud/powertools-lambda-go/pull/42))

#### Maintenance

- Require synthetic Lambda runtime and cross-language data simulation checks before merging or releasing. ([#32](https://github.com/rambow-cloud/powertools-lambda-go/pull/32))
- Automatically classify bug reports and feature proposals with standard category labels. ([#34](https://github.com/rambow-cloud/powertools-lambda-go/pull/34))
- Route CI by changed files so documentation and metadata changes avoid unrelated Go builds and runtime simulations. ([#37](https://github.com/rambow-cloud/powertools-lambda-go/pull/37))
- Add documented issue labels and dedicated documentation, CI/CD, maintenance and question forms with automatic category labels. ([#38](https://github.com/rambow-cloud/powertools-lambda-go/pull/38))
- Show actual module updates separately from unified version bumps and generate source-bound release-notes previews. ([#40](https://github.com/rambow-cloud/powertools-lambda-go/pull/40))

### Module versions

<details markdown="1">
<summary>All maintained modules (27)</summary>

| Module | Version | Changes |
|---|---|---|
| `powertools-lambda-go` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/commons/awssdk` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/commons/awssdk/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/commons/dynamodb` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/commons/dynamodb/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/commons/metadata` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/commons/metadata/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/commons/regex` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/commons/regex/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/logger` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/logger/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/metrics` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/metrics/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/parameters` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/parameters/v0.2.0) | Updated |
| `powertools-lambda-go/signer` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/signer/v0.2.0) | Updated |
| `powertools-lambda-go/jmespath` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/jmespath/v0.2.0) | Updated |
| `powertools-lambda-go/datamasking/kms` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/datamasking/kms/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/datamasking` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/datamasking/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/kafka/protobuf` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/kafka/protobuf/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/kafka/avro` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/kafka/avro/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/kafka` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/kafka/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/batch` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/batch/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/idempotency` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/idempotency/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/idempotency/cache` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/idempotency/cache/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/parser` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/parser/v0.2.0) | Updated |
| `powertools-lambda-go/validation` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/validation/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/eventhandler/http` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/eventhandler/http/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/eventhandler/appsyncevents` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/eventhandler/appsyncevents/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/eventhandler/appsyncgraphql` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/eventhandler/appsyncgraphql/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/eventhandler/bedrock` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/eventhandler/bedrock/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/eventhandler/http/metrics` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/eventhandler/http/metrics/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/eventhandler/http/tracer` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/eventhandler/http/tracer/v0.2.0) | Bumped to v0.2.0 only |
| `powertools-lambda-go/tracer` | [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/tracer/v0.2.0) | Bumped to v0.2.0 only |

</details>

**Full changelog:** [v0.1.0...v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/compare/v0.1.0...v0.2.0)

## v0.1.0

Published **2026-10-04**. [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v0.1.0).

Initial public source release of the 27 maintained modules. Each module's
initial scope and compatibility statement is recorded in its GitHub Release;
full TypeScript parity is not implied. The deprecated, frozen `tracer/xray`
adapter was excluded. As the first published version, this release has no
previous project version to compare against.

All published module downloads, consumer builds and checksum checks completed
successfully. The historical publication evidence is preserved in
[RELEASE_ACCEPTANCE.json](RELEASE_ACCEPTANCE.json).
