# Powertools for Go Lambda

An independent, early implementation of Logger, Tracer, Metrics, Parameters, Commons foundation, Metadata, Signer, JMESPath, Batch, Idempotency, Parser, JSON Schema Validation, HTTP, AppSync Events/GraphQL Bedrock Agent function handlers, Kafka and initial Data Masking contracts from Powertools for AWS Lambda (TypeScript). The reference is **v2.35.0**, commit `7bcc27b1574493f9452688673658f52b80c53847`. This is a usable development subset, not a complete one-to-one port or an official AWS distribution.

The target is a native Go Lambda executable on `provided.al2023`, for `arm64` and `x86_64`. **CGO is always disabled.** Go 1.26 or newer is required by the selected dependencies. The module path is `github.com/rambow-cloud/powertools-lambda-go`. The local source tree is prepared for the rambow-cloud organization; public repository upload and version tags are pending. See the [publication checklist](docs/PUBLICATION.md) for local preparation and remaining verification.

Data Masking provides default/dynamic/custom field erasure and concurrent application-provider transforms as an independent module. Optional ECMAScript regex replacement is shared with Validation through [commons/regex](docs/REGEX.md). An [optional uncached AWS Encryption SDK provider](docs/DATAMASKING_KMS.md) is now implemented for Linux; full regex and encryption/cache parity remain pending. See [usage and boundaries](docs/DATAMASKING.md) and [the complete plan](docs/DATAMASKING_PLAN.md).

## Documentation

The English documentation site uses Zensical's default modern theme, with searchable utility guides, light/dark themes, and native Go Lambda examples. Start with the [quickstart](docs/GETTING_STARTED.md), [Logger](docs/LOGGER.md), or [OpenTelemetry Tracer](docs/TRACER.md).

The configured GitHub Pages address is https://rambow-cloud.github.io/powertools-lambda-go/; it becomes available after the first source push and successful documentation deployment. See [local preview, CI, and Pages setup](docs/DOCUMENTATION.md).

## Local development

Signer provides standalone SigV4 signing and composable HTTP clients; see [Signer usage and compatibility](docs/SIGNER.md). JMESPath provides standard queries, custom functions, all thirteen envelopes, and optional compiled Logger correlation expressions; see [JMESPath usage and compatibility](docs/JMESPATH.md). Both have independent module dependencies and version targets. Logger alone does not import the query engine.

Batch provides typed record processing and partial failure responses; see [Batch](docs/BATCH.md). Idempotency provides atomic acquisition, stored-response replay, payload validation, and DynamoDB persistence; see [Idempotency](docs/IDEMPOTENCY.md). Redis/Valkey persistence is available through the optional [cache module](docs/IDEMPOTENCY_CACHE.md). Full cross-language and service acceptance remain open.

Parser provides schema composition, typed/manual/safe parsing, JSON/Base64/DynamoDB helpers, all 24 initial event model families and fourteen envelopes. See [Parser usage and boundaries](docs/PARSER.md), [stream contracts](docs/PARSER_STREAMS.md), [HTTP contracts](docs/PARSER_HTTP.md), [service and Kafka contracts](docs/PARSER_SERVICES.md), [AppSync and Cognito contracts](docs/PARSER_IDENTITY.md) and its [remaining implementation checklist](docs/PARSER_PLAN.md). It has no third-party runtime dependencies and composes with Batch and Idempotency through explicit callbacks. Complete type/error/encoding parity remains pending.

Validation is a separate JSON Schema module with reusable compilation, input/output wrappers, custom formats, registered references and JMESPath extraction. See [its API and compatibility boundaries](docs/JSON_SCHEMA_VALIDATION.md), [native Lambda example](examples/validation/main.go) and [remaining gates](docs/VALIDATION_PLAN.md). Parser retains its independent dependency graph.

Local Docker is the default integration environment. Run `uv run python integration/local/run.py` for the Lambda runtime and OTLP acceptance checks; see [local Docker validation](integration/local/README.md) for prerequisites and coverage. OpenTelemetry is the maintained tracing implementation, including delivery to AWS X-Ray. The legacy SDK adapter is deprecated and frozen; see [the migration guide](docs/XRAY_MIGRATION.md).

From this directory in PowerShell:

```powershell
$env:CGO_ENABLED = '0'
uv run python tools/modules.py check
go run ./examples/local
./scripts/build.ps1
```

The local example uses an in-memory OpenTelemetry exporter and prints a correlated JSON log and a span summary. It needs no AWS credentials or collector. The build script produces `dist/amd64/bootstrap`, `dist/arm64/bootstrap`, and ZIP packages with executable permissions. It validates architecture, static linking, and disabled CGO. Set the Lambda architecture to `x86_64` for the amd64 ZIP or `arm64` for the arm64 ZIP.

The repository uses multiple Go modules with a shared `go.work`. Each utility owns its dependency requirements and release version; see [module development and release instructions](docs/MODULES.md). A root `go test ./...` covers only the root module. The module check command verifies every module and standalone consumers with `GOWORK=off`, using local release-shaped archives for unpublished internal dependencies. No public versions have been published.

