# Third-party sources and licenses

Original project contributions are distributed under the [MIT License](LICENSE), copyright (c) 2026 rambow-cloud contributors. Each independently versioned Go module carries a LICENSE and a self-contained NOTICE. This grant does not replace the licenses of third-party material or dependencies.

## Powertools TypeScript reference

The behavioral baseline is Powertools for AWS Lambda (TypeScript) v2.35.0 at commit [7bcc27b1574493f9452688673658f52b80c53847](https://github.com/aws-powertools/powertools-lambda-typescript/tree/7bcc27b1574493f9452688673658f52b80c53847), licensed under **MIT-0**. Its [LICENSE](third_party/licenses/powertools-typescript-LICENSE) and [NOTICE](third_party/licenses/powertools-typescript-NOTICE) are preserved without changing their original terms or copyright statements.

The Go implementation ports or adapts feature behavior, schema/constants, examples and reference expectations into native Go interfaces. Go context, concurrency, encoding and tracing choices are documented differences; the maintained Tracer uses OpenTelemetry. Reference generators execute the pinned npm packages and record observable outputs in testdata files. The npm lockfile pins the executable reference. This is an independent community project, not an official AWS distribution or an assertion of complete parity.

## Unicode data

The repository includes the original Unicode 16.0.0 PropertyAliases.txt and PropertyValueAliases.txt under tools/reference/unicode; their original copyright headers are retained. Their canonical URLs and SHA256 digests are also recorded in commons/regex/unicode_properties.json. The property table generator is tools/reference/generate-validation-unicode.mjs. The legacy case-fold table is produced by tools/reference/generate-regex.mjs using Node.js v22.21.1 and Unicode 16.0.

These data files and derived tables retain **Unicode-3.0**, with the full [official license](third_party/licenses/Unicode-3.0.txt) retrieved from [Unicode](https://www.unicode.org/license.txt) on 2026-09-26. The source data headers are dated 2024; the retrieved license carries Unicode's current copyright year. The root, commons/regex and tools NOTICE files include the complete permission notice. The project MIT license does not replace this data license.

## OpenTelemetry integration fixture

integration/build_collector.py downloads the pinned [OpenTelemetry Lambda layer-collector/0.23.0 release](https://github.com/open-telemetry/opentelemetry-lambda/releases/tag/layer-collector%2F0.23.0) and replaces its component factory with integration/collector_components.go.txt. The change selects OTLP reception and the AWS X-Ray exporter. The pinned [Apache-2.0 license](third_party/licenses/opentelemetry-lambda-LICENSE) is retained in the root and integration notices. Source archive and build hashes are recorded in docs/AWS_ACCEPTANCE.json. Downloaded sources, collectors and Lambda artifacts remain ignored local build outputs.

## Go Gopher artwork

The website's combined ram/cloud/Gopher artwork is stored in docs/assets/logo.png and docs/assets/logo-ram-gopher.png. The base rambow.cloud logo was supplied by the project owner. The Go Gopher character was created by Renee French and is licensed under [Creative Commons Attribution 4.0](https://creativecommons.org/licenses/by/4.0/); see the [Go project's attribution](https://go.dev/brand) and [The Go Gopher](https://go.dev/blog/gopher).

The Gopher is adapted through AI image editing: it is colored and posed to peek over the cloud, and a second version removes the wordmark and reframes the emblem. Website/asset use must retain the creator credit, source and license links, and indication of modifications. The documentation footer provides that attribution. The project MIT license does not replace these artwork terms.

## External dependencies

[THIRD_PARTY_DEPENDENCIES.json](docs/THIRD_PARTY_DEPENDENCIES.json) records the 29 distinct direct Go module/version requirements and 19 direct npm reference-tool requirements inspected for this source snapshot. Go entries include hashes of the inspected upstream license/notice files. npm entries retain their declared package licenses and repository metadata. Package-specific exceptions, such as AWS Lambda sample-code licensing, are recorded explicitly.

The inventories describe declared direct requirements, not the complete transitive graph selected for a particular executable. Dependencies are consumed through Go/npm; their source trees are not vendored. MIT, MIT-0, Apache-2.0 and BSD-licensed dependencies retain their own terms. Development containers, Node.js, the Go toolchain and downloaded collector dependencies are also separate upstream distributions. Before distributing executable binaries or collector layers, produce notices for the actual resolved dependency graph and retain required copyright, license and NOTICE files.

## Keeping module notices in sync

Run `uv run python tools/licenses.py` after changing the module manifest or canonical notices. Run `uv run python tools/licenses.py --check` to verify consistency. Full license and notice files are physically present in each module, so public Go proxy downloads do not rely on links outside the module archive. Local packaging verifies their presence before creating an archive.

Update the dependency inventory when requirements change; it is a dated provenance record, not an automated legal-compliance certification.
