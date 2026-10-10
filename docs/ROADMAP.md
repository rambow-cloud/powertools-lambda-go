---
description: "Track unfinished Powertools for Go capabilities, compatibility audits and phased GitHub implementation milestones."
---

# Implementation sequence

The owner-requested backlog in [#166](https://github.com/rambow-cloud/powertools-lambda-go/issues/166) inventories the current source and plans on **2026-10-10**, after stable **v1.1.0**. It contains **38 open work issues across six milestones**: 37 new issues and existing KMS issue #164. This update schedules work; it implements no feature.

The comparison remains pinned to TypeScript **v2.35.0**, commit `7bcc27b1574493f9452688673658f52b80c53847`. An optional injected compiler or newer producer protocol is labelled separately from a missing default feature. [Feature comparison](FEATURE_PARITY.md) describes implemented APIs; [CHECKLIST.md](CHECKLIST.md) records verified progress.

## Milestones

| Order | Milestone | Work issues | First slice and completion gate |
| --- | --- | --- | --- |
| 1 | [01 - Schema and regex extensions](https://github.com/rambow-cloud/powertools-lambda-go/milestone/1) | 10 | Start with opt-in Draft 2020-12, then 2019-09 and shared regex `v`. Review transformation/extension ownership before other additions; preserve default Draft 7 behavior. |
| 2 | [02 - Kafka registry wire formats](https://github.com/rambow-cloud/powertools-lambda-go/milestone/2) | 3 | Confluent version-0 raw frames first; Glue and GUID-header modes follow the explicit framing/resolver contract. Producer fixtures and lazy bounded decoding are required. |
| 3 | [03 - KMS bounded materials cache](https://github.com/rambow-cloud/powertools-lambda-go/milestone/3) | 1 | Review supported CMM interfaces first. Implement only a reviewed cache with finite limits and authenticated interoperability; retain SDK blockers explicitly. |
| 4 | [04 - Utility compatibility contracts](https://github.com/rambow-cloud/powertools-lambda-go/milestone/4) | 17 | Finish the finite per-utility contract matrices, fix reproduced defects and document intentional Go differences. Existing implementations remain available. |
| 5 | [05 - Runtime lifecycle and durable evaluation](https://github.com/rambow-cloud/powertools-lambda-go/milestone/5) | 2 | Real native SDK multi-worker acceptance first, then an isolated durable prototype and a go/no-go decision. Local evidence does not certify Managed Instances deployment. |
| 6 | [06 - Service and resource acceptance](https://github.com/rambow-cloud/powertools-lambda-go/milestone/6) | 5 | Local Redis topology and resource budgets, default-off service tooling and owner browser review. Actual AWS runs require a separately authorized task. |

Numbers describe the proposed sequence, not release versions or deadlines. Independent work can proceed while a dependency is blocked. Every implementation slice needs its own tests and review; acceptance is not postponed until milestone 06. Close a milestone only when its assigned work is resolved, or explicitly move deferred scope with a recorded reason.

## Missing and optional capabilities

These built-in capabilities are absent. Existing application-owned `Compiler`, decoder and masking-provider interfaces remain usable.

| Capability | Classification | Issue | Milestone |
| --- | --- | --- | --- |
| ECMAScript Unicode sets (`v`) | Missing shared regex syntax | [#167](https://github.com/rambow-cloud/powertools-lambda-go/issues/167) | 01 |
| Draft 2020-12 compiler | Optional injected-engine addition | [#168](https://github.com/rambow-cloud/powertools-lambda-go/issues/168) | 01 |
| Draft 2019-09 compiler | Optional injected-engine addition | [#169](https://github.com/rambow-cloud/powertools-lambda-go/issues/169) | 01 |
| Draft 4 compiler | Optional injected-engine addition | [#170](https://github.com/rambow-cloud/powertools-lambda-go/issues/170) | 01 |
| Draft 6 compiler | Optional injected-engine addition | [#171](https://github.com/rambow-cloud/powertools-lambda-go/issues/171) | 01 |
| `useDefaults` transformation | Optional returned-snapshot API; review required | [#172](https://github.com/rambow-cloud/powertools-lambda-go/issues/172) | 01 |
| `coerceTypes` transformation | Optional returned-snapshot API; review required | [#173](https://github.com/rambow-cloud/powertools-lambda-go/issues/173) | 01 |
| `removeAdditional` transformation | Optional returned-snapshot API; review required | [#174](https://github.com/rambow-cloud/powertools-lambda-go/issues/174) | 01 |
| `$data` constraints | Optional injected-compiler extension | [#175](https://github.com/rambow-cloud/powertools-lambda-go/issues/175) | 01 |
| Typed custom-keyword extension | Optional proposal; existing Compiler injection is supported | [#176](https://github.com/rambow-cloud/powertools-lambda-go/issues/176) | 01 |
| Raw Confluent version-0 framing and Protobuf index paths | Missing explicit producer-wire adapter | [#177](https://github.com/rambow-cloud/powertools-lambda-go/issues/177) | 02 |
| Raw Glue framing and bounded compression | Missing explicit producer-wire adapter | [#178](https://github.com/rambow-cloud/powertools-lambda-go/issues/178) | 02 |
| Confluent GUID-header resolution | Optional newer-producer addition; outside pinned parity | [#179](https://github.com/rambow-cloud/powertools-lambda-go/issues/179) | 02 |
| Bounded KMS materials cache | Missing provider feature; upstream CMM review required | [#164](https://github.com/rambow-cloud/powertools-lambda-go/issues/164) | 03 |
| Durable replay-aware integration | Optional experimental-SDK evaluation; no production commitment | [#198](https://github.com/rambow-cloud/powertools-lambda-go/issues/198) | 05 |

Historical design issues [#116](https://github.com/rambow-cloud/powertools-lambda-go/issues/116), [#117](https://github.com/rambow-cloud/powertools-lambda-go/issues/117), [#118](https://github.com/rambow-cloud/powertools-lambda-go/issues/118), [#119](https://github.com/rambow-cloud/powertools-lambda-go/issues/119) and [#120](https://github.com/rambow-cloud/powertools-lambda-go/issues/120) remain closed for their completed planning scope. New work does not treat those closures as implementation evidence.

The default TypeScript validator also rejects newer dialects and `$data`. They become available through injected engines; the default Go compiler must keep its current contract. See [Validation extension design](VALIDATION_EXTENSION_DESIGN.md). JTD and arbitrary JavaScript plugins remain outside the selected adapter scope; the default-validation/custom-compiler audits can propose separate issues after proving a concrete useful contract.

Raw Kafka producer envelopes are distinct from Lambda's transformed SOURCE/JSON delivery. Existing adapters must not strip an envelope twice. See [wire-mode design](KAFKA_WIRE_DESIGN.md).

[AWS documents no Go data-key caching support](https://docs.aws.amazon.com/encryption-sdk/latest/developer-guide/go.html). The available materials interfaces need review; a hierarchical keyring changes key management and is not an equivalent replacement. See [cache design](KMS_CACHE_DESIGN.md).

## Compatibility audits for implemented utilities

These issues track remaining contracts and verification, not absent utilities. Each audit must use a finite case/export matrix and classify outcomes as verified, intentional difference, reproduced defect or a separately tracked feature. No issue promises exhaustive JavaScript/Go equivalence.

| Implemented utility | Remaining audit | Issue |
| --- | --- | --- |
| `Commons`, `commons/awssdk`, `commons/dynamodb`, `commons/metadata` | Commons encoding and native-value | [#180](https://github.com/rambow-cloud/powertools-lambda-go/issues/180) |
| `logger` | Logger configuration, serialization and buffering | [#181](https://github.com/rambow-cloud/powertools-lambda-go/issues/181) |
| `tracer`, `eventhandler/http/tracer` | OpenTelemetry tracing lifecycle and capture | [#182](https://github.com/rambow-cloud/powertools-lambda-go/issues/182) |
| `metrics`, `eventhandler/http/metrics` | Metrics metadata and wrapper lifecycle | [#183](https://github.com/rambow-cloud/powertools-lambda-go/issues/183) |
| `parameters` | Parameters invalid-input, duration and provider | [#184](https://github.com/rambow-cloud/powertools-lambda-go/issues/184) |
| `batch` | Batch malformed input and completion-order | [#185](https://github.com/rambow-cloud/powertools-lambda-go/issues/185) |
| `idempotency`, `idempotency/cache` | Idempotency cross-language persistence | [#186](https://github.com/rambow-cloud/powertools-lambda-go/issues/186) |
| `jmespath` | JMESPath grammar, numeric and serialization | [#187](https://github.com/rambow-cloud/powertools-lambda-go/issues/187) |
| `parser` | Parser type, diagnostic and envelope | [#188](https://github.com/rambow-cloud/powertools-lambda-go/issues/188) |
| `validation` | Default Validation setup, reference and diagnostic | [#189](https://github.com/rambow-cloud/powertools-lambda-go/issues/189) |
| `eventhandler/http` | HTTP request, response and streaming contract | [#190](https://github.com/rambow-cloud/powertools-lambda-go/issues/190) |
| `eventhandler/appsyncevents` | AppSync Events native output and authorization | [#191](https://github.com/rambow-cloud/powertools-lambda-go/issues/191) |
| `eventhandler/appsyncgraphql` | AppSync GraphQL native scalar and batch | [#192](https://github.com/rambow-cloud/powertools-lambda-go/issues/192) |
| `eventhandler/bedrock` | Bedrock function resolver native/error | [#193](https://github.com/rambow-cloud/powertools-lambda-go/issues/193) |
| `kafka`, `kafka/avro`, `kafka/protobuf` | Kafka lazy codecs and schema/error | [#194](https://github.com/rambow-cloud/powertools-lambda-go/issues/194) |
| `datamasking`, `datamasking/kms`, `commons/regex` | Data Masking selector and transform ownership | [#195](https://github.com/rambow-cloud/powertools-lambda-go/issues/195) |
| `signer` | Signer URL, header and transport body | [#196](https://github.com/rambow-cloud/powertools-lambda-go/issues/196) |

## Runtime, service and resource gates

| Remaining work | Status | Issue | Milestone |
| --- | --- | --- | --- |
| Native SDK multi-worker lifecycle | Worker support exists; composed acceptance is missing | [#197](https://github.com/rambow-cloud/powertools-lambda-go/issues/197) | 05 |
| Redis/Valkey expiry, topology, TLS and recovery | Persistence exists; local distributed acceptance is missing | [#199](https://github.com/rambow-cloud/powertools-lambda-go/issues/199) | 06 |
| Per-utility resource budgets | Core benchmarks exist; broader Linux budgets remain open | [#200](https://github.com/rambow-cloud/powertools-lambda-go/issues/200) | 06 |
| Guarded opt-in AWS runner | Implementation of missing reviewed runner slices | [#201](https://github.com/rambow-cloud/powertools-lambda-go/issues/201) | 06 |
| Current-artifact managed-service acceptance | Historical results exist; remaining execution gates stay open | [#202](https://github.com/rambow-cloud/powertools-lambda-go/issues/202) | 06 |
| Published documentation browser review | Owner acceptance remains separate from CI/deployment | [#203](https://github.com/rambow-cloud/powertools-lambda-go/issues/203) | 06 |

The service matrix includes CloudWatch/OTel-to-X-Ray, Parameters services, DynamoDB/cache, Batch retries/checkpoints, AppSync, Bedrock, Kafka, KMS, LMDS and streaming/platform behavior. Live cases become separate service execution issues only after scenario and resource scope are selected. This backlog authorizes no AWS execution.

## Implementation rules

- Start from the issue's evidence and acceptance criteria; record API/dependency scope before implementing optional additions.
- Use issue → branch → PR → required checks/review → merge. Partial PRs use `Refs`; use `Closes` only after the issue's acceptance passes.
- Keep stable v1 defaults and independent modules. Root Commons remains free of external dependencies; reuse the shared invocation identity.
- Keep every Go subprocess CGO-disabled. Run packaged checks with `GOWORK=off`, and Linux amd64/arm64 builds where relevant. Local Docker is the default integration target.
- Use OpenTelemetry and the collector's `awsxray` exporter. Frozen `tracer/xray`, JavaScript decorators/prototypes and strict Go JSON differences are not new parity features.
- Record verified milestones in [CHECKLIST.md](CHECKLIST.md), including reused/skipped phases. Historical artifacts and local fixtures do not establish current AWS or browser acceptance.

HTTP OpenAPI generation was **Coming soon** in the pinned TypeScript runtime; it is not a verified missing v2.35.0 feature. Bedrock OpenAPI action groups are outside the implemented function-based resolver's baseline. Add a separate proposal if either is selected later.