HTTP event routing is an independent `eventhandler/http` module with REST/v2/ALB/Function URL adapters, routes, request state, middleware, responses and optional schema callbacks. See [the API and compatibility scope](docs/HTTP.md), [native Go Lambda example](examples/http/main.go), and [remaining work](docs/HTTP_PLAN.md). It depends only on Commons and the standard library; existing Logger/Tracer wrappers and Parser/Validation callbacks compose explicitly.

HTTP also provides [CORS/preflight and gzip/deflate middleware](docs/HTTP_MIDDLEWARE.md), with configuration snapshots and route policies. HTTP also supports [ResolveStream/Streamify](docs/HTTP_STREAMING.md) with incremental reader ownership and Logger/OTel lifecycle composition. Current evidence: 2,066 HTTP reference cases across core/Metrics/Tracer modules, 868/868 RIE assertions, and a separate 95/95 real-Go-SDK/local-Runtime-API streaming suite. Cloud streaming and full compatibility remain open.

Kafka now provides an independent lazy consumer for primitive/JSON records, header decoding, typed errors and optional synchronous parsing. See [Kafka usage](docs/KAFKA.md), [the native example](examples/kafka/main.go), and [the remaining binary-format and acceptance gates](docs/KAFKA_PLAN.md). Optional [Avro and Protobuf adapters](docs/KAFKA_BINARY.md) now have independent modules and scoped differential evidence; full binary/native/service parity remains open.

## Lambda usage

Optional [HTTP Metrics and OTel middleware](docs/HTTP_OBSERVABILITY.md) have separate `eventhandler/http/metrics` and `eventhandler/http/tracer` modules. They reuse existing utility scopes, configuration and output, while preserving HTTP core dependency isolation. The HTTP Lambda example demonstrates their composition with CORS and compression.

```go
l := logger.New(logger.WithServiceName("orders"))
t, err := tracer.New(tracer.WithServiceName("orders"))
if err != nil {
    log.Fatal(err)
}

handler := func(ctx context.Context, event Event) (Response, error) {
    requestLog := l.WithContext(ctx)
    requestLog.AppendKeys(logger.Fields{"order_id": event.OrderID})
    if err := requestLog.Info("Processing order"); err != nil {
        log.Printf("Logging failed: %v", err)
    }
    return tracer.Capture(ctx, t, "process", func(ctx context.Context) (Response, error) {
        return process(ctx, event)
    })
}
lambda.Start(tracer.WrapHandler(t, logger.WrapHandler(l, handler)))
```

See [the complete example](examples/basic/main.go) for imports and concrete types. Create utilities once before `lambda.Start`. Put the tracer wrapper outside the logger wrapper and use `l.WithContext(ctx)` inside the handler. Invocation attributes, log levels, and buffers then stay isolated. A root logger used directly retains process-level state. Context-bound loggers reject writes after the invocation returns; join background work before returning.

Logger includes six output levels plus SILENT, environment configuration, ALC precedence, persistent and temporary keys, child loggers, custom formatting, correlation IDs, event logging, and optional buffering. Sampling occurs at construction; the first invocation reuses that decision and warm invocations resample independently. WARN and higher use stderr; `WithOutput` replaces both streams. Buffering uses a valid OTel context or an X-Ray runtime root ID and is explicitly enabled with `WithBuffer`; unsampled OTel contexts work too. Active OTel context supplies trace/span IDs and the X-Ray-formatted log correlation ID. Set `logger.HandlerOptions.CorrelationSource` to a built-in source such as `logger.APIGatewayREST` or `logger.EventBridge`, or supply a custom `CorrelationID` callback, which takes precedence. `GetLevelName`, `GetLogEvent`, and `GetCorrelationID` expose the corresponding reference getters.

Metrics emits EMF documents with dimensions, metadata, repeated values, high resolution, automatic size-boundary flushes, and optional cold-start capture. It shares invocation identity and has independent request state. See [Metrics usage and compatibility](docs/METRICS.md). The development-only [reference generators](tools/reference/README.md) produce JSON fixtures from TypeScript; Lambda deployment uses Go binaries only.

Parameters provides SSM reads/writes/batches, Secrets Manager, DynamoDB, AppConfig Data, and AppConfig Agent. Shared caching and transforms remain independent of observability packages. Use injected SDK clients or lazily initialized default helpers; create providers outside the handler to reuse cached values. See [Parameters usage, API mapping, and compatibility](docs/PARAMETERS.md).

Commons supplies shared environment/runtime helpers, Base64, deep merge, LRU, type helpers, snapshots, precision-preserving DynamoDB conversion, and SDK identity. Metadata provides opt-in authenticated Lambda execution-environment retrieval with timeout, cancellation, and caching. Existing utilities now reuse compatible primitives; [the source/reuse audit](docs/COMMONS_REUSE.md) records what was shared and what remains module-specific. See [Commons and Metadata usage](docs/COMMONS.md). The historical 292-assertion Docker milestone covered Signer, JMESPath, Batch, DynamoDB Idempotency, real Valkey persistence and Parser stream, HTTP, Kafka, service, AppSync, Cognito and recursive-error integration.

