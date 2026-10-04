# Releasing Go modules at one version

Several PRs can merge before a version is published. Source merges do not
publish modules. All maintained public modules release together at one version,
even when only one component changes. Go module boundaries and import paths stay
independent; nested modules still require their own prefixed tags.
The repository publishes Go source modules and GitHub Releases with GoReleaser
OSS; Lambda ZIPs remain CI example artifacts. No cloud AWS account is needed.

## Contribution notes accumulate until publication

Every PR includes a `## Release notes` section. Write one English, user-facing
description per line, with the module directory and change type:

```text
## Release notes

- logger | fix | Preserve temporary attribute lifetimes in child loggers.
- logger | fix | Include overflow error details in buffered output.
- metrics | feature | Add an optional metric configuration setting.
```

Use module directories from `tools/modules.json`, including `.` for root
Commons. Use `repository` for repository-only tooling and contribution work;
these entries appear in the root version's unified summary. Types are `breaking`,
`feature`, `fix`, `documentation`, and `maintenance`. A PR may describe several
changes across several modules. For a change with no release impact, write
`None: <specific reason>`. The contribution policy requires the section and
syntax; release preparation checks actual module names. Maintainers review
whether the declared modules and summaries accurately describe the changes.

For Logger v0.1.1, the generator collects all merged PRs between
`logger/v0.1.0` and the selected source commit, then includes only Logger entries.
A Metrics component note uses its previous tag. The root `v0.1.1` Release groups
all actual changes by component and includes repository notes and a full module
version table. Modules with only dependency alignment are identified separately.
One PR fixing three Logger behaviors can
produce three notes with the same PR link. Notes group breaking changes,
features, fixes, documentation, and maintenance in that order.

History uses Git ancestry and associated merged PRs, not date windows. Squash,
merge, and rebase histories are deduplicated. Stable releases compare against
previous stable releases; prereleases may compare against previous prereleases.
Only published GitHub Releases on the source ancestry are release boundaries,
not draft Releases or unrelated module tags.

## Prepare the unified release

