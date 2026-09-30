# Your first Go Lambda

## Prerequisites and installation

Use Go 1.26 or newer. The Lambda executable runs on `provided.al2023`; it does not require Node.js or Python. Build with `CGO_ENABLED=0`.

Public module tags have not been released yet. Until they exist, work from a local checkout and use the repository's `go.work`, which connects the independent modules. After the initial source upload, obtain a checkout with:

~~~sh
git clone https://github.com/rambow-cloud/powertools-lambda-go.git
cd powertools-lambda-go
~~~

If you already have the local source tree, start in its root directory. Do not run `go get ...@v0.1.0` until that module's release tag exists. See [module installation and versioning](MODULES.md) for independent-module usage.

## Create utilities once

The example below is included directly from the maintained `examples/basic/main.go` source when the site builds.

~~~go
--8<-- "examples/basic/main.go"
~~~

Create Logger and Tracer before `lambda.Start` so configuration is reused across invocations. Put Tracer outside Logger, pass the invocation context into operations, and obtain a request logger through `l.WithContext(ctx)`. Both wrappers reuse the same Commons invocation identity.

Response capture is disabled in this example. Logging and trace metadata are explicit application decisions; avoid placing secrets in either.

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

Run from the repository root:

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

The default runtime acceptance environment uses Docker. Follow the maintained [local integration runner](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/integration/local/README.md) and [local acceptance scope](LOCAL_VALIDATION.md).

Outside Lambda, tracing is disabled unless you explicitly supply `tracer.WithLocalTracing(true)`. A local collector is still required to export spans. `POWERTOOLS_DEV=true` enables readable local logs and disables tracing.

Continue with [Logger](LOGGER.md), [Tracer](TRACER.md), or [Metrics](METRICS.md).
