---
description: "Retrieve Lambda execution-environment metadata in Go with the optional Powertools Metadata client."
---

# Metadata

Read Lambda execution-environment information from the Lambda Metadata Service (LMDS).

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/commons/metadata@v1.2.0
```

## Read execution-environment metadata

This program prints the returned metadata object. Outside Lambda, it prints `{}` without sending a request.

**Input**

```text
Local process, outside Lambda
```

**Code**

```go
package main

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	stdlog "log"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons/metadata"
)

func main() {
	value, err := metadata.GetMetadata(context.Background(),
		metadata.Options{Timeout: 500 * time.Millisecond})
	if err != nil {
		stdlog.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		stdlog.Fatal(err)
	}
	fmt.Println(string(encoded))
}
```

**Output**

```json
{}
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
