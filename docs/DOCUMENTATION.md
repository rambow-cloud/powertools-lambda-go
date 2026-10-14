---
description: "Build and maintain the Zensical documentation site, search and AI crawler discovery, GitHub Pages mirror and Cloudflare Pages publication."
---

# Documentation and continuous integration

## Documentation structure

The site uses [Zensical](https://zensical.org/docs/) with its default modern theme and utility-oriented navigation. The English Markdown sources in `docs/` remain the source of truth.

- `mkdocs.yml` remains the configuration file supported by Zensical and defines navigation, search, themes, code rendering, and strict validation.
- `website/pyproject.toml` and `website/uv.lock` isolate and lock documentation dependencies.
- `website/check_navigation.py` checks that every Markdown guide appears exactly once in navigation and that every target exists inside `docs/`.
- `dist/site/` contains generated HTML and is ignored by Git.
- The quickstart includes the minimal README program through a checked named snippet, avoiding duplicate code.
- `RELEASE_NOTES.md` lists published versions, newest first. Keep each entry to
  its version heading, publication date, prerelease status when applicable,
  canonical GitHub Release link and short user-facing change bullets. Use
  `Features`, `Fixes`, `Breaking changes` and `Changes` headings in that order,
  omitting empty categories. Summarize the reviewed Release notes with affected
  components, named APIs, corrected behavior and material upgrade requirements.
  Group related fixes without losing their meaning. Full changelogs belong in
  GitHub Releases; CI results, commit
  identities and checksums belong in acceptance records. Retain older versions
  and their anchors; keep drafts and unreleased changes out of this page.
- Links within the site use Markdown file paths; links to source outside `docs/` use GitHub URLs.

Every guide belongs in navigation. Builds fail on missing navigation targets, broken local links, unknown anchors, or missing included snippets. Existing implementation plans and sanitized acceptance records remain available under Development.

## Utility guide contract

Write for application developers. Each guide should answer: what does this utility do, what code should I write, and what will I see?

- Lead with a one-sentence purpose and installation, then a small task example.
- Show **Input**, **Code** and **Output** together. Use fenced JSON, text or HTTP results, rather than describing results only in prose. Collapse a full AWS event envelope when it would obscure a short example.
- Prefer one operation per example. Explain the few settings needed for that task; put large option tables, native edge cases and API mappings in `reference/`.
- Reuse existing Powertools modules for application concerns: use Logger for structured application logs, pass objects as `logger.Fields` and show the emitted record. Keep protocol-specific serialization, such as `protojson`, when required by the value's contract. In masking examples, log only the returned masked copy. Explain when successive examples use different per-call options.
- Mark excerpts and values that vary, such as timestamps, trace IDs and ciphertext. Label service prerequisites and illustrative responses honestly. Run local examples without AWS credentials; do not invent successful service calls.
- Describe the project as an independent community Go port. Keep pinned upstream versions, fixture counts and audit matrices in contributor provenance/plans, not in each user guide.
- Keep error handling, invocation lifetime, important limits and the deprecated X-Ray migration warning where they affect application decisions.

The official TypeScript [Logger](https://docs.aws.amazon.com/powertools/typescript/latest/features/logger/), [Batch](https://docs.aws.amazon.com/powertools/typescript/latest/features/batch/) and [Data Masking](https://docs.aws.amazon.com/powertools/typescript/latest/features/data-masking/) guides inform the task-based structure and visible results. Adapt examples to Go APIs and OpenTelemetry instead of copying language-specific runtime behavior.

Create a documentation issue before changing guides. If an upstream comparison reveals a missing capability, link an existing feature issue or create one with evidence and acceptance criteria. Distinguish missing features from intentional language differences and unverified behavior.

`website/check_guides.py` checks all 26 primary/optional-module guides for code, explicit inputs, fenced results, valid JSON and linked API notes. `website/test_guides.py` guards against replacing results with prose, broken references and repeated pinned-version boilerplate. A strict build validates links and included sources. These static checks do not execute Go or establish AWS service acceptance; record actual example execution in the implementation PR.

## Visual design

Zensical's bundled modern theme owns typography, spacing, title permalinks, page-edit actions, search, code copying, and responsive navigation. The configuration uses the theme's default fonts and icons, with the standard system/light/dark palette toggle. A small `extrahead` override adds discovery metadata and JSON-LD without changing the theme's layout or scripts.

`mkdocs.yml` keeps all utilities in one navigation tree; implementation plans stay inside Development. The project emblem and attribution remain configured through the theme's standard logo, favicon, and copyright settings.

`docs/stylesheets/home.css` styles only the homepage introduction, actions and utility cards. Guide typography and expandable reference sections use the bundled theme.

Preview the homepage, Logger, and Getting Started pages. At desktop width, inspect the three-column layout and section highlighting; at a 390px mobile width, inspect the drawer, search overlay, code/table scrolling, and theme toggle. Check keyboard focus and copy controls. Browser visual acceptance is separate from the strict build and remains pending until a rendered review is performed.

## Project artwork

The [full rambow.cloud logo](assets/logo.png) combines the project owner's ram/cloud identity with a Go Gopher. The [compact emblem](assets/logo-ram-gopher.png) is used in the header and navigation drawer. Dedicated transparent favicon assets use the same identity: [favicon.ico](favicon.ico) contains 16, 32, 48 and 256px frames; [favicon.png](favicon.png) is 96px square. The theme declares the ICO, and the head override declares the PNG. Relative links resolve to the site root on nested guides and preserve the GitHub Pages mirror's subpath. Generation prompts and asset provenance are recorded in [website/BRANDING.md](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/website/BRANDING.md).

To refresh the committed favicon files after approving new artwork, run `uv run --with pillow==12.0.0 python website/build_favicons.py`. Pillow is only needed for asset packaging, not the documentation build. The discovery checker validates copied assets, PNG dimensions, ICO frame headers, page declarations and Googlebot-Image robots access.

After production deployment, open **https://powertools-lambda-go.rambow.cloud/favicon.png** to inspect the image, open **https://powertools-lambda-go.rambow.cloud/favicon.ico** to confirm the file is available, and refresh both the homepage and **https://powertools-lambda-go.rambow.cloud/LOGGER/** to inspect their browser-tab icons. Request homepage indexing in Search Console if needed. Google processes favicons separately; [eligibility does not guarantee display](https://developers.google.com/search/docs/appearance/favicon-in-search), and local robots checks do not prove CDN access or Googlebot visits. Keep the favicon URLs stable.

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
uv run --project website --frozen python website/test_guides.py
uv run --project website --frozen zensical build --clean --strict --config-file mkdocs.yml
uv run --project website --frozen python website/check_discovery.py
uv run --project website --frozen python website/test_discovery.py
~~~

Python 3.14 is required for the isolated documentation environment. uv can provision it when permitted. The Go runtime library has no Python dependency.

Zensical 0.0.67 is pinned in `website/pyproject.toml`; the lockfile includes its Windows and Linux wheels. To update the builder and bundled theme, change its exact version, run `uv lock --project website`, and review the resulting lockfile before running the clean strict build.

The migration retains `mkdocs.yml`, page URLs, and the checked Go snippet. The site uses Zensical's default modern theme with a metadata-only template override and no custom CSS. Zensical replaces the MkDocs and Material packages, so the Material warning about MkDocs 2.0 no longer applies. Link validation uses Zensical's `invalid_links` and `invalid_link_anchors` settings. Zensical does not implement MkDocs' navigation validation, so the navigation checker runs separately in CI. The snippets extension uses the repository root as its base path; run all documentation commands from that directory. Zensical is still in its 0.0.x release series; validate upgrades before deploying them. See the [official migration guide](https://zensical.org/docs/compatibility/mkdocs/migration/).

## Search engines and AI discovery

The canonical site serves static HTML with the complete documentation text, including maintained code snippets, before JavaScript runs. `site_url` controls canonical URLs and the generated sitemap. Keep these URLs on the canonical Cloudflare hostname even when publishing the GitHub Pages mirror.

The metadata override in `website/overrides/main.html` adds index/follow and unrestricted snippet-preview directives to documentation pages, Open Graph and social-card metadata, and JSON-LD identifying the website and technical articles. The homepage also identifies the software source repository; articles link to that software identity. Its title omits the repeated site-name suffix. The 404 page uses `noindex, follow`. Main utility guides have individual descriptions in their Markdown front matter; other pages retain the configured site-description fallback. Structured data must describe the visible content: do not add invented ratings, authors, publication dates, AWS endorsement or unsupported compatibility claims.

### Project names and query relevance

The homepage, README and repository description identify **Go Powertools Lambda**, **Go Powertools** and **Powertools Lambda Go** as this community project's name variants. Keep the homepage's concise title, visible introduction/FAQ and canonical repository link useful to readers. Do not repeat all variants in every heading or create duplicate keyword landing pages. Google's [Search Essentials](https://developers.google.com/search/docs/essentials) recommends using the words readers search for in descriptive titles, headings and link text; its [title guidance](https://developers.google.com/search/docs/appearance/title-link) discourages keyword stuffing.

`extra.project_aliases` in `mkdocs.yml` supplies the same variants to WebSite/software `alternateName` and the generated `llms.txt` introduction. These aliases must also appear in visible homepage content. The discovery checker rejects generic/duplicated homepage titles, missing or explicitly hidden aliases, inconsistent structured names and broken article/software identity. It checks initial HTML and explicit hidden attributes, not computed browser styles or search ranking.

GitHub's repository website must point to the canonical documentation. Keep its description consistent with the community Go Lambda scope, and add relevant Go/Lambda/Powertools/observability topics while preserving existing topics. The README links to the homepage, quickstart and module guide. These are discoverable project references, not a guarantee of search authority or AI citations.

`docs/robots.txt` allows all crawlers through `User-agent: *` and advertises the canonical sitemap. This includes search engines, AI search crawlers, user-requested agents and training crawlers. In particular, OpenAI's [crawler documentation](https://developers.openai.com/api/docs/bots) distinguishes `OAI-SearchBot` for search from `GPTBot` for training. Allowing training is not a requirement for appearing in AI search. If the project later adopts a narrower policy, update the generator and its acceptance checks together.

`docs/llms.txt` provides an optional [LLM-friendly documentation index](https://llmstxt.org/), generated from navigation, Markdown headings and page descriptions. User guides come first; maintainer procedures, plans and dated evidence are under Optional, with explicit scope caveats. This entrypoint supplements the HTML site. Google's [AI search guidance](https://developers.google.com/search/docs/appearance/ai-features) requires conventional indexing and useful content, with no special AI file or schema requirement. Crawl access, sitemaps, metadata and this index do not guarantee indexing, rankings or AI citations.

After changing navigation, a heading, a page description, the repository URL or canonical site configuration, regenerate the tracked entrypoints before the strict build:

~~~sh
uv run --project website --frozen python website/check_discovery.py --write
~~~

The standard Zensical build copies both text files to the site root; no deployment-specific postprocessing is required. CI runs `website/check_discovery.py` after building to reject stale or missing source/built entrypoints, sitemap coverage errors, incorrect canonicals, indexing directives and inconsistent structured data. `website/test_discovery.py` tests accidental noindex, preview-host sitemap pollution, stale descriptions, crawler-blocked published assets and an indexable error page using a small copy of the build.

### Category pages and search-result navigation

The homepage links to five useful category indexes: [Getting started](getting-started/index.md), [Observability](observability/index.md), [Event handlers](event-handlers/index.md), [Data processing](data-processing/index.md), and [Security and configuration](security/index.md), with [Release notes](RELEASE_NOTES.md) as another prominent entry. Each index has its own title, description, canonical URL, task-oriented guide links and `CollectionPage` metadata. Existing guide URLs remain unchanged. The standard sitemap and generated llms.txt include these indexes; do not create a separate category sitemap or duplicate guide pages.

`navigation.indexes` connects these real index pages to their sections; `navigation.path` renders the theme's native visible breadcrumbs. Non-home pages declare `BreadcrumbList` JSON-LD using Home, linked section indexes and the current page, with canonical absolute URLs and consecutive positions. Sections without their own index are omitted from the structured trail rather than assigned fabricated URLs. The discovery checker verifies category entry links, homepage links, structured hierarchy and native category breadcrumb links.

After deployment, open **https://powertools-lambda-go.rambow.cloud/** and follow all five category links. Check **https://powertools-lambda-go.rambow.cloud/observability/** and **https://powertools-lambda-go.rambow.cloud/LOGGER/** on desktop and mobile; the latter should link back to Home and Observability above its title. Test Logger and a category URL with Google's [Rich Results Test](https://search.google.com/test/rich-results), then inspect the new category URLs in Search Console and monitor the Breadcrumbs report after indexing. Browser and Google validation remain owner checks.

Google chooses [sitelinks](https://developers.google.com/search/docs/appearance/sitelinks) automatically according to its understanding of the site and the query. Clear categories, useful titles and descriptive internal links support that understanding; no setting, sitemap field or structured-data type chooses the sitelinks Google displays. [Breadcrumb markup](https://developers.google.com/search/docs/appearance/structured-data/breadcrumb) describes a navigational path and does not force category links below the homepage result. Observed search appearance remains part of [visibility issue #206](https://github.com/rambow-cloud/powertools-lambda-go/issues/206).

### When a page is indexed but the target queries do not find it

An indexed URL and keyword visibility are separate outcomes. After the exact production deployment, use [Google Search Console](https://search.google.com/search-console) to inspect the canonical homepage, confirm Google's selected canonical and last crawl, and request a recrawl. Check Performance's Web results for the homepage with the three query filters `powertools lambda go`, `go powertools` and `go powertools lambda`; compare impressions, clicks and average position over equivalent date ranges. Sparse reports and a manual search alone do not prove the site is absent from every result.

Follow Google's [recrawl guidance](https://developers.google.com/search/docs/crawling-indexing/ask-google-to-recrawl): a request does not guarantee inclusion or immediate updates. Improve useful examples and genuine project references as the library grows; do not manufacture backlinks, ratings or AWS affiliation. Google [AI features](https://developers.google.com/search/docs/appearance/ai-features) depend on ordinary indexing and snippet eligibility, and require no special AI schema. Actual rankings, citations and referrals remain separately observed results.

[Visibility issue #206](https://github.com/rambow-cloud/powertools-lambda-go/issues/206) remains open for those owner-observed query and AI-citation outcomes after implementation issue #205 closes. A passing build or a successful direct URL fetch does not close that goal.

### Cloudflare crawler access

`robots.txt` expresses a crawling preference; CDN security can still prevent access. In the `rambow.cloud` zone, review Security settings and AI Crawl Control. Search and agent policies must allow the documentation host, and managed robots rules must not override the repository's all-crawler policy. Avoid crawler challenges or pay-per-crawl restrictions on public documentation. Keep protections for malicious traffic; a user-agent string alone does not establish a trusted bot identity. See [Cloudflare's crawler controls](https://developers.cloudflare.com/ai-crawl-control/features/manage-ai-crawlers/) and [robots.txt management](https://developers.cloudflare.com/bots/additional-configurations/managed-robots-txt/).

The read-only API inspection on 2026-10-10 found Bot Fight Mode, AI crawler protection and robots preference synchronization disabled, unmanaged robots.txt, and no custom firewall ruleset. No Cloudflare security configuration change was needed. This inspection establishes configured policy, not successful crawler visits or engine indexing.

### Publication and owner verification

Merge the reviewed changes to `main` to trigger the existing Cloudflare and GitHub Pages builds. After a successful production deployment:

1. Open [robots.txt](https://powertools-lambda-go.rambow.cloud/robots.txt). Confirm `User-agent: *`, `Allow: /`, the canonical Sitemap line and no added `Disallow: /` group.
2. Open [sitemap.xml](https://powertools-lambda-go.rambow.cloud/sitemap.xml) and [llms.txt](https://powertools-lambda-go.rambow.cloud/llms.txt). Confirm the canonical hostname and links to Logger, Tracer and Getting Started. Follow those links without encountering login or a challenge page.
3. Open [Logger](https://powertools-lambda-go.rambow.cloud/LOGGER/) and inspect View page source. Confirm its specific description, canonical URL, robots directives and JSON-LD. The article and examples must appear in the initial HTML.
4. In [Google Search Console](https://search.google.com/search-console), verify ownership of a URL-prefix property for `https://powertools-lambda-go.rambow.cloud/` using a supported account-specific method, submit `https://powertools-lambda-go.rambow.cloud/sitemap.xml`, and run URL Inspection's live test on the homepage and key guides. Request indexing and monitor Page indexing and Performance. A verified property for the GitHub Pages mirror does not establish ownership of this hostname.
5. In [Bing Webmaster Tools](https://www.bing.com/webmasters/), add or import the canonical site, submit the same sitemap and inspect indexing and crawl reports. Monitor Cloudflare AI Crawl Control and referred visits for AI crawler activity; successful crawling does not establish a search result or citation.

Ownership tokens are account-specific. Keep them out of generic examples and never substitute a fabricated token. Search-console verification, live browser acceptance, crawling and indexing remain owner checks after publication; a local build establishes none of them.

## Go CI

`.github/workflows/ci.yml` runs on pull requests, pushes to `main`, and manual dispatch.
It selects suites from the complete changed-file list; see [CI selection](CONTRIBUTING.md#how-ci-selects-checks).
When Go/runtime inputs change, it runs:

1. Check module licenses and notices.
2. Verify the manifest's 31 packaged modules with tests, vet, tidy consistency, and 28 standalone public consumers using the existing local proxy harness.
3. Cross-compile the basic Lambda for Linux amd64 and arm64.
4. Validate static ELF binaries and executable bootstrap ZIP entries.
5. Retain example ZIPs and module verification progress as Actions artifacts for seven days.
6. Run the [Docker runtime simulation](LOCAL_INTEGRATION.md), including streaming, Batch and KMS interoperability evidence.

Every job sets `CGO_ENABLED=0`. The harness disables the development workspace while validating independently packaged modules. Deprecated X-Ray adapter coverage is regression coverage only. CI does not create AWS resources. Documentation and issue-metadata changes skip these Go/runtime jobs.

## GitHub Pages

`.github/workflows/docs.yml` is called by CI when documentation or referenced source changes, and on full manual CI runs. It checks navigation, guides, links and the strict site build without installing Go or GoReleaser. Pull requests only build an artifact. Only the canonical repository's `main` branch can deploy, using the `github-pages` environment.

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

The first successful publication and active domain/certificate status were verified on 2026-10-02. See the sanitized [Cloudflare acceptance record](CLOUDFLARE_ACCEPTANCE.json). The initial deployment was started manually. Automatic publication was subsequently verified with an actual `github:push` deployment for commit `a39d92fac53cd13b66f6efd6abc864e48a9e9813`; its dependency, navigation, guide, strict-build and deployment checks all passed. Subsequent pushes to `main` publish automatically.

### Verify automatic publication

Push a reviewed documentation change to `main`, then inspect the Pages project's production deployments. The new deployment must show source `github:push`, the pushed commit SHA, and successful build and deploy stages. An enabled production-build setting or a successful manually started deployment does not establish that GitHub push events reach Pages.

The Cloudflare Workers and Pages GitHub application must include this repository in its repository access. An organization Owner manages that authorization. A CLI user's inability to manage or query the installation does not establish whether the application's access has already been granted. After an authorization change, verify the actual push-to-deployment path before checking the automatic-publication progress item.

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
