# Working Agreements

- CGO Disabled always: set `CGO_ENABLED=0` for every Go build, test, vet, local development command, and CI job.
- Do not enable CGO to run the Go race detector. Do not use `go test -race`, which requires CGO. Use concurrency tests with CGO disabled.
- Target AWS Lambda on `provided.al2023`, with Linux `amd64` and `arm64` binaries.
- Use local Docker integration tests by default. Cloud testing requires an explicit request and explicit AWS_PROFILE and POWERTOOLS_TEST_ACCOUNT configuration; use ap-east-1 (Hong Kong).
- Use OpenTelemetry for tracing, including delivery to AWS X-Ray through a collector's awsxray exporter. The tracer/xray SDK adapter is deprecated and frozen; retain its explicit migration warning and existing regression coverage, but do not add features or use it in new examples, integration fixtures, or release recommendations.
- Write all documentation and code comments in English.
- Keep packages cohesive and loosely coupled. Reuse established primitives, keep interfaces small, and avoid speculative abstractions or duplicate wrappers.
- Maintain independent Go modules listed in `tools/modules.json` and `go.work`. Keep root Commons free of external dependencies and preserve the single shared invocation identity. Do not add local `replace` directives to module files.
- Use `uv run python tools/modules.py tidy` to maintain unpublished internal dependencies and `uv run python tools/modules.py check` to verify packaged modules with `GOWORK=off`. Root `go test ./...` does not cover nested modules. All Go subprocesses must keep CGO disabled.
- Track implemented and verified work in `docs/CHECKLIST.md`. Check an item only after its acceptance criteria pass. Do not update the checklist for documentation-only fixes.
- Avoid redundant validation and repeated retries. Repeat a check only after a relevant change or a newly understood failure.
- When a result can be verified in a browser, provide the exact page and verification steps to the user instead of issuing command-line HTTP probes.
- For DNS validation, use CloudFormation status and outputs as the source of truth. Do not perform local DNS validation.
- Keep credentials, private account/profile identifiers, raw cloud evidence, and publication backups out of Git and module archives. Use ignored local configuration for personal environment details; preserve sanitized public acceptance summaries.
