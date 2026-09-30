# Tracing backend decision

Decision date: 2026-09-12. Status: accepted.

Scope update (2026-09-13): legacy X-Ray compatibility is limited to the existing adapter. Retain the existing optional adapter and its regression coverage. Do not pursue exhaustive native document parity, SDK-specific enrichment (including the recorded DynamoDB table-name gap), or new legacy features. Prioritize Logger and OpenTelemetry tracing. Existing AWS validation remains historical evidence, and local Docker is now the default integration environment.

Scope update (2026-09-14): legacy SDK use requires explicit deprecation notices. The adapter is now deprecated and frozen, with module/API notices and a once-per-process constructor warning. All new tracing and the current Lambda fixture use OpenTelemetry, including X-Ray delivery through a collector. The legacy fixture option now fails with migration guidance. See [XRAY_MIGRATION.md](XRAY_MIGRATION.md).

## Verified direction

AWS lists the X-Ray SDKs and daemon as in maintenance mode from **February 25, 2026**, with security fixes only and **N/A** as the end date in its current table. This is not a statement that the X-Ray service is being retired. AWS recommends migration to OpenTelemetry for application instrumentation and delivery to X-Ray. Earlier claims of a fixed February 2027 SDK end date should not replace the current published timeline. Source: [AWS support timeline](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-daemon-timeline.html).

OpenTelemetry Go identifies traces and metrics as stable. The ecosystem provides HTTP instrumentation, AWS SDK v2 middleware, and AWS X-Ray propagation and ID generation independently of the legacy SDK. Sources: [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/), [contrib Go repository](https://github.com/open-telemetry/opentelemetry-go-contrib).

AWS documents Go Lambda tracing through OpenTelemetry/ADOT, including Lambda invocation instrumentation. Source: [ADOT Go Lambda guidance](https://aws-otel.github.io/docs/getting-started/lambda/lambda-go/).

The Powertools project has an OpenTelemetry Tracer RFC discussing a familiar facade, custom providers, and multiple integration approaches. The RFC explicitly disclaims an implementation commitment. It supports the architectural direction but is not evidence that TypeScript v2.35.0 already implements it or that a release date is promised. Source: [Powertools RFC #90](https://github.com/aws-powertools/powertools-lambda/discussions/90).

## Implementation decision and consequences

The default Go backend uses OTel SDK tracing, OTLP/HTTP, a private provider, a parent-based sampler, and the OTel X-Ray ID generator/propagator. Applications can inject a provider without this library taking ownership of shutdown. HTTP/SDK instrumentation is explicit. Lambda wrappers close spans and attempt a bounded flush before returning the original handler result or rethrowing the original panic.

The deprecated, frozen `tracer/xray` module retains AWS X-Ray SDK for Go v2.0.3 for existing applications. Applications that import only the OTel `tracer` module and the current Lambda integration fixture do not inherit the legacy SDK dependency. The adapter has its own dependency file and deprecation metadata; see [MODULES.md](MODULES.md).

The common facade preserves operation names, capture controls, and high-level annotation/metadata intent. It cannot make OTel attributes identical to native X-Ray metadata and exceptions. These differences are recorded in [COMPATIBILITY.md](COMPATIBILITY.md). Maintained OTel delivery requires additional cloud acceptance before claiming exhaustive service-map or raw-document parity; legacy SDK results remain historical evidence.

Inference: OTel is the stronger foundation for new work because AWS recommends it and its Go instrumentation ecosystem supplies the required primitives. Keeping the SDK adapter limits migration friction for applications that still require native X-Ray subsegments. The adapter should remain optional and should not drive new core APIs.
