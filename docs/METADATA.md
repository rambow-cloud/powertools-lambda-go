---
description: "Retrieve Lambda execution-environment metadata in Go with the optional Powertools Metadata client."
---

# Metadata

Read Lambda execution-environment information from the Lambda Metadata Service (LMDS).

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/commons/metadata@v1.3.0
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

## Read execution-environment metadata

This program logs the returned metadata object with Logger. Outside Lambda, metadata is empty and no request is sent.

**Input**

```text
Local process, outside Lambda
```

**Code**

```go
package main

import (
	"context"
	"os"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons/metadata"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

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
	value, err := metadata.GetMetadata(context.Background(),
		metadata.Options{Timeout: 500 * time.Millisecond})
	if err != nil {
		return err
	}
	return appLog.Info("Metadata retrieved", logger.Fields{"metadata": value})
}
```

The Logger timestamp varies on each run.

**Output**

```json
{
  "level": "INFO",
  "message": "Metadata retrieved",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "metadata": {}
}
```

## Read metadata in Lambda

Use the invocation context so timeouts and cancellation apply:

```go
value, err := metadata.GetMetadata(ctx,
    metadata.Options{Timeout: 500 * time.Millisecond})
```

**Example service result**

```json
{
  "AvailabilityZoneID": "example-az1"
}
```

The available fields and values depend on the execution environment. Unknown response fields are retained.

## Control caching

Successful nonempty results are cached across warm calls. Use `ClearMetadataCache()` to clear the default cache, or create an explicit client with `metadata.New(metadata.Config{...})` and call `ClearCache()` on that client.

## Things to know

- This is Lambda metadata, not the EC2 Instance Metadata Service.
- Logger and Tracer do not fetch it automatically.
- The default client uses `AWS_LAMBDA_METADATA_API` and `AWS_LAMBDA_METADATA_TOKEN`; never log the token.
- An unavailable service, timeout or malformed response returns an error.

## Next steps

See the [API and behavior notes](reference/METADATA.md) for advanced options, and [module installation](MODULES.md) for dependencies.
