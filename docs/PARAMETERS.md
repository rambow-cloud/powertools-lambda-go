---
description: "Retrieve and cache Go Lambda configuration from SSM, Secrets Manager, DynamoDB and AppConfig using Powertools Parameters."
---

# Parameters

Read and cache configuration from SSM Parameter Store, Secrets Manager, DynamoDB or AppConfig.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/parameters@v1.3.0
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

## Read an SSM parameter

Create a String parameter named `/orders/feature` with value `enabled`. This example calls SSM: configure the AWS SDK region and credentials, and allow `ssm:GetParameter` for that parameter.

**Input**

```text
/orders/feature = enabled
```

**Code**

```go
package main

import (
	"context"
	"os"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/ssm"
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
	value, err := ssm.GetParameter(context.Background(), "/orders/feature", ssm.GetOptions{})
	if err != nil {
		return err
	}
	return appLog.Info("Feature configuration loaded", logger.Fields{"feature": value})
}
```

The Logger timestamp varies on each run.

**Output**

```json
{
  "level": "INFO",
  "message": "Feature configuration loaded",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "feature": "enabled"
}
```

## Cache a value for longer

```go
value, err := ssm.GetParameter(ctx, "/orders/feature", ssm.GetOptions{
    Options: parameters.Options{MaxAge: parameters.Age(30 * time.Second)},
})
```

Repeated calls within the cache lifetime reuse the value. Set `ForceFetch: true` to bypass a cached value for that call. The default lifetime is five seconds.

## Decode JSON configuration

If `/orders/config` contains `{"enabled":true,"limit":10}`, request a JSON transform:

```go
value, err := ssm.GetParameter(ctx, "/orders/config", ssm.GetOptions{
    Options: parameters.Options{Transform: parameters.JSON},
})
```

The returned value is a decoded object with `enabled` and `limit`, rather than a JSON string. Import the root `parameters` package for transform constants.

## Choose a provider

| Source | Import suffix | Main operation | Required service permission |
| --- | --- | --- | --- |
| SSM | `/parameters/ssm` | `GetParameter` | `ssm:GetParameter` |
| Secrets Manager | `/parameters/secrets` | `GetSecret` | `secretsmanager:GetSecretValue` |
| DynamoDB | `/parameters/dynamodb` | Explicit provider | `dynamodb:GetItem` / `dynamodb:Query` |
| AppConfig Data | `/parameters/appconfig` | `GetAppConfig` | Session/configuration read permissions |
| AppConfig Agent | `/parameters/appconfigagent` | `GetConfig` | Agent-managed service access |

Use `New(client, ...)` when you need a custom SDK client, region or endpoint. See [provider options](reference/PARAMETERS.md#other-providers) for examples.

## Things to know

- Use the invocation context for service calls; default providers and their caches are reused across warm invocations.
- Reading SecureString parameters also requires decryption permissions. Grant only access to the configuration your function needs.
- Cache entries are local to an execution environment. Use `ClearCaches` or a provider’s `ClearCache` when explicitly invalidating them.

## Next steps

See the [API and behavior notes](reference/PARAMETERS.md) for advanced options, and [module installation](MODULES.md) for dependencies.
