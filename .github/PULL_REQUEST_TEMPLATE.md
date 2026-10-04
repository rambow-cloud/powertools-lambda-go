## Issue

<!-- Required for everyone, including the owner and dependency bots.
Use a real issue in this repository, one reference per line, outside this comment:
Closes #123
Use Refs #123 for partial work that should leave the issue open. -->

## Summary

<!-- Explain the change, why it is needed, and how it meets the agreed issue scope. -->

## Testing

<!-- List commands and actual results. Explain omitted or blocked checks.
Root go test ./... does not cover nested modules. Keep CGO_ENABLED=0. -->

## Risks and compatibility

<!-- Note API/dependency changes, operational effects, and remaining limitations.
Write "None identified" if appropriate. -->

## Release notes

<!-- One user-visible change per line, using a directory from tools/modules.json:
- logger | fix | Preserve temporary attribute lifetimes in child loggers.
Types: breaking, feature, fix, documentation, maintenance.
Use '.' for the root Commons module and 'repository' for repository-only work.
One PR can describe several changes in several modules. Write English summaries
for users, without test logs. If there is no release impact, use:
None: Explain why this change has no user-visible release impact.
Entries are collected when a module is released, not whenever a PR merges. -->

## Checklist

- [ ] The issue records the scope and acceptance criteria
- [ ] Tests and documentation match the change, or I explained why they are not applicable
- [ ] I kept CGO disabled and did not run cloud AWS tests without authorization
- [ ] I excluded credentials, personal configuration, and generated build artifacts
