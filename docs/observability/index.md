---
title: "Go Lambda observability with Powertools"
description: "Add structured logging, OpenTelemetry tracing and CloudWatch metrics to Go AWS Lambda functions with independent Powertools modules."
category: true
---

# Observability

Understand what your Go Lambda function did, how long it took and where a request failed. Logger, Tracer and Metrics are independent modules that can be adopted together or separately.

## Choose a utility

| Need | Utility | Start here |
| --- | --- | --- |
| Search structured application logs | Logger | [Write JSON logs with Lambda context](../LOGGER.md) |
| Follow a request through downstream operations | Tracer | [Instrument handlers and calls with OpenTelemetry](../TRACER.md) |
| Monitor application counts, durations and failures | Metrics | [Emit CloudWatch Embedded Metric Format](../METRICS.md) |

Start with the complete example in each guide. Keep reusable utility instances outside the handler, and bind invocation-specific context inside it; [usage patterns](../USAGE_PATTERNS.md) explains their lifetimes.

## HTTP observability and AWS X-Ray

For routed APIs, use [HTTP observability middleware](../HTTP_OBSERVABILITY.md) to compose logging, request metrics and route spans. Export maintained tracing through OpenTelemetry and a collector's `awsxray` exporter. The legacy X-Ray SDK adapter is deprecated and frozen; follow the [X-Ray migration guide](../XRAY_MIGRATION.md).

See [environment variables](../ENVIRONMENT_VARIABLES.md) for configuration and [Getting started](../getting-started/index.md) for installation.
