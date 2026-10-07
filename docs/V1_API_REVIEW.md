# v1 API scope review

Review date: 2026-10-07. Source `dbd70d5` contains 35 public packages and 823
declaration groups across all 27 maintained modules. Published v0.2.0 source
`31fb1ab` has the same 35 packages and 822 groups. The complete current surface
is in [V1_API_SURFACE.json](V1_API_SURFACE.json); Go documentation filtering
excludes internal/unexported, frozen and development APIs.

The review compares declarations, constructors/interfaces and documented
contracts, reusing the [critical-path review](MODULE_REVIEW_2026_10_05.md) and
subsequent issue-linked regression acceptance. It does not certify exhaustive
TypeScript equivalence or every application/service topology.

Stable **v1.0.0** was published on 2026-10-07 after the owner's explicit
authorization, preparation/main Full regression and all 27 public consumers.
The decisions below now define the maintained v1 compatibility commitment.
The declaration review retains its original source identity; the stable
promotion adds no Go source changes relative to the verified candidate.
See [stable acceptance](RELEASE_ACCEPTANCE_V1.json) for release commit `7be51eb55236`
and exact checks. Browser presentation remains a separate owner check.

## Differences from the published baseline

| Area | Reviewed difference | v1 decision |
| --- | --- | --- |
| HTTP Metrics | Optional variadic Options and CaptureRequestCount | Freeze the current option surface and default disabled request counting; #105 preserves default output |
| Parameters | Options.RequestKey and effective request-option cache isolation | Freeze key isolation and nonpositive-age bypass; clients remain application-owned |
| Idempotency | omitzero expiry tags and JSON v2 | Preserve expiry seconds, lease milliseconds, zero expiry omission and exact response numbers |
| HTTP/Kafka/Protobuf | RawMessage import aliases | Actual encoding/json.RawMessage types remain unchanged |
| Parser schemas | CloudFormation, Cognito and VPC Lattice corrections | Preserve current documented constraints/defaults and existing exported schema names |
| HTTP Response | Body encoding ownership comment | Preserve the existing fields and documented base64/reader policies |

Other public declaration shapes are unchanged. JSON v2 is the principal
cross-cutting behavior decision: strict duplicate/Unicode handling, exact field
matching, nil collections, omission and ordinary map order are explicit in the
guides. Key canonicalization and operation-specific ordering remain explicit.

## Stable contract decisions

- Use keyed fields for public configuration structs. Functional options and
  optional arguments are intended extension points. Existing function/interface
  shape changes after v1 require compatibility review.
- Preserve documented library error categories and cause unwrapping. Native
  SDK/Go JSON error details follow their dependencies; incidental stacks,
  locations and exact native messages are not a library promise.
- Constructor snapshots and invocation state remain distinct from caller-owned
  clients, transports, writers, callbacks and nested values. Guide-specific
  concurrency/immutability requirements are part of the contract.
- Logger/Metrics/Tracer and optional middleware retain invocation isolation,
  closure, cold-start and output/error policies. Pass context explicitly and
  join invocation-owned work before returning.
- Batch failure/FIFO contracts and Idempotency acquisition/replay/expiry formats
  remain stable. Logical expiry is independent of physical DynamoDB TTL removal;
  platform durable execution stays outside the declared subset.
- Parser/Validation/JMESPath retain reusable schemas/expressions, validation
  diagnostics versus operational errors, supported numeric rules and documented
  envelopes. Full AJV/JavaScript equivalence is separate scope.
- HTTP/AppSync/Bedrock preserve event/response envelopes, routing and error/body
  ownership. Kafka retains documented lazy decode/native/JSON/codec modes;
  unsupported registry/platform variants stay explicit extensions.
- Masking preserves input ownership, provider errors/concurrency and authenticated
  context. Optional KMS remains Linux-only and uncached; caching is #116.
- Preserve independent dependency graphs, dependency-free root Commons and one
  invocation identity. OTel is maintained tracing; frozen X-Ray keeps its
  migration warning/regressions and remains outside the release cohort.

## Acceptance and limits

The declaration comparison and constructor/interface review found no additional
API shape requiring redesign for the proposed supported subset. #111's full
hosted acceptance covers Go 1.27/JSON v2, all 31 modules/28 consumers, both Lambda
builds and scoped local runtime/service checks. The service-order assertion
finding is tracked under #122; it is corrected through its own issue/PR.
The six core performance modules have separate packaged acceptance under #113.

Stable v1 publication has started the compatibility commitment. Candidate and
stable public consumers passed; browser presentation remains separate in
[V1_READINESS.md](V1_READINESS.md). The review does not turn exhaustive native,
serialization or service milestones into completed functionality.
