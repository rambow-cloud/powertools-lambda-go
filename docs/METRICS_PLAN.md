# Metrics compatibility checklist

Reference: the installed TypeScript v2.35.0 `Metrics.js`, its declaration file, and `MetricsStore.js`. This checklist separates the public API from internal store implementation details.

- [x] M-CORE: Implement EMF emission, metric/dimension boundaries, default dimensions, metadata snapshots, single metrics and isolated Lambda scopes. Original four reference scenarios produce eight EMF documents; see METRICS.md.
- [x] M-STORES: Implement public selective clears, metric presence, scoped runtime empty policy, deprecated enabling alias, timestamp clearing and single-instance defaults. Verified 104 actual TypeScript store lifecycle scenarios, 64 concurrent scope policies, late-write rejection, all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 631/631 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.
- [x] M-WARNINGS: Implement skipped invalid dimensions, duplicate/collision diagnostics, empty/default-namespace warnings and out-of-range timestamp storage with warnings. Deliver callbacks after storage/output unlock and scope cleanup. Verified 203 actual TypeScript diagnostic scenarios, Unicode whitespace reuse, callback reentrancy/concurrent scopes, automatic flush and failed-output delivery, all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 646/646 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used. Typed-input, reserved-key and Go map-order boundaries remain explicit under M-EDGES.
- [x] M-COLD: Implement CaptureColdStartMetric and deprecated SetFunctionName, nullish name precedence, Commons whitespace/initialization reuse and shared wrapper/manual consumption. Verified 302 actual TypeScript cold-start cases, 64 concurrent captures, failed-write consumption, scoped function-name isolation and closed-scope rejection; all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 658/658 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks passed (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used. Single-metric configuration reconstruction is verified separately under M-CONFIG.
- [x] M-CONFIG: Implement strict constructor environment validation, explicit/custom/environment precedence, a minimal ConfigService, WithSingleMetric and fresh SingleMetric construction with explicit errors. Verified 532 actual TypeScript configuration cases, exact custom getter order/errors, strict environment validation, single-metric reconstruction and constructor publication mode; all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 673/673 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks passed (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used. The pre-release SingleMetric API now returns (*Metrics, error); output/clock/callback sharing and typed Go options are explicit language mappings.
- [x] M-VALUES: Implement non-finite/negative-zero JSON values, exact metric/sentinel errors, numeric definition order, reserved-envelope precedence and default-prototype name behavior. Verified 641 actual TypeScript value/error/key scenarios, exact warning/configuration error messages, shared key-order regression through Parser/Validation, all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 688/688 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used. Preserve broader encoding/type boundaries below.
- [x] M-TIMESTAMP: Implement SetTimestampMillis and Date-range mapping, numeric invalid-to-zero versus invalid-Date-to-null conversion, inclusive time windows and lazy clock reads. Verified 968 actual TypeScript numeric/Date timestamp scenarios, exact warnings/errors and clock-read counts, 64 concurrent scopes and late-write rejection; all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 706/706 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks passed (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used.
- [ ] M-EDGES: Complete exported-symbol/type/error/encoding and reserved-key behavior, including deliberate Go differences. Do not infer public methods from private storage classes.
- [x] M-WRAPPER-HOOKS: Implement HandlerOptions defaults/strict policy and WrapHandlers with ordered/duplicate targets, shared scope lifecycle and opt-in reference error precedence. Verified 876 actual TypeScript middleware-hook cases, exact output/warning/error and publication-order comparisons, 64 concurrent nested invocations, option snapshots, panic precedence and callback/writer cleanup; all 22 packaged modules/19 consumers, both CGO-disabled Linux builds, 724/724 RIE assertions, 95/95 streaming Runtime API checks and 14/14 Batch artifact checks passed (2026-09-22). Docker ran amd64; arm64 was cross-compiled. No AWS resources were used. Complete decorator/framework compatibility remains separate.
- [ ] M-WRAPPER: Complete decorator initialization timing, mutable option/native async behavior, framework-driven repeated error/cleanup hooks and the explicit Go invocation-isolation/default error-policy mapping. Core ExtraOptions and ordered/duplicate multi-instance behavior have direct middleware fixtures; these do not prove the complete decorator/Middy framework contract.
- [ ] M-RELEASE: Verify real CloudWatch extraction, performance/resource budgets, API documentation and publication requirements.

## Public store API mapping

| TypeScript public API | Go API | Lifecycle |
| --- | --- | --- |
| clearDimensions | ClearDimensions | Clear regular dimensions and independent sets; retain defaults and other stores |
| clearMetadata | ClearMetadata | Clear metadata only |
| clearMetrics | ClearMetrics | Clear metric values/order and explicit timestamp |
| clearDefaultDimensions | ClearDefaultDimensions | Clear defaults, including service |
| hasStoredMetrics | HasStoredMetrics | Read current scope; no mutation or output |
| setThrowOnEmptyMetrics | SetThrowOnEmptyMetrics | Change active policy, retained by clear/flush |
| throwOnEmptyMetrics | ThrowOnEmptyMetrics | Deprecated alias that enables the policy |
| serializeMetrics | Serialize | Snapshot without clearing or emitting |
| publishStoredMetrics | Flush | Publish and clear temporary stores, including on failure |

The declaration marks `isDisabled` protected; the Go `Disabled` convenience already exists. Methods such as `getAllMetrics`, `getDimensions`, and `getTimestamp` belong to internal store classes, not the public Metrics contract. No extra store getters are required merely because they appear in that implementation.

Go stores runtime policy in synchronized request state. New scopes inherit a snapshot; changes do not mutate the parent or sibling requests. Policy values are immutable once stored, so snapshots do not share mutable configuration. Single metrics start with the reference's default empty policy rather than inheriting a parent's strict policy. Go clear/set methods reject mutations after scope closure; `HasStoredMetrics` remains a read-only query.

The fixture generator freezes Date.now and normalizes only dimension-key order and the specific reference empty-metrics RangeError into the Go `ErrEmptyMetrics` identity. Full EMF timestamps and values are compared. Diagnostics are verified separately under M-WARNINGS and are not inferred from the store fixture.

## Cold-start source audit

The installed v2.35.0 `captureColdStartMetric` calls the inherited `Utility.getColdStart` before constructing an independent single metric. The utility consumes its per-instance flag and returns true only for on-demand initialization. Go already exposes this primitive as `commons.Utility`; wrapper invocation identity is a separate shared contract. The manual API reuses Utility while bound scopes also consult shared invocation identity. M-COLD verifies repeated calls, multiple instances, warm requests, disabled output and failed writes without duplicating wrapper capture.

Function-name selection uses nullish precedence after trimming, not a simple nonempty fallback. Construction ignores an explicitly empty trimmed name without falling back to the environment, while the deprecated setter stores the supplied value directly; an explicitly empty setter value suppresses the manual argument. The constructor reads `POWERTOOLS_METRICS_FUNCTION_NAME`; the deprecated method's comment mentions a different variable and must not override the executable contract. Use Commons JavaScript-compatible trimming rather than adding another local whitespace rule.

The reference `singleMetric()` constructs a new Metrics instance using namespace, default dimensions, single-metric mode and the logger. It does not copy every parent configuration field. Its behavior under explicit disabling and environment changes is covered by the 532-case configuration fixture. Shared Go output, clock, callbacks and scope closure remain explicit integration facilities.

## Configuration source mapping

The installed constructor reads and validates environment settings before applying explicit options or the custom configuration service. Its extended boolean helper throws on invalid or present-empty `POWERTOOLS_METRICS_DISABLED` values. Go now returns the shared typed EnvironmentError before applying options; direct constructor-failure fixtures cover invalid values and explicit Go overrides.

Namespace and service use JavaScript truthiness for explicit/custom values, then cached environment values. In particular, an explicit whitespace-only service name is truthy and subsequently skipped by dimension sanitization; Metrics now retains this utility-specific policy in configuration.go instead of changing Commons.ResolveServiceName for all consumers.

The Metrics configuration-service interface declares a function-name getter, but the executable constructor never calls it. Namespace and service getters are short-circuited by explicit truthy values. The Go interface therefore contains only the two called getters. Reference fixtures verify call order and exceptions.

Reconstructing a single metric can introduce a default service after defaults were cleared, re-read disabling/namespace environment settings, or throw during construction. Go now returns (*Metrics, error) and reuses New after releasing parent storage. Fixtures verify reintroduced service dimensions, overflow, refreshed environment settings and independent policies. The clock, output/callbacks and invocation closure are shared Go facilities.

## Value and remaining encoding audit

The pinned storeMetric validates JavaScript number type rather than finiteness. Go now accepts NaN and infinities and emits JSON null, including accumulated arrays, disabled emission and automatic-flush scenarios. Negative zero emits zero. The 641-case value fixture compares emitted values and complete diagnostics directly.

`serializeMetrics` places `_aws` first and then spreads metadata, dimensions and metric values. Go now preserves overwritten envelopes and ordinary collision checks. Default Object.prototype metric names and ignored __proto__ metadata/dimension assignments have actual reference coverage; arbitrary prototype mutation is not emulated. Metric names use UTF-16 length, and name/unit/resolution/conflicting-unit diagnostics match the fixture exactly.

Metadata stores retain JavaScript values until serialization, whereas Go snapshots JSON at insertion. Reference tests must separate mutation timing, unsupported native values, numeric precision and JSON encoding from ordinary dimension precedence. Full parity and performance/service gates remain open after the constructor milestone.

## Finite metadata and lifecycle audit #183

- [x] M-FINITE-183: The matrix passes both affected packaged modules and two
  independent consumers with CGO disabled/GOWORK off, scoped JSON v2 and
  documentation/discovery checks (2026-10-13). Hosted checks remain independent.

Baseline: TypeScript v2.35.0 Metrics/store/middleware implementations. Reuse the
641 value cases, 968 timestamp cases, 876 ordered/duplicate wrapper-hook cases,
scope nesting/idempotent finish and callback-panic regressions, and HTTP's 176
reference cases plus request-count opt-in coverage. The additional generator
records 15 actual metadata observations without changing the original fixtures.

| Public surface | Evidence and classification |
| --- | --- |
| `AddMetadata`, `Serialize` | Verify scalars, nested objects/arrays, reserved envelope/constructor keys and ignored __proto__. Two upstream live-reference mutation observations intentionally map to Go insertion snapshots; three upstream non-finite-to-null observations map to native JSON v2 rejection. Ordinary values compare the full EMF document. |
| Native metadata encoding/ownership | Verify exact int64 9007199254740993, private returned bytes, snapshot mutation, failed custom marshaler cause and one callback, and atomic retention of previous metadata after invalid/non-finite/duplicate-key/UTF-8/channel values. Typed Go byte/number/object behavior is JSON v2, not JavaScript runtime emulation. |
| `StartScope`, `WithContext`, late `AddMetadata` | Reproduce serialization of caller values after scope closure and fix it with an early closed check. The existing locked recheck still rejects a serialization that overlaps finish; a channel-fenced test proves it cannot add metadata after publication. Closed ignored keys also retain ErrInvocationClosed. |
| `WrapHandlers`, handler options | Thirty-two concurrent two-instance invocations verify independent metadata/EMF, original target-list/default-dimension snapshots and retained late-write rejection. Existing 876 cases retain ordered/duplicate target and publication/business-error decisions; callback-panic tests prove closure of every owned scope. |
| Optional HTTP Metrics `New`, `Options` | Existing 176 pinned cases retain latency/fault/error defaults; opt-in request count tests retain their additional metric, bounded route, parent/context restoration, concurrent scope and streaming publication policies. No middleware default or OTel policy changes. |

The early guard adds no lock around user serialization and retains the final
closed-state recheck. Default JSON v2, snapshot timing, scope ownership and
business-preserving errors remain intentional Go contracts. M-EDGES/M-WRAPPER
exhaustive symbol/decorator/framework/native acceptance and M-RELEASE/resource
#200/cloud gates remain open; this finite audit introduces no public API or
dependency and performs no AWS calls.

Declaration audit: the pinned Metrics.d.ts declares AddMetadata's value as a string even though the executable store accepts arbitrary JavaScript values. Go's any-valued AddMetadata is a broader API. Keep declared string behavior, JavaScript runtime extensions and Go-native serialization behavior distinguishable in subsequent fixtures; a passing ordinary string case does not prove mutable-object or custom-marshaler equivalence.

## Wrapper source audit

The installed v2.35.0 package exports three entry points: the root Metrics/MetricUnit/MetricResolution values, middleware/logMetrics and types. ExtraOptions declares throwOnEmptyMetrics, defaultDimensions and captureColdStartMetric. HandlerOptions now maps all three fields directly. WrapHandlers accepts ordered and duplicate target instances, while WrapHandler delegates to the same lifecycle.

Metrics.logMetrics applies truthy throwOnEmptyMetrics and defined defaultDimensions when creating the decorator. The Middy middleware applies them in before on each invocation, then optionally captures cold start. A false throwOnEmptyMetrics option does not disable a previously enabled strict policy. Middleware accepts a Metrics instance or an array, iterates in input order and installs its cleanup callback in request.internal[METRICS_KEY] only after before completes. Its after and onError share the publication loop. A failure can stop later instances, so simply nesting Go wrappers is not proof of parity. Go retains business-preserving publication errors by default and now offers PropagateErrors to match reference precedence. The fixture selects that option explicitly and compares exact failure messages and stopped publication.

Source evidence: tools/reference/node_modules/@aws-lambda-powertools/metrics/package.json, lib/esm/types/Metrics.d.ts, lib/esm/middleware/middy.js and lib/esm/Metrics.js from the locked 2.35.0 installation. Public type export mapping, constructor options and wrapper behavior must all be accounted for before closing M-EDGES/M-WRAPPER. Metrics.d.ts also exports the symbol-named [Symbol.dispose] method; the public-symbol audit must account for this explicit-resource-management contract rather than searching only identifier-named methods.