Tracing captures handler and operation spans, errors/panics, annotations, optional responses and metadata, outbound HTTP, and AWS SDK v2 calls. Response/error capture defaults follow the reference; disable capture for sensitive payloads. Use `WithErrorHandler` for instrumentation and flush diagnostics; callbacks default to ignoring errors. Log methods and explicit metadata methods return errors to the caller.

## OpenTelemetry backend (default)

`tracer.New` creates a private provider with an OTLP/HTTP exporter when enabled in Lambda. Configure `OTEL_EXPORTER_OTLP_ENDPOINT` or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` for a reachable collector. For a Lambda collector extension listening on port 4318, the base endpoint is `http://localhost:4318`. The default localhost exporter destination is not itself a collector; install/configure the extension or supply another reachable OTLP endpoint. The library does not install a Lambda layer or change global OTel providers.

The collector can forward to AWS X-Ray or another supported destination. For X-Ray, configure the collector's `awsxray` exporter and Lambda execution-role permissions. The application uses the OTel X-Ray ID generator and propagator; these do not depend on the X-Ray SDK. The default sampler preserves parent sampling and samples roots; `OTEL_TRACES_SAMPLER` and `OTEL_TRACES_SAMPLER_ARG` can configure another SDK-supported policy. Invocation completion ends spans and attempts a flush with a two-second maximum, shortened by the invocation deadline. Delivery cannot be guaranteed on a hard timeout or a failing collector.

An application-owned provider can be supplied with `tracer.WithBackend(tracer.NewOTelBackend(provider))`. Configure its resource, sampler, X-Ray-compatible ID generator if required, exporter, limits, and shutdown in the application. The wrapper flushes supported providers but does not shut down an injected provider. `tracer.WithLocalTracing(true)` explicitly enables tracing outside Lambda.

```go
client := t.HTTPClient(http.DefaultClient)
ddb := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
    t.InstrumentAWS(&o.APIOptions)
})
```

Pass the invocation context to every HTTP/SDK operation. Instrument each client once; do not additionally install another tracing wrapper on the same transport or SDK middleware. HTTP and AWS SDK v2 instrumentation use the selected backend. Custom event context extraction is available through `tracer.HandlerOptions.ExtractContext`; event-source envelope extraction is not automatic.

## Deprecated X-Ray SDK adapter

`tracer/xray` is deprecated and frozen. Its module and Go APIs carry deprecation notices, and explicit construction prints an English migration warning once per process. Use the maintained OTel path above to send traces to AWS X-Ray. The current Lambda fixture no longer imports the legacy SDK and rejects `TRACE_BACKEND=xray`. See [XRAY_MIGRATION.md](docs/XRAY_MIGRATION.md) for collector configuration and migration steps.

See [compatibility and API mapping](docs/COMPATIBILITY.md), [the backend decision and research](docs/TRACING_DECISION.md), [the completion checklist](docs/CHECKLIST.md), and [validation evidence](docs/VALIDATION.md). Implementation priorities and remaining Go compatibility gates are tracked in [the roadmap](docs/ROADMAP.md).

Historical AWS acceptance testing in `ap-east-1` passed 206 scoped checks across 20 real invocations, covering both backends and CPU architectures. All temporary test resources were removed. New fixtures use OpenTelemetry exclusively. See [the AWS results and known gaps](docs/AWS_VALIDATION.md) and [the reusable test harness](integration/README.md).

AppSync Events has an independent [module and API guide](docs/APPSYNC_EVENTS.md), [native Lambda example](examples/appsyncevents/main.go) and [remaining checklist](docs/APPSYNC_EVENTS_PLAN.md). Publish/subscribe routing, individual/aggregate processing, authorization and size warnings reuse Commons without adding Parser, Logger, Tracer or AWS SDK dependencies.

AppSync GraphQL has an independent [module and API guide](docs/APPSYNC_GRAPHQL.md), [native Lambda example](examples/appsyncgraphql/main.go) and [remaining checklist](docs/APPSYNC_GRAPHQL_PLAN.md). It supports Query/Mutation/custom resolvers, aggregate or sequential batches, router inclusion, exception handlers and scalar helpers. Its only module dependency is root Commons. Core compatibility is checked against 114 actual TypeScript resolver scenarios and 91 scalar cases; full native/type/service compatibility remains open.

Bedrock Agents has an independent [function resolver module](docs/BEDROCK.md), [native Lambda example](examples/bedrock/main.go) and [remaining checklist](docs/BEDROCK_PLAN.md). It supports tool registration, ordered parameter conversion, explicit/default responses, session attributes and execution errors. Core behavior passes 371 actual TypeScript scenarios with exact body-string comparison; complete native/serialization/service compatibility remains open.

## License and attribution

Original contributions are licensed under the [MIT License](LICENSE), copyright (c) 2026 rambow-cloud contributors. Third-party content retains its original license. See [third-party sources and licenses](THIRD_PARTY_NOTICES.md) and the self-contained NOTICE in each Go module. This is an independent community project, not an official AWS distribution.
