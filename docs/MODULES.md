---
title: Install Go Powertools modules
description: "Install Go Powertools modules for AWS Lambda. Choose utility import paths, understand independent dependencies and use the shared published release version."
---

# Install modules

Install only the utilities your application uses. Maintained modules share **one release version**; each has its own dependency graph and import path. The latest stable release is [v1.3.1](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.3.1).

## Add a utility

Use Go 1.27.3 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.1
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/metrics@v1.3.1
```

Import the packages normally:

```go
import (
    "github.com/rambow-cloud/powertools-lambda-go/logger"
    "github.com/rambow-cloud/powertools-lambda-go/metrics"
)
```

Adding Logger does not install all other utilities. Start with [your first Lambda](GETTING_STARTED.md) to see the code, response and logs.

## Choose a package

Prefix each suffix below with `github.com/rambow-cloud/powertools-lambda-go`.

| Import suffix | Purpose |
| --- | --- |
| `/commons` | Shared configuration, encoding, JSON and cache helpers |
| `/commons/awssdk` | Add Powertools identity to AWS SDK requests |
| `/commons/dynamodb` | Convert AWS SDK DynamoDB attribute values |
| `/commons/metadata` | Read Lambda execution-environment metadata |
| `/commons/regex` | Match and replace text with ECMAScript-style regular expressions |
| `/logger` | Structured logging |
| `/metrics` | EMF metrics |
| `/parameters` | Read and cache SSM, Secrets Manager, DynamoDB and AppConfig values |
| `/signer` | Sign HTTP requests with AWS Signature Version 4 |
| `/jmespath` | Query JSON documents and decode event envelopes |
| `/datamasking` | Mask selected fields or use an encryption provider |
| `/datamasking/kms` | Encrypt selected fields with the AWS Encryption SDK |
| `/kafka` | Decode Kafka record keys, values and headers |
| `/kafka/avro` | Decode Avro with an explicit schema |
| `/kafka/protobuf` | Decode Protobuf with a message descriptor or decoder |
| `/batch` | Process records and report partial failures |
| `/idempotency` | Replay completed operation results using DynamoDB persistence |
| `/idempotency/cache` | Use Redis or Valkey for shared idempotency records |
| `/parser` | Validate event data and return typed Go values |
| `/validation` | Validate payloads against JSON Schema documents |
| `/eventhandler/appsyncevents` | Handle channel publish and subscribe requests |
| `/eventhandler/appsyncgraphql` | Resolve GraphQL queries, mutations and batches |
| `/eventhandler/bedrock` | Implement Bedrock Agent functions |
| `/eventhandler/http` | Route HTTP requests and return JSON responses |
| `/eventhandler/http/metrics` | Record request counts, latency and HTTP failures |
| `/eventhandler/http/tracer` | Trace routes with OpenTelemetry |
| `/tracer` | OpenTelemetry tracing and instrumentation |

Commons is a package in the root module: install `github.com/rambow-cloud/powertools-lambda-go@v1.3.1`, then import `/commons`. Other utilities use the matching module suffix in their `go get` command. Parameters providers and the Idempotency DynamoDB adapter are subpackages of their parent module.

## One version, separate dependencies

Use the same published version when selecting several maintained modules. One project release contains their consolidated release notes. Go's nested-module tags and publication checks are maintainer details in [releasing](RELEASING.md).

The `examples`, `integration` and `tools` modules are development packages. The deprecated `tracer/xray` module is frozen and excluded from current releases; use the [OpenTelemetry migration guide](XRAY_MIGRATION.md).

## API documentation

Open a package on [pkg.go.dev](https://pkg.go.dev/github.com/rambow-cloud/powertools-lambda-go/logger) for exported types and runnable Go examples. Select the version installed by your application. Repository changes do not alter an already published module version.

See [module development](reference/MODULES.md) for workspace checks and [version policy](VERSION_POLICY.md) for release rules.
