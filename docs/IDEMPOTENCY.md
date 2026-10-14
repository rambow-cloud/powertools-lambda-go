---
description: "Protect Go Lambda operations from duplicate execution with Powertools Idempotency, DynamoDB persistence."
---

# Idempotency

Return a stored result for repeated requests, so a retried Lambda invocation does not repeat a completed business operation.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/idempotency@v1.2.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
CGO_ENABLED=0 go get github.com/aws/aws-sdk-go-v2/config@v1.33.4
```

## Process an order once

This Lambda example uses DynamoDB. Set `IDEMPOTENCY_TABLE` to a table with a String partition key `id`, configure the AWS SDK, and allow `GetItem`, `PutItem`, `UpdateItem` and `DeleteItem`. The request ID selects the key; the amount is checked when replaying a result.

**Input**

```json
{
  "id": "ORD-123",
  "amount": 42
}
```

**Code**

```go
--8<-- "examples/idempotency/main.go"
```

**Output**

```json
{
  "order_id": "ORD-123"
}
```

Send the same request again while its record is valid: the response is replayed without calling the business function again.

| Request | Result |
| --- | --- |
| First `ORD-123`, amount `42` | Runs the operation and stores the result |
| Same request again | Returns the stored response |
| Same ID, amount `99` | Payload validation error |
| Overlapping request while processing | In-progress error |

## Choose the request key

```go
manager, err := idempotency.New(store, idempotency.Options{
    KeyPrefix:                 "process-order",
    EventKeyJMESPath:           "id",
    PayloadValidationJMESPath:  "amount",
    ThrowOnNoKey:               true,
})
```

Use a stable business identifier. Do not use the Lambda request ID: a retry receives a new one. Give different operations distinct prefixes when they share a table.

## Protect one operation

```go
result, err := idempotency.Execute(ctx, manager, event, func(ctx context.Context) (string, error) {
    return "confirmed", nil
})
```

Repeated calls for the same key return `"confirmed"` from persistence. Replace the callback with your business operation; handle `err` before using the result.

## Choose persistence

| Backend | Use |
| --- | --- |
| DynamoDB | Durable records across Lambda execution environments |
| [Redis / Valkey](IDEMPOTENCY_CACHE.md) | An existing shared cache deployment |
| Optional local result cache | Reduces persistence reads within one execution environment; it does not replace persistence |

## Things to know

- Create the SDK client, persistence store and manager once outside the handler.
- Pass a context with the invocation deadline so an interrupted operation has a recoverable execution lease.
- Business errors trigger cleanup; hard process termination cannot. Expiry and lease settings must match your retry policy.
- An optional DynamoDB TTL attribute is `expiration`; the library checks logical expiry without waiting for physical TTL deletion.

## Next steps

See the [API and behavior notes](reference/IDEMPOTENCY.md) for advanced options, and [module installation](MODULES.md) for dependencies.
