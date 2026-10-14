---
title: "Go Powertools Lambda installation and quickstart guides"
description: "Install Go Powertools modules, build your first AWS Lambda handler and understand supported runtimes, configuration and compatibility."
category: true
---

# Getting started

Use Go Powertools Lambda to add common production utilities to your Go AWS Lambda functions. This is an independent community project; install only the modules your application needs.

## Choose your first step

| Goal | Guide |
| --- | --- |
| Build and run a complete handler | [Your first Lambda](../GETTING_STARTED.md) |
| Choose imports and install a published version | [Modules and installation](../MODULES.md) |
| Reuse utilities safely across invocations | [Usage patterns](../USAGE_PATTERNS.md) |
| Configure service names, logging and metrics | [Environment variables](../ENVIRONMENT_VARIABLES.md) |
| Check runtime and API boundaries | [Compatibility](../COMPATIBILITY.md) |

Start with Logger for structured logs, then add Tracer or Metrics when you need them. The quickstart includes maintained code and explains how to build a static executable for Lambda's `provided.al2023` runtime with CGO disabled.

## Next steps

- [Observability](../observability/index.md): logging, tracing and application metrics.
- [Event handlers](../event-handlers/index.md): HTTP APIs, AppSync and Bedrock functions.
- [Release notes](../RELEASE_NOTES.md): features and fixes in published versions.
