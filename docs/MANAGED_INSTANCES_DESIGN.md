# Lambda Managed Instances lifecycle assessment

Planning acceptance for [issue #119](https://github.com/rambow-cloud/powertools-lambda-go/issues/119), reviewed on 2026-10-08. This design establishes requirements, not runtime/platform acceptance. Native Go Lambda on `provided.al2023` remains the verified target.

## Platform and runtime boundary

AWS describes [multi-concurrency and capacity providers](https://docs.aws.amazon.com/lambda/latest/dg/lambda-managed-instances.html). Its [runtime list](https://docs.aws.amazon.com/lambda/latest/dg/lambda-managed-instances-runtimes.html) explicitly lists Rust on the OS-only runtime and several managed languages, but does not explicitly list Go. That omission alone does not prove Go is impossible or lacks worker support.

The pinned `aws-lambda-go` v1.55.0 already reads `AWS_LAMBDA_MAX_CONCURRENCY` and starts concurrent Runtime API loops on Go 1.22+. It constructs per-invocation deadlines and Lambda contexts; `_X_AMZN_TRACE_ID` is updated only for single concurrency. Source: [worker entry](https://github.com/aws/aws-lambda-go/blob/v1.55.0/lambda/invoke_loop_gte_go122.go), [invocation handling](https://github.com/aws/aws-lambda-go/blob/v1.55.0/lambda/invoke_loop.go), [concurrency configuration](https://github.com/aws/aws-lambda-go/blob/v1.55.0/lambdacontext/context.go). The project uses Go 1.27, so this path is available. A replacement runtime loop is not justified by the current findings.

The missing project evidence is composed multi-worker/lifecycle acceptance and explicit deployment support. Keep Go platform support conditional until the current service/runtime contract and a separately authorized deployment are verified. No AWS resources are provisioned by this issue.

## Lifecycle differences and requirements

| Boundary | Managed Instances difference | Required project contract |
| --- | --- | --- |
| Invoke overlap | Several invocations share an execution environment; runtime-specific process/thread model | Every invocation has its own context, utility scopes, response and deadline |
| Idle periods | [Execution environments remain active](https://docs.aws.amazon.com/lambda/latest/dg/lambda-managed-instances-execution-environment.html), rather than freezing between invokes | No reliance on freeze for stopping background work or retaining pending buffers |
| Timeout | A timed-out invocation's code can continue; other invocations remain active | Cooperative cancellation and bounded work; no promise to kill arbitrary goroutines |
| Worker failure | Platform replacement differs from the default reset model | Determine SDK process/worker behavior separately from service behavior |
| Logging | [JSON log format is required](https://docs.aws.amazon.com/lambda/latest/dg/monitoring-cloudwatchlogs-logformat.html) | Verify complete structured record boundaries and EMF extraction, not only valid local JSON |
| Deployment | Capacity provider, published version and concurrency configuration differ from an ordinary function | Record exact SDK, architecture, image/binary, concurrency and control-plane configuration |
| Shutdown | Whole-process resources coexist with active invocation resources | Drain/flush with a bounded independent shutdown context, respecting injected resource ownership |

`aws-lambda-go`'s worker loops share a cancellation cause; a loop failure cancels the loops and the entry point waits for their completion. Do not equate the documented platform's surviving workers with a guarantee that every Go SDK panic preserves other goroutines. Test actual worker response and process termination before describing recovery.

## Existing primitives and proposed usage

Reuse `internal/invocation.Ensure`: a new incoming context obtains one immutable process-cold-start flag, while composed wrappers reuse it. Do not create an alternative invocation identity or per-utility cold-start counter. Concurrent first calls must elect one process cold start; request IDs and trace headers come from their request contexts, not shared environment mutation.

Logger wrappers own invocation buffers/temporary fields and finish them on return or panic. Metrics wrappers own stores and flush/close them; Tracer wraps each handler with a context span and bounded flush. These are source findings and existing functional concurrency contracts, not proof of every platform lifecycle. Keep configured base clients/exporters reusable and immutable; per-request work uses `WithContext` and caller context.

Specify late-write behavior after scope closure explicitly in new tests. A cancelled handler that ignores cancellation remains responsible for its goroutines; closing one scope must not flush, erase or attribute a sibling's state. SDK operations and application callbacks must use the invocation deadline. Never call process-wide `Shutdown` after each request or cancel an exporter shared by active invocations.

Keep process shutdown separate from handler cleanup. Stop accepting work, cancel/drain owned operations within a configured budget, finish spans, flush writers/exporters, then release only resources the application owns. Verify interrupted shutdown without promising delivery after SIGKILL, timeout or host loss. The existing default Lambda freeze/resume model still needs separate regression cases.

## Dedicated acceptance proposal

| Local case group | Observable acceptance |
| --- | --- |
| Native Runtime API workers | Launch pinned SDK with maximum concurrency one and greater than one; use barriers to demonstrate actual overlapping invocations, not only direct concurrent wrapper calls |
| Shared identity | Composed Logger/Metrics/OTel wrappers have matching request identity, isolated trace parents and one process cold start across simultaneous first invocations |
| Success/error/panic overlap | One failure does not contaminate sibling fields/stores/spans; record actual SDK loop/process outcome and map it to the platform model |
| Deadlines and late work | Cancel one request while another succeeds; both request deadline propagation and calls after scope closure follow documented behavior; no repeated EMF publication |
| Output | Concurrent logs/EMF remain complete parseable records, with per-request attributes; collector spans are assigned to the correct invocation |
| Idle/cache lifecycle | Continuous idle time, configuration immutability, cache expiry and shared-client ownership remain correct without freeze assumptions |
| Shutdown/default-runtime regression | Cooperative drain, bounded exporter flush, pending handler cancellation, abrupt loss and simulated freeze/resume have explicit limits |

Use the local Runtime API fixture and existing context/scope tests, with synchronization instead of scheduling sleeps. Keep `CGO_ENABLED=0`, no race detector, independent `GOWORK=off` packaged checks and Linux amd64/arm64 builds. Local fixtures cannot establish capacity-provider routing, worker replacement, CloudWatch extraction, host shutdown or actual service deadlines.

Cloud acceptance is a separate opt-in task with explicit `AWS_PROFILE`, `POWERTOOLS_TEST_ACCOUNT`, ap-east-1 availability and cost/resource approval. Capacity providers launch billable EC2 capacity; this is not a free default test. Use CloudFormation status/outputs for infrastructure evidence and record sanitized per-case results. Do not enable this suite in PR CI.

## Decision

Complete the existing design issue without adding runtime code. Prioritize local composed SDK worker acceptance, then establish the deployment contract and authorize a small platform suite if desired. Keep the frozen `tracer/xray` adapter out of new work; maintained delivery uses OTel and a collector's `awsxray` exporter. No Managed Instances compatibility claim is added by closing #119.
