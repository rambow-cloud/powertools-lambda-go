---
title: Getting started with Powertools for Go Lambda
description: "Build your first Go Lambda with structured logging, then add optional OpenTelemetry tracing. Use CGO disabled and the provided.al2023 runtime."
---

# Your first Go Lambda

## Prerequisites and installation

Use Go 1.27 or newer. The Lambda executable runs on `provided.al2023`; it does not require Node.js or Python. Build with `CGO_ENABLED=0`.

Use explicit `json` tags on application structs. See [JSON decoding and encoding](#json-decoding-and-encoding) for the full serialization rules.

The latest stable cohort is [v1.1.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.1.0),
including Go 1.27 and the JSON v2 behavior described above. Use the
[README example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/README.md)
for a minimal application pinned to v1.1.0. See [version policy](VERSION_POLICY.md).

Install Logger and the Lambda runtime in your application's Go module:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.1.0
CGO_ENABLED=0 go get github.com/aws/aws-lambda-go@v1.55.0
```

## Create utilities once

Start with Logger. Save this complete program as `main.go` in your application;
it is included from the maintained README example.

--8<-- "README.md:first-lambda"

Create `appLog` before `lambda.Start` and use `appLog.WithContext(ctx)` inside the
wrapped handler. Input `{"name":"Ada"}` returns `{"message":"Hello, Ada"}` and
writes one structured log with `message: "Handling request"` and `name: "Ada"`.
An empty name returns an application error. No tracing collector is needed for
this Logger-only program.

## Add tracing

When you need traces, install the Tracer module and configure an OTLP collector:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/tracer@v1.1.0
```

The maintained repository example composes Tracer and Logger:

~~~go
--8<-- "examples/basic/main.go"
~~~

Put Tracer outside Logger so logs can read the active span. Both wrappers share
the same invocation identity. This example disables response capture; follow the
[Tracer guide](TRACER.md) for child spans and exporter configuration.

## Configure observability

These settings are useful starting points for the deployed function:

| Environment variable | Example | Purpose |
| --- | --- | --- |
| `POWERTOOLS_SERVICE_NAME` | `orders` | Default service name when no explicit option overrides it |
| `POWERTOOLS_LOG_LEVEL` | `INFO` | Structured log threshold |
| `POWERTOOLS_LOGGER_LOG_EVENT` | `false` | Keep full event logging disabled |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | OTLP/HTTP collector endpoint |
| `POWERTOOLS_TRACER_CAPTURE_RESPONSE` | `false` | Disable handler response metadata |

The example explicitly sets its service name to `hello`. Change that option or omit it to use the environment default.

A collector must actually be running at the configured OTLP endpoint. The library does not install a Lambda layer. To send traces to AWS X-Ray, deploy an appropriate collector extension with the `awsxray` exporter and its required execution-role permissions; follow [X-Ray through OpenTelemetry](XRAY_MIGRATION.md).

## Build and package

For the copied Logger-only program, build in your application directory:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -tags lambda.norpc -o bootstrap .
```

Use `GOARCH=amd64` for x86_64. Deploy the executable as `bootstrap` with
`provided.al2023`; the deployment ZIP must retain executable permissions.

To build and package both architectures of the maintained Logger/Tracer example,
use the repository's packaging helper. Obtain a checkout:

~~~sh
git clone --branch v1.1.0 https://github.com/rambow-cloud/powertools-lambda-go.git
cd powertools-lambda-go
~~~


Run the following commands from the repository root:

=== "PowerShell"

    ~~~powershell
    $env:CGO_ENABLED = '0'
    ./scripts/build.ps1
    ~~~

=== "Bash"

    ~~~bash
    export CGO_ENABLED=0
    GOOS=linux GOARCH=amd64 go build -trimpath -tags lambda.norpc -o dist/amd64/bootstrap ./examples/basic
    GOOS=linux GOARCH=arm64 go build -trimpath -tags lambda.norpc -o dist/arm64/bootstrap ./examples/basic
    go run ./tools/package
    ~~~

The packaging tool verifies static Linux ELF binaries, architecture metadata, and an executable `bootstrap` ZIP entry with mode `0755`.

| Lambda architecture | Artifact |
| --- | --- |
| `x86_64` | `dist/lambda-amd64.zip` |
| `arm64` | `dist/lambda-arm64.zip` |

Deploy the matching ZIP using your infrastructure tooling with runtime `provided.al2023`. Invoke with `{"name":"Ada"}`; the example returns `{"message":"Hello, Ada"}`. Deployment and cloud invocation are separate from local builds and CI.

## Local verification

The default runtime acceptance environment uses Docker. Follow the maintained [local integration runner](LOCAL_INTEGRATION.md) and [local acceptance scope](LOCAL_VALIDATION.md).

Outside Lambda, tracing is disabled unless you explicitly supply `tracer.WithLocalTracing(true)`. A local collector is still required to export spans. `POWERTOOLS_DEV=true` enables readable local logs and disables tracing.

Continue with [Logger](LOGGER.md), [Tracer](TRACER.md), or [Metrics](METRICS.md).

## JSON decoding and encoding

Maintained packages and examples use `encoding/json/v2` and `encoding/json/jsontext` directly. Go 1.27 provides these APIs without an experiment flag. JSON object member names must be unique, UTF-8 and escaped Unicode must be valid, and struct fields match JSON names case-sensitively. Give application structs explicit `json` tags that match their input.

Nil slices and maps encode as `[]` and `{}`. `omitempty` omits empty JSON values; use `omitzero` when a field's Go zero value should be absent. Map member order is unspecified unless an operation explicitly requires ordering. Strings do not escape HTML by default. Byte arrays and slices use Base64; `time.Duration` requires an explicit supported format. Custom encoding can implement `MarshalJSONTo(*jsontext.Encoder) error`; decoding can implement `UnmarshalJSONFrom(*jsontext.Decoder) error`. See the [JSON v2 API](https://pkg.go.dev/encoding/json/v2) for the complete current behavior. `json.RawMessage` and `json.Number` from `encoding/json` remain usable value types; maintained serialization calls use v2 APIs.

These rules describe JSON serialization owned by Powertools. Invocation input/output serialization performed by `aws-lambda-go` follows that SDK's encoding contract.
