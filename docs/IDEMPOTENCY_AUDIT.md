# Idempotency persistence boundary audit

Issue [#186](https://github.com/rambow-cloud/powertools-lambda-go/issues/186)
defines a finite local audit against TypeScript Powertools 2.35.0. It covers
existing APIs and records, with JSON v2 serialization and CGO disabled. It does
not establish exhaustive JavaScript equivalence or live service acceptance.

| Boundary | Evidence | Classification |
| --- | --- | --- |
| Canonical keys, projections, validation and five hashes | Existing 25 key fixtures; six new array-index, Unicode, float64 limit and escaping fixtures in `TestCanonicalBoundaryReference` | Verified for this corpus |
| Native JSON inputs and one marshaler snapshot | Existing serialization tests; `TestKeyRejectsInvalidNativeValuesBeforePersistence` | Invalid Unicode, duplicate members, nonfinite/overflow numbers, trailing values and unsupported Go values fail before persistence; deliberate strict JSON v2 behavior |
| Completed/in-progress records, custom fields, null/false/zero/empty string/array/nested responses | Sixteen new rows per backend in `TestPersistenceBoundaryReference`; actual Go SDK wire writes and cache writes read by the pinned TypeScript adapters | Verified in both directions, with the cache validation difference below |
| Exact numeric response tokens | Existing DynamoDB `TestConsistentReadsAndConditionalConflict` and cache `TestValidationPreservedAndResponseIsolated` | Go retains integers above JavaScript's safe integer range; these are not a promise that JavaScript number consumers preserve precision |
| Validation, live/absent leases, invalid status and absent/malformed completed responses | `TestReplayValidationAndMalformedResponses`, with local cache enabled and disabled | No business callback runs for these stored records; errors retain their typed category and returned JSON snapshots are isolated |
| Lease equality and one millisecond either side | `TestLeaseEqualityCannotBeRecovered`, preserving the actual upstream equality takeover fixture | Reproduced and fixed: Go protects the lease at equality and recovers only strictly older leases, matching its Manager and DynamoDB condition |
| Recovery after another writer wins | Existing `TestRecoveryCannotOverwriteChangedRecord`; missing-deadline overlap tests | Verified compare-and-set guard and deliberate protection of records without a deadline |
| Retry limits, cancellation, failed completion and panic/error cleanup | Existing `TestFailureBoundariesAndRetries` and concurrent acquisition tests | Verified; unconditional completion/delete semantics still require callbacks to honor deadlines |
| Cache writer validation | Exact upstream fixtures with `OmitValidationOnSuccess`; new default-writer bridge | TypeScript drops validation on completion; Go retains it by default. Go does not reconstruct an omitted upstream hash |

The pinned fixture generator is
`tools/reference/generate-idempotency-boundaries.mjs`. Generate the fixture files
with Node; ordinary packaged Go tests use checked-in data and do not require Node.
To repeat the actual Go-to-TypeScript reads, create an ignored output directory,
set `POWERTOOLS_IDEMPOTENCY_BRIDGE_DIR` to its absolute path, run
`TestPersistenceBoundaryReference` in **both** the `idempotency` and
`idempotency/cache` modules with CGO disabled, then run:

```text
node tools/reference/generate-idempotency-boundaries.mjs verify OUTPUT_DIRECTORY
```

The bridge requires exactly sixteen rows from each backend and verifies 32 actual
reads. SDK requests use an injected local transport; no AWS endpoint is contacted.
The cache client is a deterministic test double, not Redis topology evidence.

Remaining scope stays explicit: JavaScript Unicode case-conversion edge cases and
exhaustive floating-point formatting are outside this corpus; case-colliding Go
map keys need an application serialization contract. Redis topology, real expiry,
TLS and failover remain in #199; service tooling/current-artifact acceptance in
#201/#202; resource budgets in #200. I-07/GAP-03 therefore remain open even when
this finite audit closes.
