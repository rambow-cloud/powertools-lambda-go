---
description: "Read published Powertools for Go release notes, stable and prerelease versions, component changes and canonical GitHub Release links."
---

# Release notes

All 27 maintained modules share one version. [Browse all GitHub Releases](https://github.com/rambow-cloud/powertools-lambda-go/releases) for full changelogs.

## v1.1.0

2026-10-10 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.1.0)

### Features

- **Logger:** Add opt-in [`WrapRawHandler`](LOGGER.md#event-logging-and-child-configuration) to log incoming JSON before typed decoding, retaining unknown members, empty/null values and exact numeric tokens. ([#149](https://github.com/rambow-cloud/powertools-lambda-go/pull/149))

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
