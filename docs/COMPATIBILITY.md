# Compatibility

This project is an independent community **Go port of Powertools for AWS Lambda (TypeScript)**. It is not an official AWS distribution. The Go APIs use explicit contexts, functions and typed values rather than decorators or JavaScript runtime objects.

## Supported applications

- Go 1.27 or newer, with `CGO_ENABLED=0`.
- Native Lambda binaries for `provided.al2023` on Linux amd64 or arm64.
- Independent modules: install only the utilities your application needs.
- A unified release version for maintained modules; see [module installation](MODULES.md).

## Differences that affect application code

| Area | Go behavior |
| --- | --- |
| JSON | `encoding/json/v2`; use explicit JSON tags. Duplicate members and invalid Unicode are rejected. |
| Invocation state | Wrappers and `context.Context` carry per-request state. Create root utilities once and bind them inside the handler. |
| Tracing | OpenTelemetry, with optional X-Ray delivery through a collector. The legacy X-Ray SDK adapter is deprecated and frozen. |
| Errors | Go errors preserve their types and causes; JavaScript stacks and exact runtime wording are not an API promise. |
| Event handlers | Configure the matching AWS integration; local routing does not deploy an API or event source. |

Utility guides call out additional behavior that matters when using them. See the [X-Ray migration guide](XRAY_MIGRATION.md) before changing an existing tracing integration.

## Stable APIs

Maintained v1 modules preserve documented public APIs, configuration defaults, persistence formats and lifecycle contracts. Optional additive capabilities can arrive in minor releases. Read [release notes](RELEASE_NOTES.md) for shipped changes and [the version policy](VERSION_POLICY.md) when comparing the repository with a published module.

## For contributors

[Source provenance and detailed API mappings](reference/COMPATIBILITY.md) · [Feature comparison](FEATURE_PARITY.md) · [Project progress](CHECKLIST.md)
