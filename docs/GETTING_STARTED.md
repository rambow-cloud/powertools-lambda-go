---
title: Get started with Go Powertools Lambda
description: "Create a Go Lambda handler, see its JSON response and structured logs, then build a static executable for provided.al2023."
---

# Your first Go Lambda

Start with structured logs. This handler accepts a name, returns a greeting and writes one searchable JSON record.

## Install

Use Go 1.27 or newer and initialize your application:

```sh
go mod init example.com/hello
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.2.0 github.com/aws/aws-lambda-go@v1.55.0
```

## Write the handler

Save this as `main.go`:

--8<-- "README.md:first-lambda"

Create the logger once; obtain a request-bound logger inside the wrapped handler.

## Invoke it

**Input**

```json
{"name":"Ada"}
```

**Response**

```json
{"message":"Hello, Ada"}
```

**Log fields** (excerpt; timestamp and Lambda request fields vary)

```json
{"level":"INFO","message":"Handling request","service":"hello","name":"Ada"}
```

An empty name returns `name is required`. The logger does not turn a business error into a successful handler response.

## Build for Lambda

=== "Shell"

    ```sh
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -tags lambda.norpc -o bootstrap .
    ```

=== "PowerShell"

    ```powershell
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    $env:GOARCH = 'arm64'
    go build -trimpath -tags lambda.norpc -o bootstrap .
    ```

Use `amd64` for Lambda x86_64. Package the executable as `bootstrap` with executable permissions and configure the function for `provided.al2023` and the matching architecture. See [packaging notes](reference/GETTING_STARTED.md#build-and-package) for repository helpers.

## Add the utilities you need

| Need | Next guide |
| --- | --- |
| Log fields, levels and incoming events | [Logger](LOGGER.md) |
| Handler and outbound-call spans | [Tracer](TRACER.md) |
| Custom CloudWatch metrics | [Metrics](METRICS.md) |
| JSON HTTP routes | [HTTP](HTTP.md) |
| More modules | [Installation](MODULES.md) |

Tracing is optional. Configure a reachable OpenTelemetry collector before enabling export; use its `awsxray` exporter for X-Ray. The legacy X-Ray SDK adapter is deprecated and frozen.

## Things to know

- Keep `CGO_ENABLED=0` for builds and tests.
- Use explicit `json` tags on event/response structs. The project uses JSON v2 behavior; see [JSON notes](reference/GETTING_STARTED.md#json-decoding-and-encoding).
- Follow [usage patterns](USAGE_PATTERNS.md) when composing wrappers and sharing clients across invocations.
