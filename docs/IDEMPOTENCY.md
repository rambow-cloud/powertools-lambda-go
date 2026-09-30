# Idempotency

Reference: TypeScript v2.35.0. The Go implementation provides an independent module with a generic operation manager, typed Lambda wrappers, an optional local response cache, and a DynamoDB adapter. Redis/Valkey persistence is available through the separate [cache module](IDEMPOTENCY_CACHE.md). Full compatibility gates remain unfinished; see [IDEMPOTENCY_PLAN.md](IDEMPOTENCY_PLAN.md).

## Usage and dependency boundaries

Import `github.com/rambow-cloud/powertools-lambda-go/idempotency` for the lifecycle and `github.com/rambow-cloud/powertools-lambda-go/idempotency/dynamodb` for DynamoDB persistence. Both packages share one feature module/version. Core package imports use Commons and JMESPath; the DynamoDB package adds the service SDK and shared request identity middleware. Logger, Tracer, and Batch are composed by callers and are not module dependencies.

Create the SDK client, store, and manager once outside the Lambda handler. See [the runnable example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/idempotency/main.go).

```go
store, err := persistence.New(client, persistence.Options{TableName: "idempotency"})
if err != nil {
    return err
}
manager, err := idempotency.New(store, idempotency.Options{
    KeyPrefix: "process-order",
    EventKeyJMESPath: "id",
    PayloadValidationJMESPath: "amount",
    ThrowOnNoKey: true,
})
if err != nil {
    return err
}
handler := idempotency.WrapHandler(manager, processOrder)
```

`Execute` accepts a separate payload and a context-aware callback. Use it to select an argument or subset without changing the business function signature. `WrapHandler` uses the entire typed event as its payload. `batch.WithParser` can feed parsed records into the wrapper while retaining original SQS retry identifiers.

Each manager snapshots its configuration and owns its cache. Managers can share a persistence store with distinct `KeyPrefix` values. Queries, clocks, serializers, diagnostics, and store implementations must be safe for concurrent use. Lambda context/deadline is retained per call through the shared invocation context.

## Lifecycle

1. Resolve the payload projection and optional validation projection. A missing/null key query bypasses idempotency unless `ThrowOnNoKey` is enabled. False, zero, empty strings, and empty objects/arrays still participate by default; the strict option rejects the reference's broader missing-value set.
2. Check a completed local cache entry, if enabled, or acquire the key with an atomic store `Put`. DynamoDB uses `attribute_not_exists`, record expiry, and expired execution lease conditions with `ALL_OLD` conflict responses.
3. Validate an existing record's payload hash. Return an isolated decoded response for `COMPLETED`, reject `INPROGRESS`, and retry inconsistent acquisition up to two times. No automatic business retries are introduced.
4. Run the callback once after acquisition. On business error, delete the in-progress record; on success, persist the response and reset the response expiry from completion time. Optional replay hooks run only on stored responses.

`ExpiresAfter` defaults to one hour. A pointer distinguishes omitted configuration from an explicitly invalid zero. `POWERTOOLS_IDEMPOTENCY_DISABLED` uses shared extended boolean parsing and is read at construction. Local caching is disabled by default, uses the shared LRU, defaults to 1,000 entries, and retains only completed response snapshots. `ClearCache` does not delete persistence records.

The context deadline supplies an execution lease in Unix milliseconds. Without a deadline, a diagnostic is emitted and the full record TTL is the recovery boundary. Persistence operations retain caller cancellation; the library does not issue detached writes after timeout. A hard process termination cannot run cleanup. Recovering from an expired lease therefore remains essential.

## Keys and interoperability

The default key is `<prefix>#<Base64(MD5(canonical JSON))>`, matching the pinned source. The default prefix is the trimmed `AWS_LAMBDA_FUNCTION_NAME`; use an explicit operation prefix when different business operations share a store. Go does not infer a JavaScript function/decorator name. SHA-1, SHA-256, SHA-384, and SHA-512 are also available; a digest identifies payloads and is not an authentication signature.

