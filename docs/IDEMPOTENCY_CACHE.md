---
description: "Use Redis or Valkey persistence with Powertools Go Idempotency, including cache configuration, lease behavior and replay contracts."
---

# Redis and Valkey persistence

Import `github.com/rambow-cloud/powertools-lambda-go/idempotency/cache`. This optional adapter is an independent module/version so applications using only the Idempotency core or DynamoDB adapter do not import a Redis client. The adapter uses the pinned `github.com/redis/go-redis/v9 v9.21.0` client interfaces and the existing Idempotency record/lifecycle contract.

```go
client := redis.NewClient(&redis.Options{
    Addr: "cache.internal:6379",
    ContextTimeoutEnabled: true,
})
store, err := cache.New(client, cache.Options{})
if err != nil {
    return err
}
manager, err := idempotency.New(store, idempotency.Options{
    KeyPrefix: "process-order",
    EventKeyJMESPath: "id",
    PayloadValidationJMESPath: "amount",
    ThrowOnNoKey: true,
})
```

The caller owns the connected client, credentials, TLS, topology, timeouts, retries, and shutdown. Use connection settings appropriate to the deployment. The small `Client` interface accepts ordinary and cluster go-redis clients; Sentinel clients use the same client type. No background connection lifecycle or application credentials are created by the adapter.

## Storage and acquisition

The Redis key is the core-generated idempotency identity. Values are JSON objects with the reference's default `status`, `expiration`, `in_progress_expiration`, `data`, and `validation` fields. Options customize these attribute names. JSON response numbers remain raw numeric tokens when Go reads them.

Acquisition uses `SET NX EX`; a conflict reads the record. Live completed/in-progress records are returned to the core for replay, validation, or concurrency rejection. An in-progress record without an execution deadline remains locked until its overall expiration. An expired record, expired execution lease, or malformed JSON record follows the reference's orphan-recovery path. It acquires `<key>:lock` using `SET NX EX 10`. The lock expires naturally and is not deleted after successful recovery, matching the pinned cache adapter.

The recovery write additionally uses a single-key Lua compare-and-set against the exact value observed before locking. If another caller changed or deleted that value, recovery returns a conflict instead of overwriting it. The script references only the record key, so it does not require the record and `:lock` keys to share a cluster slot. This guard is a deliberate improvement over the reference's unconditional recovery write. It requires permission to execute `EVAL`, in addition to `GET`, `SET`, and `DEL`.

Completion resets the server TTL using the record's new expiration minus the current whole Unix second. Deletion removes the record key. Record expiration must produce a positive, representable TTL; a zero TTL is rejected instead of accidentally creating a non-expiring Redis key. Expiration and execution leases remain application timestamps; server TTL and application-clock behavior should be validated for the production topology.

## Reference differences

- Unlike TypeScript v2.35.0, Go does not recover an unexpired in-progress record solely because its execution deadline is absent or zero. This prevents overlapping operations without context deadlines from executing twice. Expired records and expired nonzero execution deadlines remain recoverable.
- The pinned TypeScript cache completion writer drops `validation`. Go retains it by default so repeated requests can validate completed responses. Set `OmitValidationOnSuccess: true` only when exact reproduction of that record shape is required; validation then cannot succeed against the omitted hash. Reference fixtures explicitly use this option for exact writer comparisons. Go cannot reconstruct a missing validation hash in an existing TypeScript record.
- Orphan recovery retains the reference lock protocol and adds the compare-and-set guard described above. Its successful write is compared structurally with the reference's final `SET`; a separate test checks that a newly completed record cannot be overwritten. This comparison does not claim command-level equality for recovery.
- A record disappearing after a failed `SET NX` returns a conflict without a snapshot, allowing the core's bounded inconsistent-state retry. The pinned cache adapter exposes a missing-item error in that case.
- Go rejects structurally invalid JSON records with `ConsistencyError`. Arbitrary primitive values, wrong field types, zero/absent timestamps, non-JSON JavaScript values, and exhaustive malformed-record behavior remain compatibility boundaries.
- Core completion and failure cleanup retain their existing unconditional semantics. This adapter does not add transaction fencing for business code that ignores its expired context. Refer to [Idempotency limitations](IDEMPOTENCY.md).

## Verification and remaining scope

`tools/reference/generate-idempotency-cache.mjs` executes the actual TypeScript v2.35.0 cache adapter and saves sixteen scenarios: fresh writes, live conflicts, expired/missing leases, expired records, malformed JSON, held locks, unknown status, disappearance, completion, reads/deletion, and custom attributes. It needs no Redis server.

The local Lambda suite uses a real digest-pinned Valkey container without published host ports. A Node development script uses `docker exec valkey-cli` as the reference adapter's client transport, writes a TypeScript record before the Lambda run, and reads a Go-written record afterward. The Lambda binary contains only Go code. These bridge scenarios cover native JSON responses and shared generated keys without payload validation; the reference's validation omission remains explicit above.

The suite also exercises warm replay, 32 concurrent conflicts per successful invocation, payload validation retained by Go, and orphan recovery. See [IDEMPOTENCY_PLAN.md](IDEMPOTENCY_PLAN.md) for completed acceptance gates. Redis server variants, real server-expiry timing, cluster failover/redirects, TLS/authentication, service throttling, full cross-language numeric/serialization cases, and benchmarks remain separate acceptance work.

Sources: [pinned TypeScript cache adapter](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/idempotency/src/persistence/CachePersistenceLayer.ts), [pinned go-redis module](https://github.com/redis/go-redis/blob/v9.21.0/go.mod), and [go-redis command interfaces](https://github.com/redis/go-redis/blob/v9.21.0/string_commands.go).
