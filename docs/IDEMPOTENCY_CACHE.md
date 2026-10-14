---
description: "Use Redis or Valkey persistence with Powertools Go Idempotency, including cache configuration, lease behavior and replay contracts."
---

# Redis and Valkey persistence

Use an existing Redis or Valkey deployment as the persistence store for [Idempotency](IDEMPOTENCY.md).

## Replay a completed operation

Install `idempotency/cache@v1.2.0` and import the adapter as `cache`, the Idempotency core, and `github.com/redis/go-redis/v9` as `redis`. This example requires a reachable Redis-compatible server; configure its TLS and credentials for your deployment.

**Input**

```json
{
  "id": "ORD-123"
}
```

**Code**

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379", ContextTimeoutEnabled: true})
defer client.Close()
store, err := cache.New(client, cache.Options{})
if err != nil { return err }
manager, err := idempotency.New(store, idempotency.Options{
    KeyPrefix: "confirm-order", EventKeyJMESPath: "id", ThrowOnNoKey: true,
})
if err != nil { return err }
for range 2 {
    result, err := idempotency.Execute(ctx, manager, map[string]any{"id": "ORD-123"},
        func(context.Context) (string, error) {
            fmt.Println("Running business operation")
            return "confirmed", nil
        })
    if err != nil { return err }
    fmt.Println(result)
}
```

Run this inside a function returning `error`, with a context carrying the invocation deadline.

**Output**

```text
Running business operation
confirmed
confirmed
```

The second call uses the stored result. Use a fresh example key, or remove its prior record, when trying this demonstration.

## Configure the client

The adapter accepts standalone, Sentinel and cluster client implementations of its small client interface. Your application owns connection configuration, retries and shutdown. Configure `ContextTimeoutEnabled` so operation deadlines are observed.

## Things to know

- The store needs `GET`, `SET`, `DEL` and `EVAL` permissions for guarded recovery.
- Expiry and execution leases govern when interrupted work can be retried; use the Lambda invocation deadline.
- Completed payload validation is retained by default. Leave `OmitValidationOnSuccess` false for normal use.
- Production topology, failover and server-expiry behavior still need validation for your deployment.

## Next steps

[API and behavior notes](reference/IDEMPOTENCY_CACHE.md) · [HTTP](HTTP.md) · [Modules](MODULES.md)
