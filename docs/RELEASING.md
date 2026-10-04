# Releasing independent Go modules

Several PRs can merge before a version is published. Source merges do not
publish modules. Each module has its own version and release-note boundary.
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
these entries do not appear in individual module notes. Types are `breaking`,
`feature`, `fix`, `documentation`, and `maintenance`. A PR may describe several
changes across several modules. For a change with no release impact, write
`None: <specific reason>`. The contribution policy requires the section and
syntax; release preparation checks actual module names. Maintainers review
whether the declared modules and summaries accurately describe the changes.

For Logger v0.1.1, the generator collects all merged PRs between
`logger/v0.1.0` and the selected source commit, then includes only Logger entries.
A Metrics release uses its own previous tag; it does not lose changes just
because Logger was released first. One PR fixing three Logger behaviors can
produce three notes with the same PR link. Notes group breaking changes,
features, fixes, documentation, and maintenance in that order.

History uses Git ancestry and associated merged PRs, not date windows. Squash,
merge, and rebase histories are deduplicated. Stable releases compare against
previous stable releases; prereleases may compare against previous prereleases.
Only published GitHub Releases on the source ancestry are release boundaries,
not draft Releases or unrelated module tags.

## Track and prepare a release

1. Open a **Release tracking** issue from the
   [issue chooser](https://github.com/rambow-cloud/powertools-lambda-go/issues/new/choose).
   Agree on selected modules, target versions, compatibility, dependency order,
   and publication acceptance. Initial releases must describe the supported
   capabilities and remaining compatibility boundaries.
2. Fetch `origin` and its tags. Start a preparation branch from current
   `origin/main`. Update selected versions in `tools/modules.json`, relevant
   internal requirements, and workspace mappings. Use
   `uv run --no-project python tools/modules.py tidy` when dependencies change.
   All Go commands use `CGO_ENABLED=0`. Development modules (`examples`,
   `integration`, `tools`) and the frozen `tracer/xray` adapter are excluded.
3. Generate a plan. Select unpublished internal dependencies explicitly; the
   publisher verifies requirements and orders dependencies before consumers.
   It does not silently expand release scope.
4. Review `releases/NAME.json` in a preparation PR using `Refs #ISSUE`, keeping
   the tracking issue open. The plan freezes versions, previous tags, reviewed
   and excluded PRs, historical acknowledgements, and rendered notes. Later
   edits to merged PR descriptions do not alter that snapshot. Update structured
   entries and their rendered notes together if review changes the wording.
5. Run offline release and documentation checks. Wait for the three existing
   required PR checks. The preparation PR may change only its plan and
   module/dependency/license metadata; merge code and tooling changes earlier.
6. Refresh the plan if `main` advances. Squash or merge the preparation PR so
   the release commit's first parent equals the plan's source SHA. Do not
   rebase-merge a preparation PR with multiple commits. Wait for main's module
   and documentation checks on the merged SHA; the publisher reuses those runs.

After updating the selected module metadata:

```sh
export CGO_ENABLED=0
git fetch origin --tags
uv run --no-project python tools/release.py prepare \
  --issue 123 --plan logger-v0-1-1 --module logger
```

Repeat `--module` to select more modules. Include `--module .` for initial Commons
publication when required. In PowerShell, use `$env:CGO_ENABLED='0'` and put the
command on one line instead of shell continuation backslashes. Preparation
creates no public tags, Releases, comments, or issue state changes.

## Historical PRs and first releases

Missing historical notes fail preparation. Supply reviewed JSON overrides;
there is no silent PR-title fallback. Each `pull_requests` value contains the
release-note section's content without its heading. Acknowledge commits with
no merged PR explicitly and declare their affected modules; matching commits
appear in their own section of the notes. Repository-only acknowledgements stay
in the reviewed plan and are excluded from module notes.

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

## Preflight and publish

Open [Actions: Publish Go modules](https://github.com/rambow-cloud/powertools-lambda-go/actions/workflows/release.yml),
choose **Run workflow** on `main`, and enter the tracking issue, merged preparation
PR, complete 40-character merge SHA, and plan name without directory or `.json`.
Leave **publish** unchecked for preflight. Review its notes/progress artifact,
then run the same inputs with **publish** checked to authorize publication.
Maintainers can also dispatch using `gh workflow run release.yml`.

Preflight checks caller write permission, Issue/PR association, merged SHA,
reviewed plan, required Actions checks, module paths/versions, previous release
boundaries, dependency order, and tag/Release conflicts. It creates local output
artifacts only, including a nonpublishing GoReleaser run for each selected module.
Publication runs are serialized and never canceled by a newer run.

For each module, publication creates a tag at the selected SHA and uses
GoReleaser to create its draft GitHub Release with the reviewed notes. After a
real public consumer passes, a second GoReleaser invocation publishes that same
draft before proceeding to the next module. Root Commons uses `vX.Y.Z`; Logger uses
`logger/vX.Y.Z`. Consumers use fresh caches, `GOWORK=off`, `CGO_ENABLED=0`,
the public Go proxy and checksum database, no local proxies/replacements, and
a consumer build. This differs from synthetic local module verification.
After all selected modules pass, the workflow posts Release links and closes
the tracking issue.

The built-in GitHub token performs writes. Publication does not depend on a
tag-triggered follow-up workflow. Independently versioned modules do not set
a repository-wide latest Release label. Prerelease versions create prerelease
Releases. v2+ module-path migrations need a separate feature and are rejected
by this initial tool.

## GoReleaser configuration and independent tags

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
```

The real CLI regression creates an isolated temporary Git repository and checks
root, nested-module, and prerelease tags with publication disabled. CI installs
the pinned CLI and runs this regression. A second real CLI regression uses a
local GitHub API fixture to verify draft creation, draft reuse/finalization,
notes, tag targets, stable/prerelease flags, and the latest-release setting.
These CLI regressions skip locally only when the CLI is absent.
Updating the pinned version requires updating the workflow, tool version
gate, and CLI compatibility coverage together.

## Failure and recovery

Inspect workflow logs and the `release-progress-RUN_ID` artifact, including
per-module GoReleaser configuration, metadata, and preflight/draft/publish logs.
Failed consumer
checks preserve tags/draft Releases and leave the tracking issue open. A tag
already makes a Go version publicly addressable; a draft Release is not rollback.
Never delete, move, or rewrite a conflicting published version.

Consumer checks make one attempt. If the proxy is not ready, stop and rerun the same
reviewed plan after resolving the condition. Matching tags/Releases are reused;
public consumers are checked in a fresh cache and missing steps are completed.
Existing notes are not overwritten. Changed source or scope needs a new version
and preparation PR. Installing the workflow does not claim hosted publication;
the first real release needs separate authorization and actual public results.

## Sources

- [Go multiple-module source and tag rules](https://go.dev/doc/modules/managing-source)
- [Go module publication](https://go.dev/doc/modules/publishing)
- [GoReleaser library releases](https://goreleaser.com/resources/cookbooks/release-a-library/)
- [GoReleaser custom release notes and draft reuse](https://goreleaser.com/customization/publish/scm/)
- [GitHub generated release notes](https://docs.github.com/en/repositories/releasing-projects-on-github/automatically-generated-release-notes)
- [GitHub workflow triggers](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)
