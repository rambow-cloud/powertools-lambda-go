# Signer assessment and implementation plan

Assessment date: 2026-09-14. Reference: TypeScript v2.35.0, commit `7bcc27b1574493f9452688673658f52b80c53847`. Signer is now implemented as an independent Go module. The assessment below records its dependency rationale; the current API and explicit compatibility differences are in [SIGNER.md](SIGNER.md).

## Priority

Recommend Signer immediately after the completed Commons/Metadata work, before JMESPath and Batch. Its public surface is small, it can reuse the AWS SDK for Go v2 SigV4 implementation already used by the integration handler, and it immediately supports custom HTTP requests to IAM-authenticated API Gateway, Lambda function URLs, and AppSync endpoints.

This is a useful standalone delivery, not a prerequisite for other utilities. JMESPath remains the next shared dependency: the pinned Idempotency and Validation manifests depend on it, and Logger declares it as a peer dependency.

## Dependency evidence

Read all 14 public `packages/<directory>/package.json` files at the pinned commit. None declares `@aws-lambda-powertools/signer` in dependencies, peerDependencies, or optionalDependencies. No public package declares it as a development dependency either. This is a manifest audit, not a claim that every example and test source has been searched.

| Audited directories | Signer dependency |
| --- | --- |
| commons, logger, tracer, metrics, parameters | None declared |
| batch, jmespath, idempotency, parser, validation | None declared |
| event-handler, kafka, data-masking | None declared |
| signer | Only `@smithy/signature-v4` and `@smithy/types` as runtime dependencies; no Commons dependency |

The Go dependency boundaries should remain equally small:

| Consumer | Relationship to the proposed Go Signer |
| --- | --- |
| Parameters SDK providers and future SDK-backed persistence | AWS SDK clients already sign their service requests. Do not insert a second signing layer. |
| Logger, Metrics, Commons, Metadata | No dependency required. Metadata uses its own bearer token protocol. |
| Tracer | Optional composition through `http.RoundTripper`; neither package needs to import the other. |
| Event Handler | Inbound routing does not require outbound signing. Applications may independently use Signer for downstream calls. |
| IAM-authenticated custom HTTP clients | Direct users of standalone signing or a signed HTTP transport. |

## Implementation shape and compatibility risks

Use one cohesive `signer` package with a small signing interface, a SigV4 implementation, typed errors, and an HTTP transport adapter. Reuse `aws.CredentialsProvider`, `aws/signer/v4`, and standard `net/http` types. Avoid separate Go packages solely to mirror TypeScript's `sigv4`, `fetch`, `errors`, and `types` export files.

Default credentials must follow the upstream Lambda environment provider, resolving again for each sign operation. Do not automatically load the full SDK configuration/provider chain: callers can supply a provider when they need profiles, role assumption, or cached credentials. The default path has no credential-discovery network calls; an injected provider may perform its own I/O. Region defaults to `AWS_REGION`; service is explicit. Shared Commons trimming must not silently change upstream raw environment semantics.

The pinned source has compatibility details that require fixtures before an implementation can claim parity:

- Duplicate query keys overwrite earlier values in the signing input, while the returned request retains the original URL. Record this upstream behavior and its effect on signature validity; do not silently claim that the Go SDK's query canonicalization is identical.
- Construction rejects missing/empty region; missing environment credentials fail during signing. Injected provider errors propagate directly in the current source despite broader wording in the documentation. Test actual error boundaries.
- Request bodies are buffered and hashed; read failures become signing errors. Define Go ownership and replay behavior explicitly because `http.Request.Clone` does not clone a body stream. Verify that signing preserves payload bytes and sending does not consume them twice.
- Verify escaped paths, host/port handling, repeated headers, payload hash headers, session tokens, and fixed signing times against actual TypeScript output.
- Preserve Go context cancellation. Define redirect and retry behavior so a signature for one request is not reused for a changed host, path, or payload. Keep any deliberate deviation from TypeScript documented.

## Checklist

- [x] S-00: Audit the 14 public manifests and read the pinned Signer implementation, fetch adapter, types, errors, and feature documentation; record priority and dependency boundaries.
- [x] S-01: Implement standalone signing and configuration/credential behavior, including typed errors. Covers upstream SIG-01 and SIG-02.
- [x] S-02: Implement request conversion, body replay, and canonicalization with thirteen fixed-time TypeScript fixtures. Twelve signatures match; duplicate-query handling intentionally preserves all values. Exhaustive URL/path behavior remains under SIG-03.
- [x] S-03: Implement the signed HTTP transport and verify injected transports, cancellation, redirects, concurrent credential retrieval, and optional Tracer composition. Covers SIG-04.
- [x] S-04: Add English usage examples for API Gateway, Lambda function URLs, and AppSync. Complete SIG-05 without claiming live IAM acceptance from synthetic tests.
- [x] S-05: Pass 14 module tests/vet, 11 independent consumer builds, both CGO-disabled Lambda architecture builds, and 114/114 Docker assertions including synthetic signature verification and OTel composition.
- [ ] S-06: Complete exhaustive URL/path/header normalization coverage, live IAM endpoint acceptance, and performance/body-memory budgets. Current differences remain explicit in SIGNER.md.

## Sources

- [Pinned package manifests](https://github.com/aws-powertools/powertools-lambda-typescript/tree/7bcc27b1574493f9452688673658f52b80c53847/packages)
- [Signer manifest](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/signer/package.json)
- [SigV4Signer source](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/signer/src/SigV4Signer.ts)
- [Signed fetch adapter](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/signer/src/fetch.ts)
- [Signer feature documentation](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/signer.md)
