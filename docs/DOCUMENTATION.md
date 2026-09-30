# Documentation and continuous integration

## Documentation structure

The site uses [MkDocs Material](https://squidfunk.github.io/mkdocs-material/), following the utility-oriented structure of the [TypeScript documentation](https://docs.aws.amazon.com/powertools/typescript/latest/). The English Markdown sources in `docs/` remain the source of truth.

- `mkdocs.yml` defines navigation, search, themes, code rendering, and strict validation.
- `website/pyproject.toml` and `website/uv.lock` isolate and lock documentation dependencies.
- `dist/site/` contains generated HTML and is ignored by Git.
- The quickstart includes the actual Go example through a checked snippet, avoiding a second copy.
- Links within the site use Markdown file paths; links to source outside `docs/` use GitHub URLs.

Every guide belongs in navigation. Builds fail on missing navigation targets, broken local links, unknown anchors, or missing included snippets. Existing implementation plans and sanitized acceptance records remain available under Development.

## Visual design

The theme follows the current TypeScript site's developer-guide layout: a slim dark utility bar, a white header, a persistent utility navigation tree, a readable article column, and an "On this page" rail. It uses the reference's 16px body / 42px page-heading scale and blue links, with coordinated light and dark palettes.

The reference is the [TypeScript documentation theme at commit 267c907](https://github.com/aws-powertools/powertools-lambda-typescript/tree/267c907a278fe3d5ee0e85b88bfe4b689923bca4/docs/stylesheets), inspected on 2026-09-30. The project-owned CSS and small Jinja template retain Material's existing search, theme selection, copy buttons, and responsive drawer instead of introducing a second UI framework.

- `docs/stylesheets/powertools.css` owns colors, typography, spacing, navigation rails, tables, code blocks, and responsive adjustments.
- `website/overrides/main.html` adds the organization bar, breadcrumbs, and guide label.
- `mkdocs.yml` keeps all utilities in one navigation tree; implementation plans stay inside Development.
- System Arial/Helvetica fonts replace Amazon Ember. No AWS logo, proprietary font, external stylesheet, or AWS portal script is bundled or hotlinked.

Preview the homepage, Logger, and Getting Started pages. At desktop width, inspect the three-column layout and section highlighting; at a 390px mobile width, inspect the drawer, search overlay, code/table scrolling, and theme toggle. Check keyboard focus and copy controls. Browser visual acceptance is separate from the strict build and remains pending until a rendered review is performed.

## Project artwork

The [full rambow.cloud logo](assets/logo.png) combines the project owner's ram/cloud identity with a Go Gopher. The [compact emblem](assets/logo-ram-gopher.png) is used in the header, navigation drawer, and favicon. Both use transparent backgrounds. Generation prompts and asset provenance are recorded in [website/BRANDING.md](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/website/BRANDING.md).

The Gopher is adapted from Renee French's design under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Keep the source, creator, license, and modification credit when reusing the artwork; the website footer includes that credit.

## Local preview

Install uv and run from the repository root:

~~~sh
uv run --project website --frozen mkdocs serve --config-file mkdocs.yml
~~~

Open **http://127.0.0.1:8000/powertools-lambda-go/**. Check the desktop/mobile navigation, search for Logger and Parameters, switch themes, and follow the quickstart and source links.

For the same strict build used in CI:

~~~sh
uv lock --project website --check
uv run --project website --frozen mkdocs build --strict --config-file mkdocs.yml
~~~

Python 3.14 is required for the isolated documentation environment. uv can provision it when permitted. The Go runtime library has no Python dependency.

To update the theme, change its exact version in `website/pyproject.toml`, run `uv lock --project website`, and review the resulting lockfile before running the strict build.

## Go CI

`.github/workflows/ci.yml` runs on pull requests, pushes to `main`, and manual dispatch:

1. Check module licenses and notices.
2. Verify the manifest's 31 packaged modules with tests, vet, tidy consistency, and 28 standalone public consumers using the existing local proxy harness.
3. Cross-compile the basic Lambda for Linux amd64 and arm64.
4. Validate static ELF binaries and executable bootstrap ZIP entries.
5. Retain example ZIPs and module verification progress as Actions artifacts for seven days.

Every job sets `CGO_ENABLED=0`. The harness disables the development workspace while validating independently packaged modules. Deprecated X-Ray adapter coverage is regression coverage only. CI does not create AWS resources or claim to run the separate Docker runtime acceptance suite.

## GitHub Pages

`.github/workflows/docs.yml` builds the site on pull requests, pushes to `main`, and manual dispatch. Pull requests only build an artifact. Only the canonical repository's `main` branch can deploy, using the `github-pages` environment.

The deployment job alone receives `pages: write` and `id-token: write`. It uses the official Pages configuration, artifact, and deployment actions. No personal access token is stored in the workflow, and no `gh-pages` branch is needed.

The canonical repository was configured with `build_type=workflow` on 2026-09-30. Its first source upload and deployment are still pending. To reproduce or inspect the setup:

1. Open [Settings > Pages](https://github.com/rambow-cloud/powertools-lambda-go/settings/pages).
2. Under Build and deployment, set Source to **GitHub Actions**.
3. Push the reviewed source to `main`, including the lockfile and both workflows.
4. Open [Actions](https://github.com/rambow-cloud/powertools-lambda-go/actions) and inspect the **Documentation** run and its **Deploy GitHub Pages** job.
5. Open **https://rambow-cloud.github.io/powertools-lambda-go/** after a successful deployment.

The initial source upload is separate from local setup. A successful local build does not mean the site is published, and the Pages URL is not available until the first successful deployment. No release tags are created by these workflows.

For another repository, update `site_url`, `repo_url`, `repo_name`, and the deployment repository guard together. Protect `main` and the deployment environment according to the organization's review policy.

## Action versions and updates

Actions are pinned to the complete commit for the latest official stable release checked on **2026-09-30**:

| Action | Release |
| --- | --- |
| [actions/checkout](https://github.com/actions/checkout/releases/tag/v7.0.1) | v7.0.1 |
| [actions/setup-go](https://github.com/actions/setup-go/releases/tag/v7.0.0) | v7.0.0 |
| [astral-sh/setup-uv](https://github.com/astral-sh/setup-uv/releases/tag/v10.2.0) | v10.2.0 |
| [actions/upload-artifact](https://github.com/actions/upload-artifact/releases/tag/v7.0.1) | v7.0.1 |
| [actions/configure-pages](https://github.com/actions/configure-pages/releases/tag/v6.0.0) | v6.0.0 |
| [actions/upload-pages-artifact](https://github.com/actions/upload-pages-artifact/releases/tag/v5.0.0) | v5.0.0 |
| [actions/deploy-pages](https://github.com/actions/deploy-pages/releases/tag/v5.0.1) | v5.0.1 |

`.github/dependabot.yml` proposes weekly GitHub Actions updates. Review and merge those updates to move the pins; the workflow does not follow mutable `main` or `latest` action references.
