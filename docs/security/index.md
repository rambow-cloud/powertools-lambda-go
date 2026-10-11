---
title: "Go Lambda security and configuration utilities"
description: "Retrieve cached application parameters, mask sensitive fields and sign AWS HTTP requests with Go Powertools Lambda utilities."
category: true
---

# Security and configuration

Load application settings, control which data leaves your handler, and authenticate outgoing AWS requests. These utilities support specific application tasks; your application still owns its access policies and data-handling decisions.

## Choose a utility

| Task | Guide |
| --- | --- |
| Retrieve and cache values from SSM, Secrets Manager, AppConfig or DynamoDB | [Parameters](../PARAMETERS.md) |
| Erase or transform selected sensitive fields | [Data Masking](../DATAMASKING.md) |
| Use the optional AWS Encryption SDK masking provider | [KMS provider](../DATAMASKING_KMS.md) |
| Sign outgoing HTTP requests with AWS Signature Version 4 | [Signer](../SIGNER.md) |

Start with the Parameters example for reusable configuration retrieval. Review cache lifetime and transformation behavior, and keep credentials out of documentation, repository files and application logs.

Apply masking at the point where your application exposes or persists data. Read the selector and provider boundaries before composing it with [structured logging](../LOGGER.md). The optional KMS provider has its own dependency and supported-environment requirements; its guide records the current caching limitations.

Use Signer when you own an HTTP request that needs SigV4 authentication. If you already use an AWS SDK client, follow that SDK's signing and credential configuration rather than adding another signing layer.

For installation, see [Getting started](../getting-started/index.md). For invocation lifetimes and configuration names, see [Usage patterns](../USAGE_PATTERNS.md) and [Environment variables](../ENVIRONMENT_VARIABLES.md).
