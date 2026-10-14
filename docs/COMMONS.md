---
description: "Reuse dependency-free Powertools Go runtime primitives and one shared Lambda invocation identity across independent utilities."
---

# Commons

Shared helpers for configuration, encoding, JSON values and caches. Most applications use these through a feature module.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go@v1.2.0
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
	"fmt"
	stdlog "log"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

func main() {
	value, err := commons.FromBase64("SGVsbG8=", "base64")
	if err != nil {
		stdlog.Fatal(err)
	}
	fmt.Println(string(value))
}
```

**Output**

```text
Hello
```

## Read application settings

```go
enabled, err := commons.BoolEnv("FEATURE_ENABLED", true, false)
```

With `FEATURE_ENABLED=true`, `enabled` is `true`. The second argument enables extended values such as `1`, `yes` and `on`; the last argument is the fallback. Handle `err` for invalid configured values.

## Reuse a small cache

Commons includes a generic LRU cache. Use it when an application needs bounded, in-process storage; it is not a distributed cache. See [cache APIs](reference/COMMONS.md#encoding-merging-and-caches).

## Decode AWS SDK DynamoDB attributes

Use the optional `commons/dynamodb` module with AWS SDK attribute values. Import the SDK's DynamoDB `types` and the adapter as `dynamodb`. This snippet is the body of a helper returning `error`:

```go
item, err := dynamodb.UnmarshalItem(map[string]types.AttributeValue{
    "name":  &types.AttributeValueMemberS{Value: "orders"},
    "count": &types.AttributeValueMemberN{Value: "3"},
})
if err != nil { return err }
fmt.Println(item["name"], item["count"])
```

**Output**

```text
orders 3
```

For raw DynamoDB JSON without SDK types, use `commons.UnmarshallDynamoDB` instead.

## Add SDK request identity

The optional `commons/awssdk` module supplies a Smithy API stack option. Most utilities install their own identity automatically; custom integrations can use it directly:

```go
stack := middleware.NewStack("example", func() any { return nil })
err := awssdk.UserAgent("parameters")(stack)
_, installed := stack.Build.Get("PowertoolsUserAgent")
fmt.Println(installed, err)
```

**Output**

```text
true <nil>
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
