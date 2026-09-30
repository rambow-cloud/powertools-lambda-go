# Deprecated X-Ray SDK adapter

This module is deprecated and frozen. Use `github.com/rambow-cloud/powertools-lambda-go/tracer` with an OpenTelemetry Collector's `awsxray` exporter to continue sending traces to AWS X-Ray.

The legacy constructor emits a migration warning once per process. Existing source and regression tests remain for compatibility, but new applications, examples, and integration fixtures must use OpenTelemetry. No new SDK-specific features or compatibility fixes are planned.

AWS moved X-Ray SDKs and the daemon to maintenance mode on February 25, 2026, with security updates only. AWS X-Ray itself is not retired. [AWS support timeline](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-daemon-timeline.html), [migration guide](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-migration.html).
