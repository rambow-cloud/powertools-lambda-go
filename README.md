# Powertools for Go Lambda

Structured logging, OpenTelemetry tracing, metrics and other utilities for native
Go AWS Lambda applications. This independent community project uses Powertools
for AWS Lambda (TypeScript) v2.35.0 as its behavioral reference. See the
[supported scope](docs/COMPATIBILITY.md) for differences and limitations.

## Install

Use Go 1.27 or newer for the current source. Lambda binaries target
`provided.al2023`, with Linux `amd64` (`x86_64`) or `arm64`. Keep CGO disabled.

The latest published version is [v0.2.0](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v0.2.0).
This README follows `main`; the installation and minimal example below use that
published version. See [version policy](docs/VERSION_POLICY.md) for released APIs
versus development changes.

From a new application directory in PowerShell:

```powershell
$env:CGO_ENABLED = '0'
go mod init example.com/hello
go get github.com/rambow-cloud/powertools-lambda-go/logger@v0.2.0 github.com/aws/aws-lambda-go@v1.55.0
```

Each utility has its own module. Install the modules you use; see
[module installation](docs/MODULES.md).

## Your first Lambda

Save this complete example as `main.go`:

```go
package main

import (
    "context"
    "fmt"

    "github.com/aws/aws-lambda-go/lambda"
    "github.com/rambow-cloud/powertools-lambda-go/logger"
)

type Event struct {
    Name string `json:"name"`
}

type Response struct {
    Message string `json:"message"`
}

func main() {
    appLog := logger.New(logger.WithServiceName("hello"))
    handler := func(ctx context.Context, event Event) (Response, error) {
        if event.Name == "" {
            return Response{}, fmt.Errorf("name is required")
        }
        requestLog := appLog.WithContext(ctx)
        if err := requestLog.Info("Handling request", logger.Fields{"name": event.Name}); err != nil {
            return Response{}, fmt.Errorf("write request log: %w", err)
        }
        return Response{Message: "Hello, " + event.Name}, nil
    }
    lambda.Start(logger.WrapHandler(appLog, handler))
}
```

An input of `{"name":"Ada"}` returns `{"message":"Hello, Ada"}` and writes
a structured log with `message: "Handling request"`, `service: "hello"` and
`name: "Ada"`. An empty name returns a handler error. Create the Logger once;
use `WithContext(ctx)` inside its wrapper to isolate invocation fields.

Build for the architecture selected on your Lambda function:

```powershell
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'arm64' # Use 'amd64' for Lambda x86_64.
go build -trimpath -tags lambda.norpc -o bootstrap .
```

Package `bootstrap` with executable permissions and configure `provided.al2023`.
The [quickstart](docs/GETTING_STARTED.md) shows repository packaging and
[OpenTelemetry composition](examples/basic/main.go).

## Documentation

Read the [documentation site](https://powertools-lambda-go.rambow.cloud/) or
[Go package documentation](https://pkg.go.dev/github.com/rambow-cloud/powertools-lambda-go/logger@v0.2.0).

| Area | Guides |
| --- | --- |
| Observability | [Logger](docs/LOGGER.md), [Tracer](docs/TRACER.md), [Metrics](docs/METRICS.md) |
| Configuration and persistence | [Parameters](docs/PARAMETERS.md), [Idempotency](docs/IDEMPOTENCY.md), [Redis/Valkey](docs/IDEMPOTENCY_CACHE.md) |
| Event processing | [Batch](docs/BATCH.md), [Parser](docs/PARSER.md), [Validation](docs/JSON_SCHEMA_VALIDATION.md), [Kafka](docs/KAFKA.md) |
| Event handlers | [HTTP](docs/HTTP.md), [AppSync Events](docs/APPSYNC_EVENTS.md), [GraphQL](docs/APPSYNC_GRAPHQL.md), [Bedrock](docs/BEDROCK.md) |
| Shared tools | [Masking](docs/DATAMASKING.md), [Signer](docs/SIGNER.md), [JMESPath](docs/JMESPATH.md), [Commons](docs/COMMONS.md), [Metadata](docs/METADATA.md) |

Tracing uses OpenTelemetry, including X-Ray delivery through a collector's
`awsxray` exporter. The legacy `tracer/xray` adapter is deprecated and frozen;
retain its [migration warning](docs/XRAY_MIGRATION.md).

See [Releases](https://github.com/rambow-cloud/powertools-lambda-go/releases),
[v1 readiness](docs/V1_READINESS.md) and the [feature comparison](docs/FEATURE_PARITY.md).

## Contributing

Follow the [issue → branch → PR → review workflow](CONTRIBUTING.md).
See [module development](docs/MODULES.md) and [local integration](docs/LOCAL_INTEGRATION.md)
for checks. Root `go test ./...` covers only the root module; use
`tools/modules.py check` with `CGO_ENABLED=0` to verify nested modules and consumers.

## License

Original contributions use the [MIT License](LICENSE). Preserved attribution is
listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and each module's NOTICE.
This is an independent project, not an official AWS distribution.
