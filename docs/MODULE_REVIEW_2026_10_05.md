# Module review, 2026-10-05

The review used main commit `6d4157d24f84f7fa2b8a1cdeb94ad5bac8359a93`, following the unified-version release change. It covered the main entry points and critical state, ownership, error and decoding paths of the 26 maintained public modules other than Logger. Existing compatibility boundaries and pinned TypeScript fixtures were checked before classifying findings.

## Scope and baseline evidence

| Area | Reviewed paths |
| --- | --- |
| Commons and adapters | Environment/number conversion, cloning/merging, cache primitives, invocation identity, SDK user agent, DynamoDB decoding, metadata retrieval and regex matching/replacement |
| Metrics and tracing | Scoped state, serialization/publication, wrapper cleanup, context propagation, OTel ownership and HTTP adapters |
| Parameters and Signer | Cache invalidation, provider retrieval/batching, transforms, signing and transport body ownership |
| Batch and Idempotency | Partial/FIFO failures, acquisition/replay/cleanup, DynamoDB persistence and cache recovery |
| Parser, Validation and JMESPath | Schema composition, envelope extraction, compilation, operational errors, reference fixtures and text decoding |
| Kafka and event handlers | Record decoding, Avro/Protobuf adapters, AppSync/Bedrock dispatch, HTTP conversion, routing, middleware and streaming cleanup |

On Windows amd64 with Go 1.27.1 and CGO disabled, `tools/modules.py check --only ...` passed for all 25 selected modules that can execute natively on this host. Each packaged archive passed tests, vet, tidy verification and an independent consumer build with `GOWORK=off`. The exact selected modules and archive digests are recorded in [the baseline summary](MODULE_REVIEW_2026_10_05.json).

The remaining maintained module, `datamasking/kms`, requires Linux. The local Docker Linux engine did not respond within a bounded probe. A local workspace attempt to run the frozen `tracer/xray` regressions also stopped at an external dependency download. The [complete Linux CI run at the exact baseline commit](https://github.com/rambow-cloud/powertools-lambda-go/actions/runs/37219502507) passed all 31 repository modules, including KMS and frozen X-Ray tests, vet, tidy and consumer builds. Its Lambda example builds and static binary packaging passed for Linux amd64 and arm64. These hosted checks supply Linux baseline evidence; a fresh local Docker runtime suite was unavailable. No AWS cloud tests were run.

## Registered findings

All four issues were created before changing production code. The ordinary baseline tests passed; additional reproductions exposed gaps in body-lifetime and malformed-text coverage.

| Issue | Module | Confirmed behavior |
| --- | --- | --- |
| [#25](https://github.com/rambow-cloud/powertools-lambda-go/issues/25) | Signer | An identity or cloned custom signer shares the input body, which the wrapper closed before an asynchronous base transport finished using it. |
| [#26](https://github.com/rambow-cloud/powertools-lambda-go/issues/26) | Parser | Base64 decoding collapsed malformed byte runs and retained the plain-path BOM; gzip JSON decoding split truncated sequences incorrectly. |
| [#27](https://github.com/rambow-cloud/powertools-lambda-go/issues/27) | Parameters | Binary transforms collapsed malformed byte runs and retained BOM; BOM-prefixed byte JSON failed to parse. |
| [#28](https://github.com/rambow-cloud/powertools-lambda-go/issues/28) | JMESPath | Plain and gzip Base64 functions collapsed consecutive malformed subparts into one replacement character. |

Four deterministic Signer cases failed before its fix. [PR #29](https://github.com/rambow-cloud/powertools-lambda-go/pull/29) passed the packaged Signer check and all three required native PR checks, then was merged using `gh` at `b02d8dc47e8529ca6b6b61e217bf37b2e819df34`. Only afterward did the shared text-decoding fix begin, mapping issues #26–#28 to one problem-focused PR.

The text audit initially compared 53 inputs against the actual TypeScript 2.35.0 npm packages. It found 13 Parser, eight Parameters and six JMESPath differences. The committed regression corpus adds three byte-input cases: 20 Parser, 16 Parameters and 20 JMESPath cases, 56 total. The generator retains the reference's separate `TextDecoder` and `Buffer.toString` BOM behavior. All three affected packaged modules pass after replacing the divergent conversions with the existing Commons UTF-8 primitive. Parameters also checks Auto transforms and preservation of untransformed raw bytes.

## Reproduction and acceptance boundaries

After installing the pinned development dependencies, run `npm run fixtures:utf8` from `tools/reference` to regenerate the three UTF-8 fixtures. Run `CGO_ENABLED=0 uv run --no-project python tools/modules.py check --only parser --only parameters --only jmespath` from the repository root for their complete packaged verification. The Signer regression is `TestTransportBodyHandoff` in `signer/transport_body_test.go`.

This is a review of the stated paths with executable baseline and targeted regressions. Existing unfinished compatibility, service-integration and performance gates remain tracked in their module documents. Frozen X-Ray retains its migration warning and existing regression scope. Module versions stay at the current release baseline; the PR release-note entries will be accumulated by the release workflow.
