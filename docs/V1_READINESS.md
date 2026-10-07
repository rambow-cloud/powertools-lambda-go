# v1 readiness and stable scope

The owner accepted issue-driven v1 preparation on 2026-10-07. Latest published
stable cohort: v0.2.0; published candidate: v1.0.0-rc.1. This plan defines the proposed scope; it does not certify an API
freeze or authorize publication. Track the full audit in [#112](https://github.com/rambow-cloud/powertools-lambda-go/issues/112).

## Maintained scope

All 27 maintained public modules in `tools/modules.json` release at one version
with independent paths/dependencies and prefixed tags. The frozen `tracer/xray`
adapter and three development modules are excluded. An API freeze must cover the
whole maintained cohort.

Source baseline: Go 1.27, direct JSON v2 APIs, CGO disabled, provided.al2023 on
Linux amd64 and arm64. SDK-owned invocation serialization follows aws-lambda-go;
library JSON rules apply to its own operations. OpenTelemetry is maintained
tracing. Root Commons stays dependency-free; utilities share one invocation identity.

[FEATURE_PARITY.md](FEATURE_PARITY.md) and utility guides define the supported
subset against TypeScript v2.35.0. Exhaustive JavaScript type, decorator and
serialization equivalence is separate work.

## Proposed compatibility commitment

After v1 publication, preserve documented exported interfaces, configuration
defaults, error categories, JSON/persistence formats and ownership/lifecycle
contracts within v1. Additive optional features can ship in later minor versions.
Fix incorrect behavior through reviewed changes and clear release notes;
incompatible changes require a separately reviewed major/module-path plan.

| Family | Review before freezing |
| --- | --- |
| Commons/adapters | Shared invocation identity, cloning/merge ownership, precision, environment parsing and isolation |
| Logger/Metrics/Tracer | Invocation isolation, closure/late writes, cold start, output/errors, precedence and injected resource ownership |
| Parameters/Metadata | Cache lifetime/invalidation, transform/missing errors, cancellation and SDK/HTTP ownership |
| Batch/Idempotency | Failure order/FIFO, conditional acquisition, expiry/lease units, replay formats, errors and cache ownership |
| Parser/Validation/JMESPath | Exported schemas/functions, validation versus operational errors, cancellation, compiled reuse and strict JSON/numbers |
| HTTP/middleware | Event fields, routes, body ownership, streaming close/errors, encoding and scoped observability |
| AppSync/Bedrock | Envelopes, routing, authorization/errors, scalar/parameter conversion and output order |
| Kafka/codecs | Lazy decode errors, JSON/native modes, framing, schemas and callback ownership |
| Masking/KMS | Immutability, selection/order, provider concurrency/errors, authenticated context and uncached operation |

The [public declaration inventory](V1_API_SURFACE.json) contains 35 packages and
823 declaration groups. The [API scope review](V1_API_REVIEW.md) compares the
v0.2.0 declarations, constructors/interfaces and documented contracts, reusing
critical-path/regression evidence. Its contract decision is reviewed through
this issue/PR; final publication remains a separate gate.

## Gates and evidence

| Gate | Tracking | Status |
| --- | --- | --- |
| Go 1.27/JSON v2 | [#110](https://github.com/rambow-cloud/powertools-lambda-go/issues/110), [PR #111](https://github.com/rambow-cloud/powertools-lambda-go/pull/111) | Merged; full PR CI passed 31 modules/28 consumers, both builds and local runtime/service scope |
| First-party pointer cleanup | [#107](https://github.com/rambow-cloud/powertools-lambda-go/issues/107), [PR #108](https://github.com/rambow-cloud/powertools-lambda-go/pull/108) | Merged: 28 packaged modules/26 consumers and DynamoDB Local in hosted CI; local package builds on both architectures. Lambda ZIP/runtime gates were skipped for #108 and passed separately in the candidate/main Full regression; transitive unsafe capabilities remain separate |
| Stable API scope review | [#112](https://github.com/rambow-cloud/powertools-lambda-go/issues/112) | Declaration/constructor/interface and documented contract review recorded in [V1_API_REVIEW.md](V1_API_REVIEW.md); publication commitment pending |
| Performance baseline | [#113](https://github.com/rambow-cloud/powertools-lambda-go/issues/113), [PR #125](https://github.com/rambow-cloud/powertools-lambda-go/pull/125) | Eleven measured scenarios and six packaged modules/consumers accepted; [Windows baseline and limits](PERFORMANCE.md) |
| Onboarding/version policy | [#48](https://github.com/rambow-cloud/powertools-lambda-go/issues/48), [#49](https://github.com/rambow-cloud/powertools-lambda-go/issues/49), [#51](https://github.com/rambow-cloud/powertools-lambda-go/issues/51) | [Policy](VERSION_POLICY.md) and isolated published-example verification |
| Sanitized service evidence | [#114](https://github.com/rambow-cloud/powertools-lambda-go/issues/114) | [Recorded scope](AWS_SERVICE_ACCEPTANCE.md), historical artifacts only |
| Remaining service claims | [#92](https://github.com/rambow-cloud/powertools-lambda-go/issues/92) | Reuse evidence, focus acceptance or state unsupported boundary |
| Candidate preparation/acceptance | [#115](https://github.com/rambow-cloud/powertools-lambda-go/issues/115), [#121](https://github.com/rambow-cloud/powertools-lambda-go/issues/121), [#129](https://github.com/rambow-cloud/powertools-lambda-go/issues/129) | v1.0.0-rc.1 published after #135/main Full regression and all 27 fresh public consumers passed; [public acceptance](RELEASE_ACCEPTANCE_V1_RC1.json) |
| Browser presentation | DOC-06 | Owner verifies deployed homepage/Logger/quickstart at desktop/390px, search/navigation/themes/keyboard/code copy |

Recent cloud examples verified CloudWatch extraction, both Lambda architectures,
DynamoDB claims/replay/Parameters, SSM and SigV4. Reuse these records. Remaining
Batch service retries/checkpoints, hard-timeout/freeze, cloud HTTP streaming,
real KMS and cache topology require focused acceptance or explicit support limits.
Additional cloud tests require explicit authorization, local profile/account
guards, ap-east-1, reviewed resources/cost scope and cleanup. #92 alone does not
authorize cloud calls.

## Post-v1 work

These extensions do not block the supported subset:

- [#116](https://github.com/rambow-cloud/powertools-lambda-go/issues/116): optional KMS data-key caching.
- [#117](https://github.com/rambow-cloud/powertools-lambda-go/issues/117): additional Kafka registry wire modes.
- [#118](https://github.com/rambow-cloud/powertools-lambda-go/issues/118): additional AJV dialect/extensions.
- [#119](https://github.com/rambow-cloud/powertools-lambda-go/issues/119): Managed Instances lifecycle.
- [#120](https://github.com/rambow-cloud/powertools-lambda-go/issues/120): platform durable execution; existing DynamoDB Idempotency replay is implemented.

Keep frozen X-Ray regressions and its migration warning. Transitive deps.dev
unsafe capability labels do not prove remaining first-party unsafe calls.

## Candidate and final publication

Candidate **v1.0.0-rc.1 is published** from `7eefb62fbde95a4d85eec1aee484a47e590a7d00` after
[PR #135](https://github.com/rambow-cloud/powertools-lambda-go/pull/135) and exact merged-main
Full regression passed. [Publication run 37630203444](https://github.com/rambow-cloud/powertools-lambda-go/actions/runs/37630203444)
verified all 27 fresh public consumers and finalized one project Release.
See [public acceptance](RELEASE_ACCEPTANCE_V1_RC1.json) for identities and checksums.
Browser presentation/pkg.go.dev review and stable v1.0.0 approval remain pending.
The preparation and publication gates below also apply to later versions.

The published v1.0.0-rc.1 cohort was prepared after implementation/scope gates passed.
Do not manually edit versions or reserve tags. Follow [RELEASING.md](RELEASING.md):
exact preparation/source identity, all modules/consumers, both Lambda builds,
runtime simulation, DynamoDB Local, documentation and release-tooling success.
Review notes and existing license/provenance checks.

The merged preparation [#135](https://github.com/rambow-cloud/powertools-lambda-go/pull/135) for [#129](https://github.com/rambow-cloud/powertools-lambda-go/issues/129)
used `--all --version v1.0.0-rc.1 --issue 129` without `--auto-publish`.
Its frozen plan shows `auto_publish: false`, `publication_mode: project`, all 27 maintained modules,
the exact source SHA and reviewed accumulated breaking/feature/fix notes.
The frozen adapter and development modules are not release components.
Preparation writes metadata and a reviewable PR; it creates no module tags.

Release metadata changes automatically select complete CI. Both the candidate
PR and its exact merged main commit must have a successful **Full regression**
marker, module/architecture artifacts, DynamoDB Local, runtime simulation,
documentation and pinned release-tooling checks. A skipped Full regression
marker or an earlier source SHA is insufficient. CI also checks all module
licenses/notices; ordinary source PR acceptance is not candidate acceptance.

Publication then uses dependency order and fresh public proxy/checksum consumers,
with `GOWORK=off`, `CGO_ENABLED=0` and no local proxy/replacement. Components
use independent tags at the same version and commit. One project Release and
consolidated notes remain draft until every consumer succeeds, then finalize
once. The publisher closes #129 only after that gate passes. These audited procedures complete
#115's tooling/scope review, not publication acceptance.

For DOC-06, open [the documentation site](https://powertools-lambda-go.rambow.cloud/),
[Logger](https://powertools-lambda-go.rambow.cloud/LOGGER/) and
[Getting started](https://powertools-lambda-go.rambow.cloud/GETTING_STARTED/).
At desktop and 390px widths, verify search results,
navigation, both themes, keyboard access and code copying. Record the result
in #129; a strict site build does not verify browser presentation.

After publication, verify fresh public proxy/checksum consumers with GOWORK off.
For each maintained module, open `https://pkg.go.dev/MODULE_PATH@v1.0.0-rc.1` and
check its overview, API links and examples. The root package uses
`github.com/rambow-cloud/powertools-lambda-go/commons@v1.0.0-rc.1`; Logger uses
`github.com/rambow-cloud/powertools-lambda-go/logger@v1.0.0-rc.1`.
Browser review is separate from builds. Candidate publication and public
consumers passed; browser review and final stable acceptance remain pending. Final v1.0.0 preparation
receives separate review; source merges and this plan do not publish modules.
