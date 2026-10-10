---
title: Go Lambda logging, tracing and metrics
description: "Independent Go utilities for AWS Lambda: structured logging, OpenTelemetry tracing, CloudWatch metrics, validation and event handling."
---

# What is Powertools for AWS Lambda (Go)

Powertools for Go Lambda is a collection of utilities for building native Go functions on AWS Lambda. Add structured logging, OpenTelemetry tracing, metrics, and event handling to your functions with independently versioned Go modules.

This is an **independent community implementation** based on Powertools for AWS Lambda (TypeScript) v2.35.0. It is a development subset with documented [compatibility boundaries](COMPATIBILITY.md). See [Release notes](RELEASE_NOTES.md) for published versions, module updates and version-only bumps.

## Getting started

Follow the [first Lambda guide](GETTING_STARTED.md) to compose Logger and Tracer around a typed handler, configure a collector, and build an executable for `provided.al2023`.

Use **Go 1.27 or newer**, target **arm64 or x86_64**, and keep **CGO disabled**. Install the published modules using the [module guide](MODULES.md), which explains dependencies, imports, and independent versioning.

Before choosing a utility, read [usage patterns](USAGE_PATTERNS.md) for object and wrapper lifetimes, and [environment variables](ENVIRONMENT_VARIABLES.md) for configuration. Every main utility guide shows a complete example, expected output and its TypeScript mapping.

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

The [quickstart](GETTING_STARTED.md#create-utilities-once) includes a complete Go handler directly from the maintained source. For other use cases, browse the [examples directory](https://github.com/rambow-cloud/powertools-lambda-go/tree/main/examples) or start with one of these guides:

- [HTTP routing and middleware](HTTP.md), including [response streaming](HTTP_STREAMING.md).
- [Batch processing](BATCH.md) with [typed event parsing](PARSER.md).
- [AppSync Events](APPSYNC_EVENTS.md), [AppSync GraphQL](APPSYNC_GRAPHQL.md), and [Bedrock Agents](BEDROCK.md).

## Project status

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
