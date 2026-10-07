# Contributing

Everyone, including the repository owner, follows the same path:
**issue → agreed scope → branch or fork → pull request → checks and review → merge**.
This is an independent community implementation, not an official AWS distribution.
Read the [compatibility boundaries](COMPATIBILITY.md) before proposing parity work.

## 1. Open an issue

Search [existing issues](https://github.com/rambow-cloud/powertools-lambda-go/issues)
and the [roadmap](ROADMAP.md) first, then choose a form in
[New issue](https://github.com/rambow-cloud/powertools-lambda-go/issues/new/choose):

- **Bug report:** affected module/version or commit, Go/runtime environment,
  a minimal reproduction, expected behavior, and actual behavior
- **Feature proposal:** the problem, proposed behavior, alternatives,
  compatibility impact, and observable acceptance criteria
- **Documentation request:** the affected guide, example or site page and the requested correction
- **CI/CD request:** the affected workflow, observed behavior and completion criteria
- **Maintenance request:** the affected tool or repository area and a scoped change
- **Usage question:** the module, what you tried and what answer would resolve it
- **Release tracking:** the unified version, accumulated changes and publication acceptance

Each form assigns its matching label. The maintained definitions are in
the [label catalog](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/.github/labels.json):

| Label | Color | Purpose |
|---|---|---|
| `bug` | Red `#d73a4a` | Incorrect behavior |
| `enhancement` | Light blue `#a2eeef` | New capability or behavior |
| `documentation` | Blue `#0075ca` | Guides, examples and the documentation site |
| `cicd` | Purple `#5319e7` | GitHub Actions, CI checks and delivery workflows |
| `maintenance` | Gray `#ededed` | Repository upkeep, tooling and refactoring |
| `release` | Green `#0e8a16` | Version preparation and publication tracking |
| `question` | Pink `#d876e3` | Usage and compatibility questions |

Combine type and area labels when useful: a workflow bug can have `bug` and
`cicd`; a documentation feature can have `enhancement` and `documentation`.
Labels organize work; they do not select CI suites or authorize publication.

Module labels use `module:<directory>`, for example `module:logger` and
`module:eventhandler/http/metrics`. The root uses `module:powertools-lambda-go`.
The catalog covers every manifest module, including development modules and the
frozen X-Ray adapter; a label does not change a module's publication status.
Select one or more **Affected modules** in the issue form, or **Repository only**
for repository work. The **Issue labels** workflow applies module and category
labels on creation, editing, or reopening. Editing this selector replaces its
previous module labels and preserves unrelated labels. Older forms are classified
additively from their explicit affected-area field; narrative text is not used.

CLI/API-created module issues can include the same field, for example:

```markdown
### Affected modules

logger, metrics
```

Run **Issue labels** from the Actions tab on `main` to synchronize the label
catalog and backfill open and closed issues. Pull requests are excluded, and
unrelated repository labels are retained. Unified release issues use `release`
without attaching every module label. Release preparation sets this label
directly for new and reused tracking issues, including built-in-token creation
that does not trigger another Actions workflow.

When creating an issue with `gh issue create`, pass labels explicitly, for example
`--label bug --label cicd`: CLI/API creation with a body does not apply web form
defaults. Maintainers keep the catalog and repository labels in sync; unrelated
standard labels such as `help wanted` are retained. To create or update a catalog
label, use its name, color and description with `gh label create --force`, for example:

```sh
gh label create cicd --color 5319e7 --description "GitHub Actions, CI checks, and delivery workflows" --force
```

Track one independently verifiable outcome per issue, not one issue per commit.
The owner can self-triage and assign their own work. Several commits or partial
PRs may share a tracking issue; questions and ideas do not have to become
implementation tasks.

Use small synthetic examples. Remove credentials, account identifiers, personal
information, and production payloads from logs and attachments. Do not report a
suspected vulnerability in a public issue or PR. Use private reporting on the
[Security page](https://github.com/rambow-cloud/powertools-lambda-go/security)
if available; otherwise ask a maintainer for a private reporting route without
disclosing the vulnerability publicly.

A maintainer checks for duplicates, confirms the scope and acceptance criteria,
and comments that the work is ready. Labels such as `bug`, `enhancement`,
`documentation`, `help wanted`, or `good first issue` are optional triage aids,
not prerequisites. Contributors do not need permission to label or assign issues.
Comment before starting to avoid duplicate work. Large API, dependency, or
compatibility changes should wait for agreement; a small fix may be proposed as
a draft while its issue is being triaged. The owner records the same scope and
acceptance decision on their own issue. No response-time guarantee is implied.

## 2. Work on a branch

External contributors fork the repository; maintainers can branch in the main
repository. Do not commit directly to `main`. For a fork, replace `YOUR-USERNAME`
and `123` below with your account and real issue number:

```sh
git clone https://github.com/YOUR-USERNAME/powertools-lambda-go.git
cd powertools-lambda-go
git remote add upstream https://github.com/rambow-cloud/powertools-lambda-go.git
git fetch upstream
git switch -c fix/123-short-description upstream/main
```

For an existing maintainer checkout:

```sh
git fetch origin
git switch -c fix/123-short-description origin/main
```

Use a focused branch such as `fix/123-description`, `feat/123-description`, or
`docs/123-description`. Keep unrelated cleanup in separate issues/PRs. Write
documentation and code comments in English; keep feedback respectful and specific.
Original contributions use the repository's MIT license; preserve upstream
licenses and attribution for third-party material.

## 3. Implement and verify

Read [AGENTS.md](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/AGENTS.md)
and [module development](MODULES.md). Use Go 1.27 or newer, uv, and Python 3.14
for documentation. Always set `CGO_ENABLED=0`; do not run the race detector.
Root `go test ./...` does **not** cover nested modules.

For code changes, add regression tests and run the packaged module and license
checks from the repository root:

```sh
export CGO_ENABLED=0
uv run --no-project python tools/licenses.py --check
uv run --no-project python tools/modules.py check
```

In PowerShell, use `$env:CGO_ENABLED = '0'` instead of `export`. Use
`tools/modules.py tidy` when dependency metadata must change; keep filesystem
`replace` directives out of `go.mod`. Focused `check --only MODULE` runs help
iteration, but are not full-workspace acceptance. Ordinary CI uses scoped
packaged checks; release preparation and manual full runs check every module
and both Linux Lambda architectures.

For documentation changes (no Go toolchain is needed):

```sh
uv lock --project website --check
uv run --project website --frozen python website/check_navigation.py
uv run --project website --frozen python website/check_guides.py
uv run --project website --frozen zensical build --clean --strict --config-file mkdocs.yml
```

For issue forms or repository automation changes, run `actionlint` and:

```sh
uv run --project website --frozen python tools/test_contribution_workflow.py
uv run --project website --frozen python tools/test_ci_changes.py
```

Release tooling changes additionally run `tools/test_release.py` and
`tools/test_release_automation.py` with the same Python command, Go installed,
CGO disabled, and the pinned GoReleaser version from [Releasing modules](RELEASING.md).

### How CI selects checks

The **CI** workflow compares the complete PR diff (merge base to head) or main
push diff (before to after), including deleted and renamed paths. It calls
separate documentation and repository automation workflows as needed:

| Changed inputs | Checks |
|---|---|
| Documentation, Markdown, site assets or site configuration | Documentation navigation, guides, strict build and links |
| Issue forms, PR template or label configuration | Workflow lint, contribution/metadata tests and CI routing tests |
| Documentation workflow | Documentation and automation checks |
| Release tools or release workflows | Automation checks plus offline GoReleaser/release tests |
| Go production source or a module's `go.mod`/`go.sum` | Changed modules and transitive consumers of their current local versions; Go source also checks documentation |
| Go tests or module `testdata` | Owning modules; production consumers do not import these test files |
| Parameters, Idempotency or shared DynamoDB changes | Affected module checks plus the dedicated DynamoDB Local suite |
| DynamoDB Local tests or runner | Integration module checks and DynamoDB Local; no unrelated runtime simulation |
| Runtime scripts, fixtures or TypeScript reference | Integration module checks and runtime simulation |
| Release plans, module manifest, shared workspace/build tools, CI routing, shared actions or unknown paths | Full regression: all modules, both architectures, runtime, DynamoDB Local, documentation and automation/release checks |

Mixed changes select the union. Shared documentation/Python dependency changes
also run automation/release tests. **Run workflow** on CI runs every suite.
No workflow-level path filter can leave a required check pending.

Module ownership uses the longest manifest-directory match, so
`eventhandler/http/metrics` is distinct from `eventhandler/http`. Dependency
expansion follows current manifest versions, matching the packaged checker;
older pinned releases use their published contents. The selection job lists
the chosen modules and suites in its summary. Deleted and renamed paths take
part in the same selection, including both sides of a rename.

The module job invokes `tools/modules.py check --only DIRECTORY` for each selected
module and verifies tests, vet, tidy metadata and independent public consumers
with `GOWORK=off`. License checks remain repository-wide. Full runs omit
`--only` and additionally produce both Lambda architecture artifacts. Runtime
simulation and DynamoDB Local use separate selection flags. A DynamoDB-related
production change includes consumers but does not run the unrelated complete
Lambda simulation; that simulation runs for runtime inputs and full regression.

The main ruleset requires **PR contribution policy** and **CI gate**. The gate
always runs and fails if classification fails, outputs are missing, or any
selected suite fails, is cancelled, or is unexpectedly skipped. Only suites
explicitly excluded by classification may be skipped. Release publication
additionally requires **Full regression** and actual module, runtime, DynamoDB
Local, documentation and release-tooling success on the preparation PR and
exact main commit. The full marker runs only for complete regression and depends
on the successful gate; a scoped run or skipped check cannot authorize publication.

For behavior involving the Lambda runtime, use the maintained
[local Docker integration runner](LOCAL_INTEGRATION.md). It includes the module
checks and both architecture builds, so do not repeat them unnecessarily. Cloud
AWS testing requires explicit authorization and explicit account/profile
configuration; it is not a contribution prerequisite. Keep generated caches,
binaries, private evidence, and credentials out of Git. Update [project progress](CHECKLIST.md)
only for verified acceptance milestones, not routine wording fixes.

## 4. Open a pull request

Commit your focused changes and push the feature branch to your fork or the main
repository. Open a **draft PR targeting `main`**. Use the PR template to explain
what changed, why, the actual tests/results, and remaining risks. Prefer titles
such as `fix(logger): preserve invocation fields` or `docs: clarify local setup`.

In the PR description's **Issue** section, put one reference per line:

```text
Closes #123
```

Use `Refs #123` for a partial step that should leave its tracking issue open.
`Fixes` and `Resolves` are also accepted. A same-repository full issue URL or
`rambow-cloud/powertools-lambda-go#123` is accepted after the keyword. The number
must identify a real issue in this repository, not another PR. References inside
HTML comments or code blocks do not count. GitHub closes issues from closing
keywords when the PR is merged into the default branch; `Refs` does not auto-close.

The **PR contribution policy** check verifies the issue reference and nonempty
**Summary** and **Testing** sections, plus structured **Release notes**. Write
one `- MODULE | TYPE | Description.` entry per user-visible change, or
`None: <specific reason>` when there is no release impact. Use module directories
from `tools/modules.json`; `repository` denotes repository-only work. See
[Releasing modules](RELEASING.md) for types and examples. Notes accumulate until
the relevant module is released. State the commands and results, or explain
why a check is not applicable or blocked; do not claim unrun tests passed. This
metadata check cannot judge scope agreement or test quality: maintainers still
review both. Owner and dependency-bot PRs need the same tracking issue; a
maintainer can edit a bot PR description to add it and the missing sections.
Drafts may fail until their description is complete.

The **CI gate** runs for every PR and verifies the suites selected from its changed
files, as described above. A first-time fork contribution
may wait for a maintainer to approve running Actions. This is expected, not a
request for credentials or repository write access. PR jobs receive no AWS
credentials and do not deploy. Edit the PR body to rerun the policy check; push
a new commit to rerun code checks, or ask a maintainer to rerun a transient failure.

## 5. Review and merge

When ready, leave draft mode and request review. Address feedback on the same
branch; keep the PR description and evidence current. Update from `main` when
required and resolve conflicts. A maintainer checks the issue's acceptance
criteria, source changes (especially workflows), tests, documentation, and
compatibility impact before merging. Passing CI does not guarantee acceptance.

With one maintainer, formal required approvals stay at zero because authors
cannot approve their own PRs. The owner still opens an issue and PR, reviews the
diff, records test evidence, and waits for all required checks. With a second
active maintainer, administrators can require one independent approval.

Prefer squash merging with a meaningful title. A completed issue closes through
the PR's closing keyword; leave partial tracking issues open. Delete the merged
feature branch when no longer needed. Merging source does not publish Go module
versions; [releasing modules](RELEASING.md) remains a separate decision.
