---
title: Go Lambda logging, tracing and metrics
description: "Independent Go utilities for AWS Lambda: structured logging, OpenTelemetry tracing, CloudWatch metrics, validation and event handling."
hide:
  - toc
---

<div class="pt-hero" markdown="1">
<div markdown="1">

# Powertools for AWS Lambda (Go)

**Build Go Lambda functions with less boilerplate.**

Structured logging, OpenTelemetry tracing, CloudWatch metrics and HTTP routing. Choose the independent Go modules your application needs.

<div class="pt-actions" markdown="1">

[Get started](GETTING_STARTED.md){ .pt-button .pt-button-primary }
[Build an HTTP API](HTTP.md){ .pt-button }
[GitHub](https://github.com/rambow-cloud/powertools-lambda-go){ .pt-button }

</div>
</div>
<div class="pt-mascot"><img src="assets/logo-ram-gopher.png" alt="Powertools for Go ram-gopher mascot" width="240" height="240"></div>
</div>

An **independent community implementation**, based on Powertools for AWS Lambda (TypeScript) v2.35.0. See the [compatibility boundaries](COMPATIBILITY.md) and [release notes](RELEASE_NOTES.md). This is not an official AWS distribution.

## Getting started

Install one utility to begin. For example, add structured logging:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.1.0
```

Use **Go 1.27 or newer**, target **arm64 or x86_64**, and keep **CGO disabled**. Install the published modules using the [module guide](MODULES.md), which explains dependencies, imports, and independent versioning.

<div class="pt-cards" markdown="1">
<div class="pt-card" markdown="1">

### Your first Lambda

Compose a typed handler with logging and tracing, then build a static Lambda executable.

[Follow the quickstart](GETTING_STARTED.md)

</div>
<div class="pt-card" markdown="1">

### Route HTTP requests

Start with `app.Get` and `app.Post`. Read path parameters and JSON bodies, and return JSON responses.

[Build a GET/POST handler](HTTP.md#complete-example)

</div>
<div class="pt-card" markdown="1">

### Install only what you use

Pick independent modules and add middleware or AWS integrations when your application needs them.

[Choose your modules](MODULES.md)

</div>
</div>

## Start with the essentials

<div class="pt-cards" markdown="1">
<div class="pt-card" markdown="1">

### Logger

Structured JSON logs with Lambda context, correlation IDs and configurable levels.

[Write your first log](LOGGER.md#write-your-first-log)

</div>
<div class="pt-card" markdown="1">

### Tracer

Trace handlers and downstream calls with OpenTelemetry. Send spans to AWS X-Ray through a collector.

[Add tracing](TRACER.md)

</div>
<div class="pt-card" markdown="1">

### Metrics

Emit custom CloudWatch metrics with Embedded Metric Format, including cold-start metrics.

[Record application metrics](METRICS.md)

</div>
</div>

## Features

Choose the utilities your application needs. Each feature has its own Go module; adding one utility does not install the entire toolkit.

| Utility | What you can do |
| --- | --- |
| [Tracer](TRACER.md) | Trace handlers, operations, HTTP requests, and AWS SDK calls using OpenTelemetry. Export to AWS X-Ray through a collector. |
| [Logger](LOGGER.md) | Write structured JSON logs with Lambda context, correlation IDs, configurable levels, and optional buffering. |
| [Metrics](METRICS.md) | Emit application metrics using CloudWatch Embedded Metric Format. |
| [Event handlers](HTTP.md) | Route HTTP requests, stream responses, and handle AppSync Events, AppSync GraphQL, and Bedrock Agent functions. |
| [Parameters](PARAMETERS.md) | Retrieve and cache configuration from SSM, Secrets Manager, DynamoDB, and AppConfig. |
| [Idempotency](IDEMPOTENCY.md) | Protect operations from duplicate execution with DynamoDB or optional Redis/Valkey persistence. |
| [Batch processing](BATCH.md) | Process SQS, Kinesis, and DynamoDB Streams batches with partial failure handling. |
| [JMESPath](JMESPATH.md) | Query JSON documents and unwrap supported event envelopes. |
| [Parser](PARSER.md) | Parse event payloads into typed values and compose validation with your handler. |
| [Validation](JSON_SCHEMA_VALIDATION.md) | Validate inputs and responses against JSON Schema. |
| [Data Masking](DATAMASKING.md) | Erase or transform selected fields, with optional regex and KMS providers. |
| [Kafka](KAFKA.md) | Decode Kafka events with JSON, Avro, or Protobuf payloads. |
| [Signer](SIGNER.md) | Sign HTTP requests with AWS Signature Version 4. |
| [Commons](COMMONS.md) and [Metadata](METADATA.md) | Reuse runtime primitives and retrieve Lambda execution-environment metadata. |

The manifest contains 31 modules, including 28 public modules. The legacy X-Ray SDK adapter is included in that count but is deprecated and frozen. Follow the [OpenTelemetry migration guide](XRAY_MIGRATION.md) for maintained X-Ray support.

## Examples

Every main utility guide starts with maintained code and expected results. Browse the [examples directory](https://github.com/rambow-cloud/powertools-lambda-go/tree/main/examples) or choose a use case:

- [Simple GET/POST routing](HTTP.md#complete-example), then [middleware](HTTP_MIDDLEWARE.md), [observability](HTTP_OBSERVABILITY.md) or [response streaming](HTTP_STREAMING.md).
- [Batch processing](BATCH.md) with [typed event parsing](PARSER.md).
- [AppSync Events](APPSYNC_EVENTS.md), [AppSync GraphQL](APPSYNC_GRAPHQL.md), and [Bedrock Agents](BEDROCK.md).

## Project status

Read [usage patterns](USAGE_PATTERNS.md) for object lifetimes and [environment variables](ENVIRONMENT_VARIABLES.md) for configuration. Utilities can be adopted independently, and maintained v1 APIs follow the [version policy](VERSION_POLICY.md).

Use the [TypeScript feature comparison](FEATURE_PARITY.md), [project progress](CHECKLIST.md) and [roadmap](ROADMAP.md) to distinguish implemented behavior from remaining compatibility and release work. [Local acceptance records](LOCAL_VALIDATION.md) describe the scope of previous verification; they do not establish exhaustive parity or cloud acceptance.

Original contributions are licensed under [MIT](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/LICENSE). Third-party content retains its [original attribution](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/THIRD_PARTY_NOTICES.md). This project is not an official AWS distribution.

## Common questions

### Is Powertools for Go an official AWS project?

No. This is an independent community implementation maintained in the rambow-cloud repository. Its feature baseline is Powertools for AWS Lambda (TypeScript) v2.35.0; the [compatibility guide](COMPATIBILITY.md) describes the maintained subset and differences.

### Do I need to install every utility?

No. Install only the Go modules your application uses. Logger, Metrics, Tracer and the other utilities have independent import paths. The [installation guide](MODULES.md) explains the shared release version and module dependencies.

### How do I send Go Lambda traces to AWS X-Ray?

Use the maintained [OpenTelemetry Tracer](TRACER.md) and send spans through an OTLP collector with an `awsxray` exporter. The legacy `tracer/xray` SDK adapter is deprecated and frozen; follow the [migration guide](XRAY_MIGRATION.md).

### Which Lambda runtime and architectures are supported?

Build static Go binaries with `CGO_ENABLED=0` for Linux amd64 or arm64 and deploy them to AWS Lambda's `provided.al2023` runtime. The [first Lambda guide](GETTING_STARTED.md) includes the handler and build commands.
