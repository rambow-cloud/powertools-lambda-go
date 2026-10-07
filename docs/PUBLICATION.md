# Publication readiness

Repository path: github.com/rambow-cloud/powertools-lambda-go.

This is an independent development subset of Powertools for AWS Lambda (TypeScript) v2.35.0. Public source availability does not imply complete feature parity or a released Go module version. See [the roadmap](ROADMAP.md), [module layout](MODULES.md) and [release plan](PUBLISHING_PLAN.md).

## Public source boundary

Include maintained source, module/workspace files, examples, synthetic test fixtures, reference generators, CI configuration, licenses and sanitized acceptance summaries. Test credentials, example addresses, account IDs and local cryptographic fixtures are synthetic; their purpose and limitations are documented with the tests.

Exclude personal configuration, authentication files, private backups, raw cloud evidence, dependency caches, build outputs, editor recovery files and local agent state. The repository ignore rules also apply when generating module verification archives, including to accidentally staged ignored files.

Historical summaries preserve test counts, scope, limitations and archive hashes. Historical module identities use repository-relative directories, and combined runs use anonymous checkpoint labels. They do not expose previous personal repository names, workstation paths or local execution-directory identifiers. Original audit records are kept outside the public source set.

AWS integration tools require explicit AWS_PROFILE and POWERTOOLS_TEST_ACCOUNT configuration and verify the expected account. Cloud testing must be explicitly authorized. The default integration workflow uses local Docker.

## Maintained tooling

- tools/modules.py maintains dependency metadata and verifies independently packaged modules.
- tools/licenses.py checks each module's license and notice files.
- tools/run-linux-test.py executes Linux test binaries from non-Linux hosts.
- integration/local/ runs local Lambda and streaming acceptance and summarizes saved evidence.
- integration/run.py and integration/build_collector.py support explicitly configured cloud tests and reproducible collector builds.
- tools/reference/ contains maintained TypeScript reference generators and pinned source data.
- scripts/build.ps1 builds and validates Lambda example artifacts.

One-time investigation and historical report-generation scripts are not part of the maintained public tooling.

## Verified local scope

The namespace migration checkpoint (2026-09-27) covers 31 packaged modules and 28 independent public consumers, with GOWORK=off, CGO_ENABLED=0, tests, vet, tidy consistency and dependency isolation. It combines 13 accepted modules with an 18-module continuation rather than representing one uninterrupted run. See [module acceptance](MODULE_ACCEPTANCE_MIGRATION.json).

The basic Lambda example was built for Linux amd64 and arm64 with CGO disabled. Packaging verified static ELF binaries, architecture/build metadata and executable bootstrap ZIP entries with mode 0755. This migration did not repeat the full RIE/streaming suite or make AWS calls. [Runtime acceptance](LOCAL_VALIDATION.md) retains its separate historical scope.

Publication hygiene checks inspect candidate files, existing staged blobs, ignore rules and module archive contents. They preserve the reference fixtures and dependency metadata and do not rerun the Go/Lambda suites for documentation-only changes. Pattern checks are scoped detection, not a guarantee against every secret format.

## Pre-upload privacy review (2026-09-30)

Reviewed 657 public candidate files (approximately 36.7 MiB before this review note), plus all 32 existing index blobs. No private/generated directories or ignored-but-staged files were found. Ignore checks cover documentation output, preview logs, virtual environments, tool caches, private backups, credentials, and infrastructure state.

Gitleaks v8.30.1 reported 350 matches. All were classified against the actual fixtures and their generators: 338 encoded Kafka message keys, 11 SigV4 authorization strings generated with synthetic example credentials, and one historical artifact SHA-256 hash. No unclassified scanner findings remain. Supplemental checks found no personal workstation paths or credential tokens; cloud account IDs and email domains were verified as test/example values. Logo C2PA provenance was preserved and checked for sensitive patterns.

Only the root module gained website assets since the previous module-archive review. A fresh root archive contained 122 files and excluded private/generated content. This was a packaging-boundary check, not a rerun of Go tests or cloud acceptance. Private snapshots, redacted scanner output, and classification details are retained outside the public source set.

At the privacy-review checkpoint, the Git index contained only 32 files from an earlier snapshot, with 22 outdated versions. The subsequent staging review replaced that partial index with all 657 reviewed public files, verified that staged content matches the working tree, and confirmed that no ignored files are staged. The original index is backed up privately. No commit or push was performed. Scanner results are scoped evidence, not a guarantee against every possible secret format.

Retain all 64 files under testdata/ (approximately 32.2 MiB), together with their maintained reference generators. These synthetic inputs and expected outputs support compatibility regression tests; they are source assets, not disposable runtime output. Raw cloud responses, execution logs, caches, generated sites, and private scanner reports remain excluded.

Whitespace review recognizes Windows CRLF line endings. Two trailing spaces in the vendored Unicode PropertyValueAliases.txt comments are preserved with the upstream reference file; no other whitespace findings remain.

## Before initial upload

- [x] Use the rambow-cloud module namespace consistently in source, workspace mappings and documentation.
- [x] Include project MIT licensing and preserved third-party notices in all 31 modules.
- [x] Remove one-time tooling and keep maintained verification/reference tools.
- [x] Sanitize personal identifiers and local execution metadata while preserving verification scope.
- [x] Verify packaged modules, independent consumers and both Lambda example architectures with CGO disabled.
- [x] Prepare and review the complete staged source snapshot, including fixture retention and ignored-file boundaries.
- [x] Create the public rambow-cloud/powertools-lambda-go repository.
- [x] Push reviewed source and verify hosted CI/documentation publication; current checks and deployment milestones are recorded in [CHECKLIST.md](CHECKLIST.md).

## Published versions and remaining acceptance

The 27 maintained public modules were initially released at v0.1.0 with fresh
public consumers recorded in [RELEASE_ACCEPTANCE.json](RELEASE_ACCEPTANCE.json).
The latest published cohort is [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v0.2.0).
The frozen X-Ray adapter remains excluded. Local proxy fixtures remain distinct
from public-version verification. Existing module license/notice checks run in CI;
binary distributions require review of their resolved transitive notices.

The full stable API audit, candidate acceptance and final publication are tracked
in [V1_READINESS.md](V1_READINESS.md). Keep historical reports at their original
scope; source merges do not publish new versions.
