# Metrics

The `metrics` package emits CloudWatch Embedded Metric Format (EMF) documents through an `io.Writer`, defaulting to stdout. It does not import Logger, Tracer, or an AWS SDK client. It reuses Commons configuration/string helpers and shared invocation identity, which keeps cold-start state consistent across wrappers. There is no network exporter in this package.

```go
m, err := metrics.New(
    metrics.WithNamespace("Orders"),
    metrics.WithServiceName("checkout"),
    metrics.WithDefaultDimensions(metrics.Dimensions{"environment": "production"}),
    metrics.WithErrorHandler(func(err error) { log.Printf("metrics flush: %v", err) }),
)
if err != nil {
    log.Fatal(err)
}

handler := func(ctx context.Context, event Event) (Response, error) {
    requestMetrics := m.WithContext(ctx)
    if err := requestMetrics.AddMetric("OrdersReceived", metrics.Count, 1); err != nil {
        return Response{}, err
    }
    return process(ctx, event)
}

lambda.Start(metrics.WrapHandler(m, handler, metrics.HandlerOptions{CaptureColdStart: true}))
```

The excerpt assumes application-defined `Event`, `Response`, and `process` symbols. The complete executable composition with Logger and Tracer is in [the integration handler](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/integration/lambda/main.go).

## Behavior

- `AddMetric` accepts all 27 reference units, standard resolution (60), and high resolution (1). Standard resolution is omitted from the EMF definition; high resolution emits `StorageResolution: 1`.
- Repeated names accumulate values, retaining the first resolution. A conflicting unit returns the reference diagnostic. Non-finite metric values serialize as JSON null; negative zero serializes as zero.
- The next addition after 100 distinct metrics flushes the existing document. Reaching 100 values for one metric flushes immediately. Automatic flush clears request dimensions and metadata, as the pinned TypeScript implementation does.
- `AddDimension` updates the regular dimension combination. `AddDimensionSet` creates an independent combination and maps to TypeScript `addDimensions`. Every combination includes the current defaults.
- `SetDefaultDimensions` merges defaults. `ClearDefaultDimensions` removes all defaults, including service. The reference's implemented limit is 29 combined keys per set, including the service key.
- `AddMetadata` snapshots JSON immediately. Dimension values override metadata; metric collisions with dimensions or metadata return an error during serialization.
- `SetTimestamp` accepts a Go `time.Time`; `SetTimestampMillis` accepts numeric milliseconds; `Serialize` returns JSON without clearing state. `Flush` emits a newline-terminated document and clears request state even on errors. `Clear` clears request state without emitting it.
- `ClearDimensions` removes regular dimensions and independent sets while retaining defaults. `ClearMetadata` removes only metadata. `ClearMetrics` removes values and the explicit timestamp without changing dimensions or metadata. `HasStoredMetrics` queries the active buffer without output.
- `SetThrowOnEmptyMetrics` changes the active scope's empty-buffer policy. The policy survives clear/flush and is snapshotted into new scopes. `ThrowOnEmptyMetrics` is the reference's deprecated enabling alias. A new single-metric instance uses the default empty policy, even if its parent is strict.
- `SingleMetric` returns `(*Metrics, error)` and reconstructs independent configuration from fresh environment settings, the parent's namespace and current defaults. Each addition flushes immediately. A single metric derived from an invocation-bound instance shares that invocation's closed state and output lock.
- The wrapper isolates concurrent invocations, emits an optional separate `ColdStart` document for on-demand initialization, flushes on success/error/panic, and rejects writes after closure. Use `WithContext` inside the handler and join background work before returning.
- Defaults changed through an invocation-bound instance stay local to that invocation. Root defaults provide a snapshot for new invocations. Pending root metrics are not copied into invocation state; flush them explicitly if recording outside the wrapper.
- Wrapper flush failures are reported through `WithErrorHandler`; the default preserves the original handler result, error, or panic, while `HandlerOptions.PropagateErrors` enables reference publication-error precedence. `WithRequireMetrics(true)` makes an empty explicit `Flush` return `ErrEmptyMetrics`; the wrapper reports that error through the same callback.

