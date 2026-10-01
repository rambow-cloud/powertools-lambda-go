# Documentation and continuous integration

## Documentation structure

The site uses [Zensical](https://zensical.org/docs/) with its default modern theme and utility-oriented navigation. The English Markdown sources in `docs/` remain the source of truth.

- `mkdocs.yml` remains the configuration file supported by Zensical and defines navigation, search, themes, code rendering, and strict validation.
- `website/pyproject.toml` and `website/uv.lock` isolate and lock documentation dependencies.
- `website/check_navigation.py` checks that every Markdown guide appears exactly once in navigation and that every target exists inside `docs/`.
- `dist/site/` contains generated HTML and is ignored by Git.
- The quickstart includes the actual Go example through a checked snippet, avoiding a second copy.
- Links within the site use Markdown file paths; links to source outside `docs/` use GitHub URLs.

Every guide belongs in navigation. Builds fail on missing navigation targets, broken local links, unknown anchors, or missing included snippets. Existing implementation plans and sanitized acceptance records remain available under Development.

## Utility guide contract

The [feature comparison](FEATURE_PARITY.md) fixes the TypeScript baseline and maps implementation and evidence across all utility families. Main guides begin with a complete local or maintained Lambda example, explain input and observable output, identify objects and their lifetimes, and map the pinned TypeScript capabilities to Go. Advanced contract details follow those sections. Incomplete fragments must identify their required application symbols.

`website/check_guides.py` checks those sections, complete example presence, fixed-source attribution and referenced source-file existence for eighteen guides, including Commons. It checks documentation structure, not functional parity. Packaged tests and [the maintained Docker runner](LOCAL_INTEGRATION.md) establish the stated behavioral scope. Verify changed executable examples and compare their documented results before claiming acceptance.

Use site-local links for usage and verification pages. Use source links for code, fixtures and the fixed upstream version. Keep historical dates/counts in acceptance records or plans and link to them from user guides; do not mix past milestones into configuration instructions. [CHECKLIST.md](CHECKLIST.md) remains the project progress record: ordinary wording repairs need no entry, but verified feature audits and acceptance milestones do.

## Visual design

Zensical's bundled modern theme owns typography, spacing, title permalinks, page-edit actions, search, code copying, and responsive navigation. The previous custom CSS and template overrides have been removed. The configuration uses the theme's default fonts and icons, with the standard system/light/dark palette toggle.

`mkdocs.yml` keeps all utilities in one navigation tree; implementation plans stay inside Development. The project emblem and attribution remain configured through the theme's standard logo, favicon, and copyright settings.

Preview the homepage, Logger, and Getting Started pages. At desktop width, inspect the three-column layout and section highlighting; at a 390px mobile width, inspect the drawer, search overlay, code/table scrolling, and theme toggle. Check keyboard focus and copy controls. Browser visual acceptance is separate from the strict build and remains pending until a rendered review is performed.

## Project artwork

The [full rambow.cloud logo](assets/logo.png) combines the project owner's ram/cloud identity with a Go Gopher. The [compact emblem](assets/logo-ram-gopher.png) is used in the header, navigation drawer, and favicon. Both use transparent backgrounds. Generation prompts and asset provenance are recorded in [website/BRANDING.md](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/website/BRANDING.md).

The Gopher is adapted from Renee French's design under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Keep the source, creator, license, and modification credit when reusing the artwork; the website footer includes that credit.

## Local preview

Install uv and run from the repository root:

~~~sh
uv run --project website --frozen zensical serve --config-file mkdocs.yml
~~~

Open **http://127.0.0.1:8000/**. Zensical serves the local preview at the root path. The canonical Cloudflare site also uses the root path; the GitHub Pages mirror uses `/powertools-lambda-go/`. Check the desktop/mobile navigation, search for Logger and Parameters, switch themes, and follow the quickstart and source links.

For the same strict build used in CI:

~~~sh
uv lock --project website --check
uv run --project website --frozen python website/check_navigation.py
uv run --project website --frozen python website/check_guides.py
uv run --project website --frozen zensical build --clean --strict --config-file mkdocs.yml
~~~

Python 3.14 is required for the isolated documentation environment. uv can provision it when permitted. The Go runtime library has no Python dependency.

Zensical 0.0.67 is pinned in `website/pyproject.toml`; the lockfile includes its Windows and Linux wheels. To update the builder and bundled theme, change its exact version, run `uv lock --project website`, and review the resulting lockfile before running the clean strict build.

The migration retains `mkdocs.yml`, page URLs, and the checked Go snippet. The site now uses Zensical's default modern theme without custom CSS or template overrides. Zensical replaces the MkDocs and Material packages, so the Material warning about MkDocs 2.0 no longer applies. Link validation uses Zensical's `invalid_links` and `invalid_link_anchors` settings. Zensical does not implement MkDocs' navigation validation, so the navigation checker runs separately in CI. The snippets extension uses the repository root as its base path; run all documentation commands from that directory. Zensical is still in its 0.0.x release series; validate upgrades before deploying them. See the [official migration guide](https://zensical.org/docs/compatibility/mkdocs/migration/).

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

The canonical repository was configured with `build_type=workflow` on 2026-09-30. Go CI and Documentation, including GitHub Pages deployment, succeeded for commit `adc8f28` on 2026-10-02. To reproduce or inspect the setup:

1. Open [Settings > Pages](https://github.com/rambow-cloud/powertools-lambda-go/settings/pages).
2. Under Build and deployment, set Source to **GitHub Actions**.
3. Push the reviewed source to `main`, including the lockfile and both workflows.
4. Open [Actions](https://github.com/rambow-cloud/powertools-lambda-go/actions) and inspect the **Documentation** run and its **Deploy GitHub Pages** job.
5. Open **https://rambow-cloud.github.io/powertools-lambda-go/** after a successful deployment.

A successful local build does not establish remote deployment or browser acceptance. Inspect the deployment job for publication status and review the rendered site separately. No release tags are created by these workflows.

For another repository, update `site_url`, `repo_url`, `repo_name`, and the deployment repository guard together. Protect `main` and the deployment environment according to the organization's review policy.

## Cloudflare Pages

The canonical site URL is **https://powertools-lambda-go.rambow.cloud/**. The Pages project is `powertools-lambda-go`, with Git integration for `rambow-cloud/powertools-lambda-go` and production branch `main`. Zensical generates static HTML, so the site needs no Pages Functions or Go runtime in Cloudflare.

Use the following project build settings:

| Setting | Value |
| --- | --- |
| Framework preset | None |
| Repository root | Repository root; leave the dashboard field empty |
| Build output directory | `dist/site` |
| Build image | v3 |
| `PYTHON_VERSION` | `3.14` |
| `SKIP_DEPENDENCY_INSTALL` | `1` |
| `CGO_ENABLED` | `0` |
| `PYTHONDONTWRITEBYTECODE` | `1` |

The build command installs the fixed build tool and runs the same lock, navigation, utility-guide and strict build checks used by GitHub Actions:

~~~sh
python -m pip install uv==0.12.21 && python -m uv lock --project website --check && python -m uv run --project website --frozen python website/check_navigation.py && python -m uv run --project website --frozen python website/check_guides.py && python -m uv run --project website --frozen zensical build --clean --strict --config-file mkdocs.yml
~~~

Calling uv through its Python module also works when the Pages build environment does not add newly installed commands to `PATH`.

Install the Cloudflare Workers and Pages GitHub application with access to this repository before creating the Git-integrated project. Production pushes then build and deploy in Cloudflare independently of GitHub Actions. GitHub Pages remains a mirror. Git integration does not require a Cloudflare API token in GitHub Actions. Keep account identifiers and credentials out of the repository.

Under the Pages project's **Custom domains**, associate `powertools-lambda-go.rambow.cloud`. Then create the host's CNAME to the project's actual `*.pages.dev` address, or accept the record created by the dashboard. Associating the custom domain is required even when the CNAME already exists. See [Cloudflare's custom domain guide](https://developers.cloudflare.com/pages/configuration/custom-domains/).

Use Cloudflare's deployment and custom-domain status to verify publication and certificate activation. Open the canonical homepage, Logger and feature comparison pages in a browser to review rendering, search, internal navigation and source links. A local strict build does not establish custom-domain activation. Verified publication milestones belong in [project progress](CHECKLIST.md); browser acceptance remains separate.

The first successful publication and active domain/certificate status were verified on 2026-10-02. See the sanitized [Cloudflare acceptance record](CLOUDFLARE_ACCEPTANCE.json). The initial deployment was started manually; production builds are enabled for subsequent pushes to `main`.

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