After the workflows are merged, open
[Actions: Prepare release](https://github.com/rambow-cloud/powertools-lambda-go/actions/workflows/prepare-release.yml)
and choose **Run workflow** on `main`:

| Input | Value |
|---|---|
| `bump` | Leave `auto` for note-based versioning, or select `patch`, `minor`, or `major` |
| `auto_publish` | Leave checked to publish after the preparation PR merges and main checks pass |

The workflow creates a Release tracking issue and a preparation PR. It computes
one shared version, includes every maintained public module, synchronizes
manifest versions and internal `go.mod` requirements, rebuilds `go.work` version
mappings, tidies dependency sums, and freezes each module's accumulated notes.
No manual version-file edits, module-list script, issue number, SHA, or plan name
are required. Every release includes all maintained public modules and Commons;
development modules and the frozen `tracer/xray` adapter are excluded.

Review the generated PR's shared version, full module table, notes,
initial compatibility statements, and automatic-publication setting. Required
checks use the native PR workflows. Preparation waits for those runs to appear
and attempts to authorize pending runs with its Actions write permission. If
GitHub requires maintainer authorization, select **Approve workflows to run**
in the PR. A manually dispatched job check does not satisfy PR rulesets, even
when it passes on the same commit; see [required-check troubleshooting](https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks).
Merge the preparation PR with squash or merge after the checks pass. The
recorded source must still be its first parent; if main advances first, rerun
preparation against current main and use the newly generated PR.

With automatic publication enabled, merging this preparation PR authorizes its
frozen release batch. After both required main checks pass, GoReleaser publishes
in dependency order and the tracking issue closes after public consumer checks.
Ordinary feature/bug PR merges do not publish modules.

The repository's **Settings → Actions → General → Workflow permissions** must
allow **GitHub Actions to create and approve pull requests** for automated PR
creation. The preparation workflow does not approve or merge PRs. Its explicit
job permissions grant the built-in token only the operations needed for issue,
branch, PR, and native workflow activation; no additional secret is required.

An organization or enterprise policy can prohibit that repository setting.
If GitHub reports that the organization does not allow Actions to create or
approve PRs, an organization owner must first permit it under **Organization
Settings → Actions → General → Workflow permissions**. Then enable the
repository setting above. See [GitHub's organization policy documentation](https://docs.github.com/en/organizations/managing-organization-settings/disabling-or-limiting-github-actions-for-your-organization).
The CLI preparation commands below can use a maintainer's existing `gh`
authentication while the organization policy stays restricted.

## Automatic versions and dependency metadata

| Situation | Default target |
|---|---|
| First public release | The configured initial version, normally `v0.1.0` |
| Fix, maintenance, documentation, or an explicit batch with no module notes | Next patch version |
| Feature | Next minor version |
| Breaking change while on v0 | Next minor version |
| Breaking change while on v1 | Stop for a separately reviewed v2 module-path migration |

For example, Logger fixes produce `v0.1.0 → v0.1.1`; a Logger feature produces
`v0.1.0 → v0.2.0` for every maintained component. The strongest accumulated change
across components and repository notes determines one automatic increment.
Reserved tags from any maintained module, including drafts or tags without
Releases, are never reused; the entire cohort advances beyond reserved versions.
An explicit `major` increment can move v0 to v1; v2+ paths are not automated.

The project version lives in `tools/modules.json` as `release_version`; all
maintained module entries must match it. The automation updates internal
dependency requirements and workspace mappings while preserving module paths
and Go language-version directives. Workspace consumer `go.mod` files are
synchronized too. Publication always covers the full maintained cohort.
Dependency changes appear in the frozen plan and component notes.
Go commands keep `CGO_ENABLED=0`; no module-file replacements are added.
Tidy uses the existing local module fixtures before published external modules.

Maintainers who prefer the CLI can start from a clean checkout of current
`origin/main`, authenticate `gh`, and run:

```sh
uv run --no-project python tools/release.py prepare --all --auto-publish
```

This command creates the issue, branch, and PR automatically. Component selection
is no longer supported. Use `--bump patch` to override the shared version
policy. CLI preparation enables automatic publication only with `--auto-publish`.
For local metadata/plan generation with no GitHub writes, pass `--local --issue
123`; `--issue` otherwise reuses an existing open tracking issue. `--plan` and
`--overrides` are optional advanced controls. All tracked metadata is restored
if preparation/tidy fails before the branch is committed.

## Historical PRs and first releases

Current PRs require structured release notes. Detailed entries are preserved
and grouped by module and category. Historical PRs without that section use
their actual titles and changed module paths; the full file-count coverage is
checked, and the plan identifies inferred summaries for review. Malformed
structured notes still fail rather than being silently replaced. Direct commits
are acknowledged using their subjects and affected modules. Initial releases
receive a conservative scope statement referring to module documentation.

The first release therefore needs no hand-authored overrides file. Maintainers
can improve generated historical summaries and initial capability statements in
the preparation PR, or supply optional JSON overrides for preparation. Each
`pull_requests` value contains the release-note section without its heading.
Repository-only entries are included in the root unified summary.

```json
{
  "pull_requests": {
    "2": "- repository | maintenance | Establish an issue-to-PR contribution workflow.",
    "6": "- logger | fix | Preserve temporary attribute lifetimes in child loggers.\n- logger | fix | Omit ordinary empty and null top-level attributes.\n- logger | fix | Correct buffer trace ownership and overflow error details."
  },
  "initial_summaries": {
    "logger": "Initial structured logging module. Describe approved capabilities and compatibility limitations here."
  },
  "untracked_commits": {
    "0123456789012345678901234567890123456789": {
      "modules": ["logger"],
      "description": "Initial source import; supported capabilities are described in the initial scope."
    }
  }
}
```

Use actual source hashes, PR numbers, and approved descriptions. Save input in
an ignored file such as `.tmp/release-overrides.json`, then pass
`--overrides .tmp/release-overrides.json`. Relevant overrides are copied into
the reviewed plan. Historical overrides do not rewrite merged PR descriptions.
First release notes include both a capability/compatibility summary and PR changes.

## Automatic publication and manual recovery

Automatic publication listens for completed **Go CI** and **Documentation**
workflows on main push commits. It resolves the merged preparation PR and its
exact plan, requires the plan's `auto_publish: true`, and rechecks the latest
required main checks. If another check is still pending or failed, it makes no
publication writes; the next completion event re-evaluates the batch. Fork and
PR-check events are excluded. A closed tracking issue prevents duplicate
automatic completion from repeating an already finished batch.

For a plan prepared with automatic publication disabled, or to resume after a
failure, open
[Actions: Publish Go modules](https://github.com/rambow-cloud/powertools-lambda-go/actions/workflows/release.yml)
and choose **Run workflow** on `main`. Enter only the merged preparation PR number.
Leave `publish` unchecked for preflight or check it to publish/resume. Issue,
SHA, and plan are resolved from the merged PR; they need no manual copying.

The equivalent CLI commands are:

```sh
uv run --no-project python tools/release.py publish --pr 456
uv run --no-project python tools/release.py publish --pr 456 --publish
```

Publication checks caller write permission, Issue/PR association, merged SHA,
clean checkout, reviewed plan, required PR/main checks, module paths/versions,
dependency metadata, previous release boundaries, dependency order, and
tag/Release conflicts. Preflight writes local artifacts only, including a
nonpublishing GoReleaser run for every selected module. Publication is serialized
and an active run is not canceled by a newer request.

For each module, publication creates a tag at the selected SHA and uses
GoReleaser to create its draft GitHub Release with the reviewed notes. After a
real public consumer passes, a second GoReleaser invocation publishes that same
draft before proceeding to the next module. The root summary stays draft until
every component consumer and Release succeeds, then is finalized last. It marks
the stable project version as GitHub's Latest Release. Root Commons uses `vX.Y.Z`; Logger uses
`logger/vX.Y.Z`. Consumers use fresh caches, `GOWORK=off`, `CGO_ENABLED=0`,
the public Go proxy and checksum database, no local proxies/replacements, and
a consumer build. This differs from synthetic local module verification.
After all selected modules pass, the workflow posts Release links and closes
the tracking issue.

The built-in GitHub token performs writes. Publication does not depend on a
tag-triggered follow-up workflow. Component Releases do not replace the root
Latest label. Prerelease versions create prerelease
Releases. v2+ module-path migrations need a separate feature and are rejected
by this initial tool.

## GoReleaser configuration and independent tags

New preparations use schema 2: the complete maintained module set, one version,
aligned internal requirements, and frozen unified summary are required before
publication writes. Immutable schema-1 plans from before this policy can still
be recovered at their original commits, whose manifests lack `release_version`.
They cannot be used to bypass the policy at a new preparation commit. Published
`v0.1.0` tags, notes, and acceptance records are not rewritten by this change.

The workflow installs GoReleaser OSS **v2.18.2** through a commit-pinned official
action. `.goreleaser.json` is the shared configuration; GoReleaser accepts JSON
through its YAML parser. The publisher derives per-module configurations under
`dist/releases/NAME/MODULE/`, changing the project name, output directory, and
explicit prerelease status from the reviewed plan.
Library releases skip binary builds, checksums, and artifact uploads.
Each invocation saves a phase configuration with the required draft state;
the shared configuration defaults to draft releases.

GoReleaser owns GitHub Release creation and finalization. The Python tooling
prepares accumulated PR notes, enforces the reviewed plan, orders dependencies,
creates exact tags, verifies public consumers, and completes the tracking issue.
GitHub's by-tag API returns published Releases. The adapter finds matching
drafts through authenticated release listing, rejects duplicate matches, and
validates existing notes/tag targets before resuming.
After a successful GoReleaser write, the publisher waits up to 30 seconds for
the requested draft or published state to become visible. It checks reviewed
content on every observed Release and stops immediately on conflicts or API
errors. This bounded read-after-write wait never repeats a publication write.
If visibility does not converge, existing objects are preserved for recovery.
It passes each frozen Markdown file using `--release-notes`; GoReleaser does not
replace it with a repository-wide commit changelog. Existing notes are kept;
conflict detection tolerates only terminal newline formatting differences.

[Native monorepo tag-prefix support](https://goreleaser.com/customization/monorepo/)
requires GoReleaser Pro. This source-library integration uses the OSS release
command with `GORELEASER_CURRENT_TAG` set to the complete reviewed tag and
`--skip=validate`. It is a compatibility adapter, rather than native OSS
monorepo support. The publisher replaces those skipped checks: the checkout
must be clean at the exact merged SHA, the module version must be valid and
agree with the manifest, and local/remote tags must identify that SHA. It also
checks required CI and the preparation PR before writes. Prerelease status is
set explicitly from the reviewed version; no artifact templates use GoReleaser's
semantic-version fields for prefixed tags.

Preflight additionally skips `publish` and `announce`, removes inherited SCM
tokens from the GoReleaser environment, and creates no public tags or Releases.
For first releases, the selected SHA fills the previous-tag environment field;
PR history boundaries still come exclusively from the reviewed plan.
Use the documented workflow or `tools/release.py publish` entry point rather
than publishing directly with GoReleaser: the entry point supplies these gates,
notes, tag context, and dependency ordering.

For local offline checks, install GoReleaser OSS v2.18.2 on `PATH`, then run:

```sh
goreleaser check --config .goreleaser.json
uv run --project website --frozen python tools/test_release.py
uv run --project website --frozen python tools/test_release_automation.py
```

The real CLI regression creates an isolated temporary Git repository and checks
root, nested-module, and prerelease tags with publication disabled. CI installs
the pinned CLI and runs this regression. A second real CLI regression uses a
local GitHub API fixture to verify draft creation, draft reuse/finalization,
notes, tag targets, stable/prerelease flags, and the latest-release setting.
These CLI regressions skip locally only when the CLI is absent.
Automation regressions use isolated Git repositories, real Go metadata/tidy
commands, and a fake GitHub API; they do not create real issues, tags, or Releases.
Updating the pinned version requires updating the workflow, tool version
gate, and CLI compatibility coverage together.

## Failure and recovery

For preparation failures, rerun **Prepare release** with the same inputs on
the same main source. An existing preparation PR is reused; successful and
pending native checks are retained, runs needing authorization are activated,
and failed native runs are rerun while preserving their PR event association.
If automatic authorization is forbidden, approve workflows in the PR and use
the existing runs. A pushed preparation branch whose PR creation
failed is reused after its source/request identity is verified. It is never
force-pushed. Local metadata is restored on generation/tidy failures. If main
advances, preparation creates a fresh plan and PR for the new source.

For publication failures, inspect workflow logs and the
`release-progress-RUN_ID` artifact, including per-module GoReleaser phase
configurations, metadata, and logs. Public-consumer dependency/build caches
stay on the runner. Failed consumer checks preserve tags/drafts and leave the
tracking issue open. A tag already makes a Go version publicly
addressable; a draft Release is not rollback. Never delete, move, or rewrite a
conflicting version.

Consumer checks make one attempt. If the proxy is not ready, resolve the
condition and rerun **Publish Go modules** with the same preparation PR and
`publish` checked. Matching tags/Releases are reused and incomplete steps resume;
notes are never overwritten. Changed source or scope needs a new preparation PR
and version. Workflow installation does not publish the first real version;
hosted acceptance requires an approved preparation PR and actual public results.

## Sources

- [Go multiple-module source and tag rules](https://go.dev/doc/modules/managing-source)
- [Go module publication](https://go.dev/doc/modules/publishing)
- [Go module/workspace editing](https://go.dev/ref/mod)
- [GoReleaser library releases](https://goreleaser.com/resources/cookbooks/release-a-library/)
- [GoReleaser custom release notes and draft reuse](https://goreleaser.com/customization/publish/scm/)
- [GitHub workflow triggers and token behavior](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)