## Configuration

Configuration milestone: Verified 532 actual TypeScript configuration cases, exact custom getter order/errors, strict environment validation, single-metric reconstruction and constructor publication mode; all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 673/673 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks passed (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.

| Setting | Behavior |
| --- | --- |
| `POWERTOOLS_METRICS_NAMESPACE` | Namespace fallback; an empty namespace emits `default_namespace` |
| `POWERTOOLS_SERVICE_NAME` | Service dimension fallback; otherwise `service_undefined` |
| `POWERTOOLS_METRICS_FUNCTION_NAME` | Optional function dimension for `ColdStart` |
| `AWS_LAMBDA_INITIALIZATION_TYPE` | Cold-start metrics require `on-demand`; unknown and provisioned initialization do not emit them |
| `WithFunctionName` | Explicit constructor name, trimmed with Commons; an explicitly blank name suppresses the environment fallback |
| `POWERTOOLS_METRICS_DISABLED` | When present, overrides development-mode disabling; true/1/on disable and false/0/off enable |
| `POWERTOOLS_DEV` | Disables emission when the explicit disabled variable is absent |
| `WithDisabled` | Explicit Go configuration overrides the environment-derived value |
| `WithOutput` | Replaces the EMF writer; no dependency on a particular logging library |
| `WithWarningHandler` | Replaces the diagnostic callback; defaults to plain warning messages on stderr |
| `WithConfigService` | Supplies namespace/service fallbacks after explicit options and before cached environment values |
| `WithSingleMetric` | Enables immediate publication after each addition on the constructed instance |

Disabled emission still validates and buffers API additions; flushing clears state without output. Configuration is read during construction. Keep dimensions low-cardinality; put request IDs in metadata.

`New` validates `POWERTOOLS_METRICS_DISABLED` before applying options. Invalid or present-empty values return `*commons.EnvironmentError`, even with `WithDisabled`; accepted extended values include true/false, 1/0, yes/no, y/n, t/f and on/off. Invalid `POWERTOOLS_DEV` values retain the reference's false fallback. A present valid disabled variable takes precedence over development mode.

Namespace/service resolution follows explicit nonempty string, custom getter, cached environment and finally the service default. Explicit and custom strings are not trimmed before this decision: a whitespace-only service is selected and then skipped by dimension sanitization. Environment values are trimmed through Commons. The custom `ConfigService` exposes only `GetNamespace() (string, error)` and `GetServiceName() (string, error)`; the pinned implementation never calls its declared function-name getter. Getters run in namespace/service order only when needed; errors propagate unchanged. Environment snapshots are taken before getter calls, so a getter changing process state does not retroactively alter the parent configuration.

Single metrics use fresh construction rather than inheriting the parent's custom service, explicit disabled option, strict-empty policy, function-name setter or pending stores. Cleared defaults can therefore regain a service dimension; adding that dimension can also exceed the limit and return `ErrDimensionLimit`. A nonempty parent namespace is retained; an empty one falls back to the current environment. Output, warning/error callbacks and the injected clock remain shared Go integration facilities. Environment mutation is covered by sequential fixtures, not recommended as request-local configuration.

The pre-release API now requires handling construction errors:

```go
single, err := m.SingleMetric()
if err != nil {
    return err
}
return single.AddMetric("Completed", metrics.Count, 1)
```

This replaces the previous one-result `SingleMetric()` signature. Closed scopes reject construction as well as subsequent writes from previously derived instances.

## Reference coverage and remaining gaps

Store lifecycle milestone: Verified 104 actual TypeScript store lifecycle scenarios, 64 concurrent scope policies, late-write rejection, all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 631/631 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used. The 104 scenarios compare all timestamps and values with an injected clock, sorting only dimension-name arrays and mapping the specific empty-buffer error. Diagnostics have their own fixture and acceptance gate below.

The development-only `.mjs` generator executes `@aws-lambda-powertools/metrics@2.35.0`. Go compares eight actual emitted documents across four scenarios: dimensions/flush, 100-metric boundary, 100-value boundary, and isolated single metrics. Normalization removes timestamps and sorts dimension-name arrays only. Neither Node.js nor these generators are included in a Lambda deployment.

The first implementation has known differences that remain parity work:

- Go dimension arguments are typed strings. Non-string JavaScript arguments have no direct Go equivalent. Non-index keys in map batches use lexical order because Go maps cannot retain JavaScript insertion order.
- `_aws` now follows reference overwrite precedence, including output that no longer has a valid EMF envelope. Metadata still uses Go JSON serialization and snapshot timing rather than mutable JavaScript references.
- Wrapper instrumentation errors are reported rather than replacing business errors. Go functional options and explicit context binding replace decorators and automatic disposal.
- Selective clears, runtime policy, manual cold-start APIs, custom configuration, single-metric reconstruction and numeric/Date timestamp inputs are implemented. Internal store getters are not public Metrics APIs. Remaining decorator/framework lifecycle differences, exported-type/metadata encoding and service acceptance gates remain in [METRICS_PLAN.md](METRICS_PLAN.md).
- Exhaustive exported-symbol and invalid-input differential coverage, performance budgets, and real CloudWatch metric extraction for this new package remain pending.

The Docker suite checks emitted EMF, cold-start isolation, and error/panic flush alongside logs and OTLP spans. It does not establish CloudWatch service-side ingestion or metric visibility.

## Explicit metric scopes

`ctx, finish := metric.StartScope(ctx)` creates fresh request storage using the active scope's default dimensions. It shares configuration and the synchronized writer. Retrieve the bound instance with `metric.WithContext(ctx)` and call `finish()` after the work completes. Finish flushes once, retains its error for repeated calls, and rejects late writes, including derived single metrics. Pending parent metrics, temporary dimensions, metadata and timestamps remain in the parent. This primitive is reused by the Lambda wrapper and optional [HTTP middleware](HTTP_OBSERVABILITY.md).

## Diagnostics

Diagnostic milestone: Verified 203 actual TypeScript diagnostic scenarios, Unicode whitespace reuse, callback reentrancy/concurrent scopes, automatic flush and failed-output delivery, all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 646/646 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.

Empty or whitespace-only dimension names/values are skipped with warnings. Commons supplies JavaScript-compatible whitespace checks, including BOM removal and preservation of U+0085 and U+200B; accepted values retain their original whitespace. Duplicate dimensions and metadata/dimension collisions report the reference precedence. Metric collisions remain errors. An empty namespace warns when serialized and uses `default_namespace`. Empty non-strict flushes warn even when emission is disabled. Strict empty serialization returns `ErrEmptyMetrics` before namespace diagnostics.

`SetTimestamp` stores timestamps outside the inclusive window from fourteen days before the clock to two hours after it and emits the reference warning. It does not silently replace the supplied timestamp. CloudWatch acceptance of such documents is outside the local fixture's scope.

Warnings are collected under the storage lock and delivered after storage and output locks are released. Flush cleanup happens before callback delivery, including failed writes; scope completion also marks the scope closed first. `WithWarningHandler` may run during construction or concurrently across requests. A callback may inspect Metrics or log through another utility, but must synchronize its own shared state, avoid panics, and avoid recursively finishing the same scope. Callback panics propagate. Use a no-op callback to suppress warnings. Default stderr diagnostics are separate from the EMF writer.

The warning generator records 203 actual TypeScript v2.35.0 scenarios covering invalid/duplicate dimensions, collision precedence, empty/disabled/strict behavior, namespace fallback, timestamp boundaries and Unicode whitespace. It compares exact warnings, errors and full documents, normalizing only dimension-name order. This is scoped reference evidence; arbitrary JavaScript value types and broader encoding parity remain open in [METRICS_PLAN.md](METRICS_PLAN.md).

## Manual cold-start metrics

Cold-start milestone: Verified 302 actual TypeScript cold-start cases, 64 concurrent captures, failed-write consumption, scoped function-name isolation and closed-scope rejection; all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 658/658 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks passed (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.

Call `m.WithContext(ctx).CaptureColdStartMetric("fallback-name")` inside an invocation scope, or `m.CaptureColdStartMetric()` for a standalone instance. It uses `commons.Utility` to consume the instance's on-demand cold-start decision once. Scope-bound calls additionally honor shared invocation identity, so constructing another Metrics instance in a warm handler does not turn that invocation cold. The wrapper calls this same method with the Lambda function name as its fallback. Manual calls after wrapper capture do not duplicate the metric.

Capture emits a separate `ColdStart` count using default dimensions, without consuming parent metrics, request dimensions, metadata or the parent's explicit timestamp. Repeated and concurrent calls on one instance emit at most once. The decision is consumed before output, including disabled output and failed writes; there is no automatic retry. A separately created `SingleMetric` has its own cold-start helper, matching the reference's new-instance behavior, while sharing invocation closure.

Function-name precedence is the configured constructor/environment name, then the capture argument. `WithFunctionName("")` suppresses the environment fallback but leaves the capture argument available. Deprecated `SetFunctionName("")` instead explicitly suppresses the capture argument. The setter stores the raw value; capture applies JavaScript-compatible trimming. Setters are scope-local, new scopes inherit a snapshot, and clear/flush preserve the name. Closed scopes reject setters and capture calls. A new single-metric instance resolves its function name from the environment rather than copying the parent's setter.

302 actual v2.35.0 cases compare full EMF documents and warning strings across constructor/environment/setter/argument combinations, Unicode whitespace, initialization types, disabled output, clearing, repeated captures and derived single metrics. Go functional tests additionally exercise concurrent capture, failed-output/construction consumption and scope-name isolation. Another 532 configuration cases cover fresh environment/default reconstruction, constructor validation, custom getter order/errors and immediate-publication mode. Only dimension-name order is normalized; complete documents, warning strings and constructor errors are compared.

## Metric values and object keys

Value milestone: Verified 641 actual TypeScript value/error/key scenarios, exact warning/configuration error messages, shared key-order regression through Parser/Validation, all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 688/688 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.

The pinned implementation accepts NaN and positive/negative infinity as metric numbers. Go retains these values in the buffer and emits null in scalar and accumulated JSON values. This mirrors reference serialization; it does not establish CloudWatch ingestion for those values. Metric names use UTF-16 length and exact reference name/unit/resolution/conflicting-unit errors. The existing ErrEmptyMetrics and ErrDimensionLimit identities remain usable with errors.Is; their messages now match the reference directly.

Numeric metric names follow JavaScript object enumeration: canonical indices from 0 through 4294967294 precede other names and sort numerically. Other metric names retain insertion order. Commons.SortObjectKeys supplies the shared rule also used by Parser Kafka envelopes and Validation diagnostic traversal. Dimension-name arrays retain their documented normalization.

The generated _aws envelope has lower precedence than user metadata, dimensions and metric values. A supplied _aws key can therefore replace it, matching the pinned implementation; this no longer returns the former Go-only reserved-key error. Ordinary metric-versus-dimension/metadata collisions still fail. Default Object.prototype names used as metric names produce the reference's conflicting-unit error with an undefined existing unit. The special __proto__ name creates no own dimension or metadata property; single-dimension capacity projection still occurs before that ignored assignment, as upstream does. This models observable default-object behavior, not arbitrary JavaScript prototype mutation.

641 actual TypeScript scenarios cover all units, valid/invalid resolutions, UTF-16 length boundaries, non-finite and negative-zero values, inherited names, overwritten envelopes, exact errors, ordered metric definitions and automatic flush/recovery in enabled/disabled, strict/non-strict and single/buffered modes. Only dimension-name arrays are sorted for comparison. Metadata mutation timing, arbitrary native values, malformed strings and remaining numeric/encoding boundaries stay under M-EDGES.

## Timestamp inputs

Verified 968 actual TypeScript numeric/Date timestamp scenarios, exact warnings/errors and clock-read counts, 64 concurrent scopes and late-write rejection; all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 706/706 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks passed (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.

`SetTimestamp(time.Time)` maps an instant to JavaScript Date semantics at millisecond precision. The inclusive representable range is plus or minus 8,640,000,000,000,000 milliseconds since the epoch. Out-of-range instants warn and serialize as JSON null. Submillisecond Go instants are truncated to a millisecond boundary, including instants before the epoch. Go's zero time represents year 1; it is not an invalid Date.

`SetTimestampMillis(float64)` maps the public numeric overload. Finite integers are retained, including integers beyond the Date range. Fractional values, NaN and infinities warn and become zero. This conversion differs from constructing a JavaScript Date from a number, which applies TimeClip first.

Both setters warn for timestamps outside the inclusive interval from fourteen days before the injected clock to two hours after it. The warning does not discard the timestamp or change valid integers. Numeric validation rejects non-integers before reading the clock. Date validation reads the clock even for invalid Dates. Serialization reads the clock only when no explicit timestamp is stored, including after ClearMetrics/flush; stored zero and null suppress that fallback.

The 968-case reference suite compares full timestamps, warning/error text and clock call counts across numeric/Date input, clear, single-metric, disabled and strict-policy lifecycles. Functional tests cover 64 concurrent scopes and late writes. Metadata/native encoding and the complete public type audit remain separate gates.

## Lambda wrapper options and multiple instances

Verified 876 actual TypeScript middleware-hook cases, exact output/warning/error and publication-order comparisons, 64 concurrent nested invocations, option snapshots, panic precedence and callback/writer cleanup; all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 724/724 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks passed (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.

`WrapHandler` and `WrapHandlers` share one implementation. The latter accepts an ordered `[]*Metrics`; an empty list simply invokes the handler. Nil elements panic when creating the wrapper. Both APIs snapshot the target list and default-dimension map at construction.

```go
wrapped := metrics.WrapHandlers(
    []*metrics.Metrics{applicationMetrics, auditMetrics},
    handler,
    metrics.HandlerOptions{
        CaptureColdStart:    true,
        ThrowOnEmptyMetrics: true,
        DefaultDimensions:  metrics.Dimensions{"environment": "production"},
        PropagateErrors:    true,
    },
)
lambda.Start(wrapped)
```

For each owned target, the wrapper enables a truthy ThrowOnEmptyMetrics policy, merges non-nil DefaultDimensions, then optionally captures cold start. False does not disable an inherited strict policy. Defaults and policy remain request-local; concurrent calls and the parent instance are unaffected. Use each target's WithContext(ctx) inside the handler.

Targets publish in input order after normal returns, business errors, panics and early returns from the handler. Duplicate targets share storage but retain repeated setup/publication operations, including empty-buffer diagnostics on the later publication. Publication stops at the first error. Every scope owned by the wrapper closes, including later instances whose metrics could not be published; their buffers are discarded. Existing outer scopes remain owned by the outer wrapper and are neither reconfigured nor published by an inner wrapper.

The failing target's WithErrorHandler observes a publication error after all owned scopes close. By default, the business result/error/panic survives. With PropagateErrors, a publication error returns the zero result and replaces the business error or panic, matching the reference finally/after precedence through Go's error return. With no publication error, the original panic is rethrown unchanged. Preparation failures always skip the business handler, report and return the error, and close created scopes without application publication. A panic from a writer or callback propagates while cleanup still closes all owned scopes.

Compatibility boundaries remain explicit: Go snapshots mutable options and isolates each invocation, whereas the JavaScript middleware reuses its instances. The decorator configures defaults when decorating; these Go wrappers configure the invocation like Middy's before hook. Middy-specific repeated error-hook scheduling is not represented by direct hook fixtures. Go does not preserve unpublished failed-invocation buffers for a later invocation. Broader decorator/framework equivalence remains under M-WRAPPER; use PropagateErrors when migrating reference error precedence.
