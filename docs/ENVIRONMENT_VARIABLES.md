---
description: "Configure Powertools for Go logging, metrics and tracing using documented environment variables, defaults and option precedence."
---

# Environment variables

These variables configure the implemented Go utilities. The behavioral baseline is [TypeScript v2.35.0](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/environment-variables.md); Go-specific OTel settings are explicit extensions. Configure variables before constructing utilities. Do not mutate process environment as request-local state in concurrent handlers.

## Logging, tracing and metrics

| Variable | Purpose and precedence | Details |
| --- | --- | --- |
| `POWERTOOLS_SERVICE_NAME` | Shared service fallback; utility-specific explicit options take precedence | [Logger](LOGGER.md), [Tracer](TRACER.md), [Metrics](METRICS.md) |
| `POWERTOOLS_DEV` | Readable Logger output; disables default tracing and Metrics emission | A valid explicit metrics-disabled setting can override the Metrics development fallback |
| `POWERTOOLS_LOG_LEVEL` / `LOG_LEVEL` | Configured Logger threshold fallback; default INFO | ALC has precedence; [Logger configuration](LOGGER.md#configuration) |
| `AWS_LAMBDA_LOG_LEVEL` | Lambda advanced logging threshold; also affects default router debug diagnostics | Configure through Lambda logging settings |
| `POWERTOOLS_LOGGER_SAMPLE_RATE` | Debug sampling probability; default zero | Construction/first/warm invocation decisions are distinct |
| `POWERTOOLS_LOGGER_LOG_EVENT` | Opt-in event logging; default disabled | Emits input data, separately from business messages |
| `TZ` | Logger timestamp timezone | UTC default; timezone validation is utility-specific |
| `POWERTOOLS_TRACE_ENABLED` | Disable tracing with false; local enablement remains explicit | [Tracer configuration](TRACER.md#configuration) |
| `POWERTOOLS_TRACER_CAPTURE_RESPONSE` / `POWERTOOLS_TRACER_CAPTURE_ERROR` | False vetoes automatic capture | Explicit application decisions; response defaults differ from the quickstart's opt-out |
| `POWERTOOLS_TRACER_CAPTURE_HTTPS_REQUESTS` | HTTP tracing capture switch | Instrument a client explicitly |
| `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | OTLP collector base or traces URL | Does not install/start a collector |
| `OTEL_TRACES_SAMPLER` / `OTEL_TRACES_SAMPLER_ARG` | Default OTel provider sampler settings | Unset default is parent-based always-on |
| `POWERTOOLS_METRICS_NAMESPACE` | Metrics namespace fallback | Missing namespace emits `default_namespace` with a warning |
| `POWERTOOLS_METRICS_FUNCTION_NAME` | Optional cold-start function dimension | [Name precedence](METRICS.md#manual-cold-start-metrics) |
| `POWERTOOLS_METRICS_DISABLED` | Explicit enable/disable; validated before options | Invalid/present-empty values return a construction error |
| `AWS_LAMBDA_INITIALIZATION_TYPE` | Metrics ColdStart requires on-demand initialization | Runtime-owned; provisioned/unknown differs |

## Configuration, persistence and metadata

| Variable | Purpose | Details |
| --- | --- | --- |
| `POWERTOOLS_PARAMETERS_MAX_AGE` | Ordinary parameter cache TTL seconds; default five | [Cache policy](PARAMETERS.md#cache-and-transformations); named batches have separate reference quirks |
| `POWERTOOLS_PARAMETERS_SSM_DECRYPT` | SSM decryption fallback | Explicit per-call flag, then SDK input, then environment |
| `AWS_APPCONFIG_EXTENSION_HTTP_PORT` | AppConfig Agent port; default 2772 | Agent extension must be installed separately |
| `POWERTOOLS_APPCONFIG_AGENT_RETURN_VALUE` | Agent helper's outside-Lambda development value | [Provider behavior](PARAMETERS.md#other-providers) |
| `POWERTOOLS_IDEMPOTENCY_DISABLED` | Disable Idempotency at construction | [Lifecycle/configuration](IDEMPOTENCY.md#lifecycle) |
| `AWS_LAMBDA_METADATA_API` / `AWS_LAMBDA_METADATA_TOKEN` | Default LMDS endpoint and bearer token | [Metadata](METADATA.md); outside-Lambda default makes no request |

Signer's default credentials provider reads `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and optional `AWS_SESSION_TOKEN` for each operation; `AWS_REGION` is the default region. Inject a provider for another credential source. Other AWS SDK clients use their supplied configuration; this table does not replace SDK configuration or required service permissions.

Lambda function name, memory size, request identity and trace headers enrich telemetry when invocation context is available. Prefer wrapper-provided runtime context. Applications should not forge runtime variables to turn local execution into service acceptance.

## Parsing and scope

Boolean, numeric and whitespace parsing is utility-specific. Commons exposes strict/extended modes; Metrics explicitly validates its disabled variable, while Logger retains its own fallback behavior. Refer to each utility's configuration table rather than assuming an identical parser everywhere.

Construct reusable objects once. Root configuration and caches survive warm invocations, but Logger/Metrics request state belongs to the wrapper-created scope. [Usage patterns](USAGE_PATTERNS.md) explains this lifecycle and how method errors differ from business errors.
