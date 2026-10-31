---
description: "Reuse dependency-free Powertools Go runtime primitives and one shared Lambda invocation identity across independent utilities."
---

# Commons

Shared helpers for configuration, encoding, JSON values and caches. Most applications use these through a feature module.

## Install

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go@v1.3.1
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.1
```

## Decode Base64 text

Select Base64 decoding explicitly, then convert the returned bytes to a string.

**Input**

```text
SGVsbG8=
```

**Code**

```go
package main

import (
	"os"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
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
	value, err := commons.FromBase64("SGVsbG8=", "base64")
	if err != nil {
		return err
	}
	return appLog.Info("Text decoded", logger.Fields{"text": string(value)})
}
```

The Logger timestamp varies on each run.

**Output**

```json
{
  "level": "INFO",
  "message": "Text decoded",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "text": "Hello"
}
```

## Read application settings

```go
enabled, err := commons.BoolEnv("FEATURE_ENABLED", true, false)
```

With `FEATURE_ENABLED=true`, `enabled` is `true`. The second argument enables extended values such as `1`, `yes` and `on`; the last argument is the fallback. Handle `err` for invalid configured values.

## Reuse a small cache

Commons includes a generic LRU cache. Use it when an application needs bounded, in-process storage; it is not a distributed cache. See [cache APIs](reference/COMMONS.md#encoding-merging-and-caches).

## Decode AWS SDK DynamoDB attributes

Use the optional `commons/dynamodb` module with AWS SDK attribute values. Import the SDK's DynamoDB `types` and the adapter as `dynamodb`. These snippets run in a helper returning `error`, using the `appLog` created above:

```go
item, err := dynamodb.UnmarshalItem(map[string]types.AttributeValue{
    "name":  &types.AttributeValueMemberS{Value: "orders"},
    "count": &types.AttributeValueMemberN{Value: "3"},
})
if err != nil { return err }
return appLog.Info("Item decoded", logger.Fields{"item": item})
```

**Output**

```json
{
  "level": "INFO",
  "message": "Item decoded",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "item": {
    "name": "orders",
    "count": 3
  }
}
```

For raw DynamoDB JSON without SDK types, use `commons.UnmarshallDynamoDB` instead.

## Add SDK request identity

The optional `commons/awssdk` module supplies a Smithy API stack option. Most utilities install their own identity automatically; custom integrations can use it directly:

```go
stack := middleware.NewStack("example", func() any { return nil })
if err := awssdk.UserAgent("parameters")(stack); err != nil { return err }
_, installed := stack.Build.Get("PowertoolsUserAgent")
return appLog.Info("SDK identity installed", logger.Fields{"installed": installed})
```

**Output**

```json
{
  "level": "INFO",
  "message": "SDK identity installed",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "installed": true
}
```

Import `github.com/aws/smithy-go/middleware` and `commons/awssdk`. On SDK requests, the installed middleware adds the feature/version marker to `User-Agent`. This example installs the stack option without sending a request.

## Optional adapters

| Need | Module / guide |
| --- | --- |
| Lambda execution-environment metadata | [Metadata](METADATA.md) |
| SDK DynamoDB attribute values | `commons/dynamodb` |
| SDK request identity middleware | `commons/awssdk` |
| ECMAScript regular expressions | [Regular expressions](REGEX.md) |

## Things to know

- Root Commons has no external dependencies; optional SDK adapters are separate modules.
- Feature wrappers share one invocation identity for cold-start and request state.
- `FromBase64` returns bytes. Its `"utf8"` mode validates the encoded input but returns its UTF-8 text bytes instead of decoding it.

## Next steps

See the [API and behavior notes](reference/COMMONS.md) for advanced options, and [module installation](MODULES.md) for dependencies.
