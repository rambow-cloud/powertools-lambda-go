# Your first Go Lambda

## Prerequisites and installation

Use Go 1.27 or newer. The Lambda executable runs on `provided.al2023`; it does not require Node.js or Python. Build with `CGO_ENABLED=0`.

Maintained packages and examples use `encoding/json/v2` and `encoding/json/jsontext` directly. Go 1.27 provides these APIs without an experiment flag. JSON object member names must be unique, UTF-8 and escaped Unicode must be valid, and struct fields match JSON names case-sensitively. Give application structs explicit `json` tags that match their input.

Nil slices and maps encode as `[]` and `{}`. `omitempty` omits empty JSON values; use `omitzero` when a field's Go zero value should be absent. Map member order is unspecified unless an operation explicitly requires ordering. Strings do not escape HTML by default. Byte arrays and slices use Base64; `time.Duration` requires an explicit supported format. Custom encoding can implement `MarshalJSONTo(*jsontext.Encoder) error`; decoding can implement `UnmarshalJSONFrom(*jsontext.Decoder) error`. See the [JSON v2 API](https://pkg.go.dev/encoding/json/v2) for the complete current behavior. `json.RawMessage` and `json.Number` from `encoding/json` remain usable value types; maintained serialization calls use v2 APIs.

These rules describe JSON serialization owned by Powertools. Invocation input/output serialization performed by `aws-lambda-go` follows that SDK's encoding contract.

The latest published cohort is v0.2.0. This guide describes current main,
including the unreleased Go 1.27/JSON v2 upgrade. Use the [README example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/README.md)
for a minimal application pinned to v0.2.0; installing that version does not
install main's new JSON behavior. See [version policy](VERSION_POLICY.md).

For the maintained source example and repository packaging, obtain a checkout:

~~~sh
git clone https://github.com/rambow-cloud/powertools-lambda-go.git
cd powertools-lambda-go
~~~

Use the repository workspace for these development examples. See
[module installation](MODULES.md) for isolated released-version consumers.

## Create utilities once

The example below is included directly from the maintained `examples/basic/main.go` source when the site builds.

~~~go
--8<-- "examples/basic/main.go"
~~~

Create Logger and Tracer before `lambda.Start` so configuration is reused across invocations. `appLog` is the Powertools Logger object; `stdlog` is Go's standard `log` package, used only for fallback error messages. Inside the handler, `requestLog := appLog.WithContext(ctx)` creates the invocation-bound Logger object used for `Info` calls. Put Tracer outside Logger and pass the invocation context into operations. Both wrappers reuse the same Commons invocation identity.

With the default `INFO` level, an event such as `{"name":"Ada"}` writes one structured application record with `message: "Handling request"`, `service: "hello"`, and `name: "Ada"`, plus timestamp, Lambda identity, and active tracing fields. The handler returns `{"message":"Hello, Ada"}` separately; that response is not a log record. See [the Logger examples and JSON output](LOGGER.md#write-your-first-log) for the complete record shape and method usage.

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

The default runtime acceptance environment uses Docker. Follow the maintained [local integration runner](LOCAL_INTEGRATION.md) and [local acceptance scope](LOCAL_VALIDATION.md).

Outside Lambda, tracing is disabled unless you explicitly supply `tracer.WithLocalTracing(true)`. A local collector is still required to export spans. `POWERTOOLS_DEV=true` enables readable local logs and disables tracing.

Continue with [Logger](LOGGER.md), [Tracer](TRACER.md), or [Metrics](METRICS.md).
