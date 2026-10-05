# Maintainer setup

The [contribution guide](CONTRIBUTING.md) applies to the owner, other maintainers,
external contributors, and automated dependency updates. Files in Git define the
forms and checks; **GitHub settings must be enabled separately** to enforce
merge requirements. This guide does not claim those settings are already active.

## Bootstrap without an exemption

1. Create a real tracking issue for introducing this workflow. Before the new
   forms exist on `main`, use the existing issue editor and include the problem,
   proposed scope, and acceptance criteria in its description.
2. Publish the contribution-workflow feature branch and open a draft PR with
   that issue, Summary, and Testing filled in. There is no special owner,
   `no-issue`, label, or bootstrap bypass in the policy check.
3. Verify **PR contribution policy**, **CI gate**, and every selected suite on
   that PR's latest revision. The routing rollout must select all suites. Inspect the
   workflow diff before approving a fork workflow run.
4. Configure the protection below **before merging the bootstrap PR**, selecting
   the two stable checks from their observed successful runs. Do not require a check
   that has never run or was renamed. Confirm the PR's merge box applies the rules.
5. Merge the reviewed bootstrap PR through the protected PR interface. Forms
   become available once they are on the default branch; verify the New issue
   chooser. Verify missing-issue rejection and the fork flow in a follow-up PR.

The bootstrap follows the same issue, PR, checks, and protection requirements.
Waiting for observed checks before enabling protection avoids requiring a
nonexistent check; it does not exempt the initial merge or future changes.

## Protect `main`

Use [Settings → Rules → Rulesets](https://github.com/rambow-cloud/powertools-lambda-go/settings/rules)
to create an **active branch ruleset** targeting `main` (or use equivalent classic
branch protection). The repository is public, so a paid-plan feature is not
needed for basic PR and status-check protection.

- Require a pull request before merging; require **0 approvals** while there is
  only one active maintainer. Do not enable code-owner or last-push approval
  requirements until another eligible reviewer is available
- Require conversation resolution and these exact, stable status-check job names:
  **PR contribution policy** and **CI gate**. Select GitHub Actions as the expected source where
  available, and require the branch to be up to date before merging
- Block force pushes and deletion of `main`
- Leave the bypass list empty, including repository administrators. For classic
  protection, enable **Do not allow bypassing the above settings**
- Do not require **Deploy GitHub Pages**, Cloudflare previews, a cloud AWS test,
  a merge queue, or a paid review service. Deployment is not a pre-merge gate

Enable squash merging and automatic deletion of merged branches if desired.
When another maintainer can review owner-authored changes, raise approvals to
one and dismiss stale approvals after new commits. An automated review does not
replace a required human approval. See GitHub's
[protected branch guidance](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
and [author review limitation](https://docs.github.com/en/pull-requests/how-tos/review-pull-requests/approving-a-pull-request-with-required-reviews).

## Triage and safe CI

When introducing selective CI to an existing protected repository, first verify
the native full-suite PR run and **CI gate** on the latest commit. Replace the
individual conditional job requirements with **CI gate**, retaining **PR
contribution policy** and selecting GitHub Actions as the expected source.
Preserve strict branch freshness, review rules and the empty bypass list. Apply
the migration before merging the rollout PR. Follow with a metadata/documentation
PR to verify successful selected jobs, skipped Go/runtime jobs and a green gate.
The publication tool additionally requires actual module, runtime, documentation
and release-tooling success on the preparation PR and exact main commit;
missing, pending, failed, or skipped checks cannot publish.

Use issue comments to record accepted scope and remaining questions. Optional
labels are organizational only; no custom label creation, project board,
assignment, CLA, or signing service is required to start contributing. If a
project board becomes useful, use Backlog → Ready → In progress → In review →
Done, with type labels rather than a second set of status labels. Do not
close reports merely because a contributor cannot reproduce the maintainer's
full local environment.

Keep Actions' default token read-only. Retain fork workflow approval controls;
consider requiring approval for all external contributors if abuse becomes a
problem. Approve a fork run only after inspecting workflow and script changes.
Never enable secrets or write tokens for fork PR runs. No self-hosted runner is
needed. The policy job uses `pull_request`, no checkout, read-only API calls,
and treats PR text as data. It cannot protect against an approved malicious
workflow edit: review changes to `.github/` and validation tools carefully.
Do not replace this with `pull_request_target` that executes contributor code.

The policy workflow includes description edits and ready-for-review events;
CI runs for source updates and calls the selected suites. Avoid path filters on
required checks, because a skipped workflow can leave a PR waiting indefinitely.
The existing main-only deployment boundary stays unchanged. Adding a future
merge queue requires updating all required workflows for `merge_group` first.
See [fork workflow approval](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/approve-runs-from-forks)
and [required-check troubleshooting](https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks).

## Rollout verification

For unified version publication across independent modules, follow [Releasing modules](RELEASING.md).
Preparation automatically generates module versions, dependency metadata,
accumulated notes, a tracking issue, and a PR. The publisher reuses required
checks, binds the reviewed plan to the merged preparation SHA, and keeps the
tracking issue open until public consumers pass. With automatic publication
enabled in that plan, merging the preparation PR authorizes publication after
main checks pass; manual dispatch remains available for recovery. PRs require
structured Release notes or an explained absence;
edit dependency-bot descriptions using the same rule as other PRs.

Verify both an owner branch PR and a fork PR. Check the two required job names on the
latest commit, verify description edits refresh the policy check, and confirm
only `main` deploys. Keep one deliberately missing issue reference long enough
to confirm merging is blocked, then correct it. Check that a PR number and an
issue in another repository fail the policy check. Confirm the Issue forms and
PR template render in GitHub; local YAML/tests do not establish hosted behavior.
Do not make a destructive test push to `main`; inspect active rules and the PR's
merge box to verify enforcement.
