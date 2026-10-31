---
description: "Process SQS, Kinesis and DynamoDB Streams batches in Go Lambda with Powertools partial failures, FIFO policies and typed handlers."
---

# Batch processing

Process SQS, Kinesis and DynamoDB Streams records individually and return the IDs that Lambda should retry.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/batch@v1.3.1
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.1
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## Retry only failed SQS messages

One message contains a valid order; the other has the wrong field type. Enable `ReportBatchItemFailures` on the SQS event source mapping to use the partial response.

**Input**

```json
{
  "Records": [
    {
      "messageId": "m1",
      "body": "{\"id\":\"ORD-123\"}"
    },
    {
      "messageId": "m2",
      "body": "{\"id\":123}"
    }
  ]
}
```

**Code**

```go
package main

import (
	"context"
	json "encoding/json/v2"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/batch"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

type Order struct {
	ID string `json:"id"`
}

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	if err := run(appLog); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func run(appLog *logger.Logger) error {
	processor, err := batch.NewSQS[string](batch.Options{})
	if err != nil {
		return err
	}
	handler := func(_ context.Context, record events.SQSMessage) (string, error) {
		var order Order
		if err := json.Unmarshal([]byte(record.Body), &order); err != nil {
			return "", err
		}
		// Replace this with your business operation.
		return order.ID, nil
	}
	lambda.Start(batch.WrapSQS(processor, handler))
	return nil
}
```

**Output**

```json
{
  "batchItemFailures": [
    {
      "itemIdentifier": "m2"
    }
  ]
}
```

Only `m2` is reported for retry. The successful order ID is a business result, not part of the SQS response.

## Limit concurrency

```go
processor, err := batch.NewSQS[string](batch.Options{MaxConcurrency: 5})
```

Use `Sequential: true` when callbacks must run one at a time. The processor waits for all started callbacks before returning.

## Process records directly

Use `Process` when you need successful business results as well as the Lambda response:

```go
report, err := processor.Process(ctx, records, handler)
```

`report.Response` contains retry identifiers; `report.Results` keeps results in input order. Handle both the report and `err`.

## Choose a source

| Source | Constructor | Retry identifier |
| --- | --- | --- |
| SQS Standard | `NewSQS` | Message ID |
| SQS FIFO | `NewSQSFIFO` | Message ID, with FIFO stop/skip behavior |
| Kinesis | `NewKinesis` | Sequence number |
| DynamoDB Streams | `NewDynamoDB` | Sequence number |

For FIFO queues, review [FIFO processing](reference/BATCH.md) before changing ordering or group policies.

## Things to know

- A returned callback error marks that record as failed. Successful calls are not retried by this response.
- By default, all records failing produces `FullBatchFailureError` as well as a populated report. `SuppressFullBatchFailure` keeps the response without that error.
- A Lambda invocation that returns an error still fails the whole batch, regardless of a partial response.
- Create the processor once; use the invocation context inside callbacks.

## Next steps

See the [API and behavior notes](reference/BATCH.md) for advanced options, and [module installation](MODULES.md) for dependencies.
