# AppSync GraphQL implementation checklist

Reference: installed Powertools TypeScript v2.35.0 public AppSync GraphQL exports.
Keep full service, type and language compatibility gates separate from core work.

- [x] GQL-CORE: Map resolver/router/scalar exports, implement single/custom/Query/Mutation routing, registry inclusion and exact-name exception handling. Verify replacement ordering, framework exception bypass, original-error fallback and invalid event guards against actual TypeScript.
- [x] GQL-BATCH: Implement default aggregation, sequential individual execution, graceful null results, first-failure abort and first-event route selection. Preserve reference empty-batch and invalid-response failures.
- [x] GQL-SCALARS: Implement UUID v4 and date/time/timestamp helpers with explicit Go clock input, offset validation, fractional offsets and reference formatting. Verify 91 deterministic scalar cases and UUID shape/uniqueness.
- [x] GQL-REFERENCE: Verify 114 actual TypeScript resolver scenarios, ordered diagnostics/calls/errors, 64 concurrent contexts, reentrant diagnostics, panic/error identity and native aggregate arrays with CGO disabled.
- [x] GQL-LOCAL: Verify all 24 packaged modules and 21 standalone consumers, both CGO-disabled Linux architectures, the native Lambda example and Parser/Logger/OTel composition. Local acceptance passed 766/766 RIE, 95/95 streaming Runtime API and 14/14 saved Batch artifact checks (2026-09-22). Docker executed amd64; arm64 was cross-compiled. The verified GraphQL ZIP matches every current module file. Temporary resources were cleaned; no AWS resources were used.
- [x] GQL-ERROR-TYPE: Preserve the concrete TypeError for empty batches so the Go Lambda SDK reflects the reference Runtime API error name. Verify its concrete type and the existing differential scenarios (2026-09-23).
- [ ] GQL-COMPLETE: Finish public declaration/native-type mapping, typed event adapters, malformed Unicode and serialization boundaries, decorator/scope/async mappings and scalar nanosecond/out-of-domain behavior. Audit remaining native Runtime API error forms. Document intentional Go differences without treating them as complete parity.
- [ ] GQL-SERVICE: Verify actual AppSync single/batch invocation, service response/error representation and authorization context when cloud testing is requested. Establish latency/allocation/payload budgets and complete publication requirements.