`CanonicalJSON` recursively sorts object keys case-insensitively while retaining array order. It applies JavaScript array-index property enumeration, float64 number formatting, and JSON string escaping without HTML escaping. `json.RawMessage` preserves input object order where lowercase keys compare equally. Go maps have no insertion order, so case-colliding property names require a shared ordered input representation or an application serializer. The reference's `__proto__` omission is reproduced and covered explicitly.

`SerializeKey` can define a deliberate cross-language canonicalization contract. It applies to both identity and validation hashes. `KeyQuery` and `ValidationQuery` accept compiled expressions, including custom JMESPath functions. String expressions enable the existing Powertools JSON/Base64/gzip functions automatically.

DynamoDB response data is stored as native AttributeValues, including lists, maps, numbers, booleans and null, rather than a JSON-encoded string. The default columns are `id`, `status`, `expiration`, `in_progress_expiration`, `data`, and `validation`. Attribute names are configurable. A sort key uses a static partition key (`idempotency#<function-name>` by default) and the generated identity as the sort key. Reads are strongly consistent and conflict responses avoid an extra read when DynamoDB returns the old item. Numeric response tokens are preserved during decoding.

## Explicit differences and unfinished gates

- This is scoped JSON interoperability, not proof of every JavaScript type. Malformed UTF-8, lone UTF-16 surrogates, Unicode case conversion edge cases, custom JavaScript objects, BigInt, undefined values, and exhaustive floating-point formatting remain compatibility work. Arbitrary Go values that cannot be represented as JSON return an error.
- The pinned in-progress consistency check compares an epoch-millisecond deadline with `getUTCMilliseconds()`. Go compares complete Unix milliseconds. A missing item after a conflict also receives bounded reacquisition instead of exposing an unwrapped missing-item error.
- Key/query errors remain discoverable as `KeyError`; the TypeScript handler wraps some of these in a persistence error. Cleanup failures use `errors.Join` to preserve the original business error. Panics are cleaned up best-effort and rethrown with their original value.
- Go snapshots the key before execution and reuses one JSON marshaler snapshot during default key generation. A handler mutating its input cannot redirect completion or cleanup to another key. Replayed JSON responses are independently decoded rather than returning shared mutable cache objects.
- A composite DynamoDB read preserves the logical generated key in `Record.Key`; the pinned record object exposes the static partition key and a separate sort key. Physical storage keys remain the same.
- Update and delete use the pinned unconditional semantics. They do not add an ownership token or transactional fencing against a worker that continues after its lease expires. Callbacks must honor their deadline. Adding fencing across language versions needs a separate compatible storage contract and is not silently claimed here.
- Configuration rejects nonpositive TTLs, negative cache sizes, duplicate DynamoDB attribute names, and unknown hash algorithms. It uses explicit SDK injection rather than loading AWS configuration inside the store.
- Durable replay/platform integration, full DynamoDB bidirectional acceptance, live IAM/TTL/throttling behavior, and performance budgets remain open. The cache adapter's reference differences and service acceptance scope are documented separately. Local HTTP fixtures demonstrate composed behavior and SDK wire payloads, not service guarantees.

## Reference evidence

The pinned npm distribution is executed by [generate-idempotency.mjs](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/tools/reference/generate-idempotency.mjs). It generates 25 canonical key cases, 14 lifecycle scenarios, and two real DynamoDB SDK command sequences. The fixtures retain the raw JSON used for identity checks. See the checklist for which verification gates have actually passed.

Primary reference sources: [BasePersistenceLayer](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/idempotency/src/persistence/BasePersistenceLayer.ts), [IdempotencyHandler](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/idempotency/src/IdempotencyHandler.ts), [DynamoDBPersistenceLayer](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/idempotency/src/persistence/DynamoDBPersistenceLayer.ts), and [deepSort](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/idempotency/src/deepSort.ts).
