# Local Docker integration runner

Latest recorded runtime acceptance (2026-10-04, Logger TextMarshaler follow-up): 904/904 Lambda RIE assertions, 95/95 streaming checks and 14/14 saved Batch checks, including six new assertions for empty-string TextMarshaler values/errors. All 31 packaged modules and 28 public consumers passed one complete check; the same flow rebuilt normal and streaming handlers for Linux amd64/arm64 with CGO disabled. Docker executed amd64. Docker Desktop was restarted before validation after an engine HTTP 500; a private temporary container kept the engine active during the complete run and was removed afterward. Runtime metadata records that module checks and builds both executed. See [packaged evidence](MODULE_ACCEPTANCE_LOGGER.json), [acceptance history](LOCAL_VALIDATION.md) and [runtime evidence](LOCAL_ACCEPTANCE.json). No AWS resources were used; exhaustive parity and browser review remain separate gates.

## GitHub Actions

First hosted runtime acceptance (2026-10-05): PR #32's native
[Go CI run](https://github.com/rambow-cloud/powertools-lambda-go/actions/runs/37233122495)
passed all 31 packaged modules, 28 public consumers, 904 RIE/Valkey assertions,
95 streaming checks, 14 Batch checks, and 108 KMS reverse-interop checks.
The runtime rebuilt both architectures, executed amd64, preserved checked-in
reports, and cleaned up without errors. The [acceptance summary](CI_RUNTIME_ACCEPTANCE.json)
records the tested PR merge SHA, run/attempt, suite counts, and six binary hashes;
it is distinct from the historical local acceptance above.

CI runs **Runtime simulation** for Go/runtime inputs, release batches, unknown
paths and full manual runs, after **Modules and Lambda artifacts** passes.
Documentation and metadata-only changes skip both jobs; the always-running
**CI gate** validates this selection. It installs Node.js
22.21.1 and reference dependencies with `npm ci`, then uses
`integration/local/run.py --skip-module-checks`: module tests are reused from
the prerequisite job while normal and streaming fixtures are freshly built for
Linux amd64 and arm64. Docker executes amd64 Lambda RIE, streaming, and the live
TypeScript/Go Valkey bridge. Batch checks consume that same run's logs/spans;
the separate KMS executable emits Go ciphertext for the pinned TypeScript
provider's reverse interoperability checks. All endpoints and credentials are
synthetic; no AWS secrets or cloud resources are required.

CI sets `POWERTOOLS_ACCEPTANCE_DIR` to the ignored `dist/runtime-acceptance`
directory so Streaming, Batch, and KMS reports preserve the checked-in local
acceptance history. Each report identifies its commit, workflow run, and attempt.
`integration/local/ci_report.py` rejects missing/empty/failed suites, stale
identity, reused builds, missing binaries, Batch artifact hash mismatches, and
cleanup errors. It writes a job summary and SHA-256 hashes for all six binaries.
The `runtime-simulation-SHA-RUN-ATTEMPT` artifact retains reports and diagnostic
logs for seven days, including on failure; build caches and binaries are excluded.

Merge protection requires **CI gate**, which requires runtime success whenever
the Go/runtime suite is selected. Release publication additionally requires
actual **Runtime simulation** success even when the aggregate gate passes.
See [maintainer setup](MAINTAINING.md) for rollout. Checked-in Go reference
corpora run in the module job; the full Node fixture regeneration command remains
a deliberate development operation when reference versions or generators change.
Runtime simulation does not establish arm64 execution or live AWS acceptance.

## Local execution

Run from the repository root with local Go, Python 3.10+ (or `uv`), Node.js with the pinned reference dependencies, and Docker Desktop using Linux containers:

```powershell
npm.cmd ci --prefix tools/reference
uv run python integration/local/run.py
```

The runner verifies all independently packaged modules with `GOWORK=off`, including tests, vet, dependency metadata, and standalone public-module consumers. It then cross-compiles the fixture for Linux amd64 and arm64 using the workspace and runs the amd64 binary in the digest-pinned AWS Lambda `provided.al2023` image with its Runtime Interface Emulator (RIE). Every Go command sets `CGO_ENABLED=0`. Package compilation is sequential with `GOMAXPROCS=2`, a 128 MiB soft Go heap limit, and `GOGC=20` to reduce memory pressure; the soft limit is not a hard process-memory cap. Build cache and temporary files stay under the repository's ignored `dist/` directory.

Three disposable containers share an internal Docker network: Lambda, the capture service, and a digest-pinned Valkey server. No host ports are published; the image's existing HTTP client, invoked through `docker exec`, drives the integration APIs inside the network. The TypeScript bridge uses `docker exec valkey-cli` to create and read reference records. Valkey stores data only in tmpfs with persistence disabled. No AWS configuration, profiles, or real credentials are mounted or passed to the containers. The Lambda fixture uses synthetic credentials and fixed internal endpoints when `LOCAL_TEST=true`.

The capture service decodes real OTLP/HTTP protobuf exports and provides deterministic HTTP, DynamoDB, SSM, Secrets Manager, AppConfig Data, and AppConfig Agent response fixtures. It is not a production collector, DynamoDB Local, or an AWS service emulator. Tests exercise actual SDK serialization/middleware and HTTP instrumentation; they do not verify live service semantics, IAM, KMS, or S3 signature enforcement.

Five invocations cover initial/warm success, returned error, an unsampled parent, and panic. Assertions inspect runtime request IDs, log isolation, buffering, trace identity, span relationships, annotations, error status, downstream propagation, and completed OTLP export. The fixture explicitly injects deterministic parent headers through the test handler's context extractor: these checks do not establish native AWS trace-header injection behavior. RIE does not reproduce Lambda freeze/thaw, hard timeouts, X-Ray indexing, or service maps. Arm64 is cross-compiled but is not executed by this runner.

Evidence is written to `dist/local/report.json`, `dist/local/otlp.json`, and `dist/local/lambda.log`. The runner removes only its own named containers and network in a `finally` block. The downloaded image and build artifacts remain reusable. Interrupted processes or Docker failures can leave resources with the run's `ptgo-local-` prefix; check the report for cleanup failures.

Metrics acceptance checks one separate cold-start document and five invocation-specific EMF documents, including error/panic cleanup. Parameters acceptance checks all five providers, warm cache reuse, forced refresh on the unsampled invocation, AppConfig token rotation/unchanged values, and agent-owned caching. Metadata checks authenticated HTTP, response fields, warm cache reuse, and explicit clearing. SDK requests must contain one Powertools marker. The historical 292-assertion suite covered OTel trace/span log correlation, JMESPath decoded projections and Logger correlation, synthetic SigV4 verification, Batch source/FIFO responses, DynamoDB-backed Idempotency, and real Valkey replay/validation/recovery. Parser checks typed stream/notification payloads, DynamoDB images and large integers, five HTTP body envelopes, ALB multi-value headers, combined HTTP metadata/body errors, safe aggregation versus first-invalid-record failures, and invalid-order rejection before the Idempotency business handler. Service checks add ordered Kafka payloads and safe paths, CloudFormation updates, Transfer IPv4, empty Connect profiles, SES receipts, S3 key/size preservation and Object Lambda transformations. Identity checks cover typed AppSync arguments and batches, publish/subscribe isolation, native Cognito responses, input rejection, token scopes and empty challenge sessions. Error checks cover recursive union trees, sole-branch diagnostics, continued refinements, array issue order and nested JSON/nullable/SQS aggregation. The TypeScript adapter and Go Lambda exchange native JSON records in both directions. The DynamoDB Idempotency fixture is separate from ordinary parameter reads and verifies native SDK requests, shared marker precedence, and record-level Logger/Tracer composition. Warm success and returned-error paths use pure W3C context without a runtime X-Ray header, exercising OTel-only buffering and error flush. Live IAM acceptance, CloudWatch ingestion, service-side retries/checkpoints, cache cluster/failover/expiry timing, Parameters services, and LMDS behavior are outside this local test.

After a successful run, `uv run python integration/local/batch_report.py` checks 14 additional Batch log/span properties using saved artifacts without invoking Lambda again. `uv run python integration/local/report.py` preserves the run results and hashes in `docs/LOCAL_ACCEPTANCE.json`.

Use `--runtime-only` to reuse previously built binaries while debugging Docker infrastructure. It skips Go validation and compilation and must not be used to validate source changes until the affected binaries have been rebuilt. The emulator can report a panic as HTTP 502 with `Runtime.ExitError`; the suite separately checks the original panic in runtime logs and the completed error spans.

Use `--skip-module-checks` when module checks have already passed and only a build or Docker infrastructure issue needs another attempt. Both architectures are rebuilt; the report explicitly records that module verification was reused.

The image digest is pinned in `run.py`. Refresh it deliberately when updating the Lambda runtime baseline. Official references: [Go Lambda container images](https://docs.aws.amazon.com/lambda/latest/dg/go-image.html), [Runtime Interface Emulator](https://github.com/aws/aws-lambda-runtime-interface-emulator).
