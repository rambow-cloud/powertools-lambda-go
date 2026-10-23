---
description: "Read published Powertools for Go release notes, stable and prerelease versions, component changes and canonical GitHub Release links."
---

# Release notes

All 27 maintained modules share one version. [Browse all GitHub Releases](https://github.com/rambow-cloud/powertools-lambda-go/releases) for full changelogs.

## v1.3.0

2026-10-23 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.3.0)

### Features

- **Logger (Preview):** Attach source stacks with `WithStack`, add a logging-site stack to ordinary errors by default, and record handler panics before rethrowing the original value. `WithStackTrace(false)` and `WithPanicLogging(false)` disable these defaults. The Preview is included in v1.3.0 and main; v1.2.0 does not provide these APIs. ([#242](https://github.com/rambow-cloud/powertools-lambda-go/pull/242))

### Security and fixes

- Require **Go 1.27.3 or newer** for the patched standard library. Update tracing dependencies to `golang.org/x/net v0.61.0`, `golang.org/x/crypto v0.58.0` in the selected graph and gRPC v1.83.2; update Unicode text handling to v0.43.0. Rebuild deployed Lambda binaries with the patched toolchain. ([#256](https://github.com/rambow-cloud/powertools-lambda-go/pull/256))
- **Avro:** Replace the archived codec with the maintained `github.com/iskorotkov/avro/v2 v2.34.0`, retaining cumulative collection bounds and adding explicit codec allocation limits. ([#256](https://github.com/rambow-cloud/powertools-lambda-go/pull/256))
- **Protobuf:** Align truncated short-prefix diagnostics with patched protobufjs v7.6.6. Audit the pinned reference tooling against the official npm registry during CI. ([#260](https://github.com/rambow-cloud/powertools-lambda-go/pull/260))
- Preserve native boundary behavior in Signer, Data Masking/KMS, Avro, AppSync GraphQL and Bedrock error cases. See the consolidated Release for individual fixes. ([#233](https://github.com/rambow-cloud/powertools-lambda-go/pull/233))

### Performance and verification

- Reduce Logger filtering, correlation and normalization allocations; reuse bounded private snapshots for ordinary JSON-shaped data in Commons/JMESPath and decode Validation snapshots directly. JSON v2 remains the fallback for custom and other unsupported fast paths. ([#249](https://github.com/rambow-cloud/powertools-lambda-go/pull/249), [#253](https://github.com/rambow-cloud/powertools-lambda-go/pull/253))
- Publish one Release and 27 matching module tags. The deprecated X-Ray SDK adapter remains excluded. See [public publication acceptance](RELEASE_ACCEPTANCE_V1_3_0.json) and [scoped security acceptance](SECURITY_ACCEPTANCE_V1_3_0.json) for source identity, checks and evidence boundaries.

## v1.2.0

2026-10-13 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.2.0)

### Features

- **Validation:** Add opt-in [Draft 2020-12 and 2019-09 compilers](reference/JSON_SCHEMA_VALIDATION.md#optional-draft-2020-12-compiler) with registered references, input snapshots and native diagnostics. Default Draft 7 behavior remains unchanged. ([#212](https://github.com/rambow-cloud/powertools-lambda-go/pull/212), [#213](https://github.com/rambow-cloud/powertools-lambda-go/pull/213))

### Fixes

- **Logger:** Preserve native TextMarshaler values and errors without invoking a failed marshaler twice. ([#219](https://github.com/rambow-cloud/powertools-lambda-go/pull/219))
- **Idempotency:** Retain in-progress records until their execution deadline is strictly earlier than acquisition time. ([#217](https://github.com/rambow-cloud/powertools-lambda-go/pull/217))
- **HTTP:** Reject invalid response headers before output and close retained bodies once when conversion fails. ([#221](https://github.com/rambow-cloud/powertools-lambda-go/pull/221))
- **Commons:** Deduplicate NaN DynamoDB numeric set members and preserve numeric BOOL semantics for native Go values. ([#222](https://github.com/rambow-cloud/powertools-lambda-go/pull/222))
- **Metrics:** Reject metadata writes to closed invocation scopes before calling user serializers. ([#223](https://github.com/rambow-cloud/powertools-lambda-go/pull/223))
- **JMESPath:** Correct numeric conversion, JSON v2 serialization and null multi-select evaluation. ([#226](https://github.com/rambow-cloud/powertools-lambda-go/pull/226))
- **Validation:** Preserve null oneOf branch indices and empty property names in JSON v2 diagnostics. ([#215](https://github.com/rambow-cloud/powertools-lambda-go/pull/215))

### Changes

- Complete first-party JSON v2 API adoption and expand finite compatibility audits across utility modules. Go 1.27 remains required.
- Align all 27 maintained module versions and internal dependencies at v1.2.0. The frozen X-Ray SDK adapter remains excluded from new publication.
- Improve quickstart, utility guides and documentation discovery. See [publication acceptance](RELEASE_ACCEPTANCE_V1_2_0.json) for verified public consumers and source identity.

## v1.1.0

2026-10-10 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.1.0)

### Features

- **Logger:** Add opt-in [`WrapRawHandler`](reference/LOGGER.md#event-logging-and-child-configuration) to log incoming JSON before typed decoding, retaining unknown members, empty/null values and exact numeric tokens. ([#149](https://github.com/rambow-cloud/powertools-lambda-go/pull/149))

### Changes

- Align all maintained module versions and internal dependencies at v1.1.0.

## v1.0.0

2026-10-07 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.0.0)

### Features

- **HTTP Metrics:** Add opt-in request-count metrics with bounded route dimensions. ([#106](https://github.com/rambow-cloud/powertools-lambda-go/pull/106))
- **Logger:** Support streaming JSON marshalers and JSON v2 collection/error behavior while preserving exact numeric tokens. ([#111](https://github.com/rambow-cloud/powertools-lambda-go/pull/111))

### Fixes

- **Logger / Metrics:** Reject invalid log field names without partial output, preserve decoded number precision and omit unspecified EMF storage resolution.
- **Parameters:** Isolate SSM and Secrets Manager caches by effective request options; bypass caching for nonpositive lifetimes.
- **Batch / Idempotency Cache:** Return every retry identifier when a full batch fails; retain unexpired in-progress records without execution deadlines to prevent duplicate execution.
- **Parser:** Accept documented SES, Cognito and VPC Lattice inputs; preserve CloudFormation `PhysicalResourceId` and REST authorizer context.
- **HTTP:** Honor compression quality values, validate structured `+json` bodies and preserve empty 204/205/304 responses.
- **HTTP:** Preserve binary/preencoded bodies, cookies and repeated headers; bound error-handler redispatch and respect cancellation.
- **AppSync Events:** Propagate per-handler authorization errors after publication workers finish.
- **Data Masking:** Apply missing-field errors and warnings to erasure `Rules` as well as `Fields`.

### Breaking changes

- Require **Go 1.27** and adopt JSON v2: reject duplicate members and malformed Unicode, match typed field names case-sensitively and encode nil collections as `[]` / `{}`.
- **HTTP Metrics:** `New` accepts variadic `Options`. Update explicitly nonvariadic constructor function types; direct calls remain valid.

### Changes

- Publish the first stable v1 release with the [maintained API compatibility commitment](COMPATIBILITY.md) and package usage examples.

## v1.0.0-rc.1

2026-10-07 · Prerelease · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.0.0-rc.1)

### Features

- Preview HTTP request-count metrics and Logger streaming JSON support for the v1 API.

### Fixes

- **Batch / Idempotency Cache:** Preserve full-batch retry identifiers and prevent reacquisition of unexpired in-progress records without execution deadlines.
- **Parameters:** Separate caches by effective request options and bypass caching for nonpositive lifetimes.

### Breaking changes

- Require **Go 1.27** and strict JSON v2 decoding with case-sensitive typed field names and v2 collection defaults.
- **HTTP Metrics:** Change `New` to accept variadic `Options`; explicitly nonvariadic constructor function types need updating.

## v0.2.0

2026-10-05 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v0.2.0)

### Features

- Release all maintained modules at one shared version with aligned dependencies.

### Fixes

- **Parameters:** Correct UTF-8 replacement and strip leading BOMs from byte/binary transformations.
- **Parser:** Correct UTF-8 replacement and BOM handling in Base64 text and gzip JSON decoding.
- **JMESPath:** Decode malformed UTF-8 consistently in Base64 and gzip functions.
- **Signer:** Keep request bodies available until asynchronous transport uploads finish.

## v0.1.0

2026-10-04 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v0.1.0)

### Features

- Initial public release of 27 maintained modules, including Logger, OpenTelemetry Tracer, Metrics, Parameters, Parser, Batch and Idempotency.
