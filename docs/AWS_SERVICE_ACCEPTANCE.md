# Scoped AWS service acceptance

Disposable examples ran in ap-east-1 on 2026-10-07, before the Go 1.27/JSON v2
merge. These are historical results for those artifacts/scenarios, not fresh
acceptance of current main or v1. Sanitized milestones are in [CHECKLIST.md](CHECKLIST.md);
source, runners and raw evidence stay ignored. Publication is tracked in
[#114](https://github.com/rambow-cloud/powertools-lambda-go/issues/114).

## Verified examples

| Area | Accepted scope | Limits |
| --- | --- | --- |
| Lambda Logger/Metrics | Both Linux architectures, provided.al2023, CGO disabled; six calls including handler errors, cold/warm behavior and shared log/metric request identity | No tracing, stress or billing acceptance |
| CloudWatch | Six structured request logs; Requests Sum=6 and ColdStart Sum=2 extracted across three custom metric series; metrics flushed on errors | No quota/throttling or hard-timeout delivery guarantee |
| DynamoDB | Three suites/six subcases: one winner among 16 conditional claims, prior-item conflicts, completion, exact integers, fresh-manager replay, payload validation, logical expiry/leases, protected completion, deletion and composite tenant keys | No Lambda invocation, Streams, physical TTL removal, stress or billing acceptance |
| DynamoDB Parameters | JSON/cache/force refresh/clear, native binary/missing policies, Limit=1 pagination and transforms | No exhaustive IAM/KMS/throttling acceptance |
| Standard SSM | Three suites: JSON/cache/overwrite/versioning/refresh/clear/zero-age/missing/cancellation; path isolation/pagination; 11-name batch chunking and strict/graceful missing policies | Standard String/throughput only; no SecureString/KMS |
| SigV4 | Regional STS through standalone signed GET and signed-transport POST | Disposable certificate-verified TLS 1.2 workaround; library defaults unchanged |

Both temporary functions, log groups, inline policy and role were removed. Five
DynamoDB tables were deleted and deletion confirmed. All thirteen temporary SSM
parameters were deleted and absence confirmed. Metric history follows CloudWatch
retention; it is not a separately deletable resource.

A separate CGO-disabled native workspace audit passed 37 packages with 35,478
test events, including subtests. It excluded Linux-only KMS and Docker/runtime
acceptance; it is not a packaged/public-version run.

## Remaining service gates

Historical OTel collector-to-X-Ray results remain in [AWS_VALIDATION.md](AWS_VALIDATION.md).
Do not combine its dates/artifacts with these examples to imply a fresh full run.

- Batch SQS/FIFO and stream service retries/checkpoints.
- Tracer hard-timeout/freeze recovery and broader service-map behavior.
- Actual cloud HTTP/Lambda streaming.
- Secrets Manager/AppConfig, IAM rejection, encryption changes and throttling.
- Real KMS policies/wrapping and optional masking-provider acceptance.
- Redis/Valkey expiry, cluster redirects/failover, TLS and authentication.
- AppSync, Bedrock and Kafka managed-service acceptance.

The planning-only [#92](https://github.com/rambow-cloud/powertools-lambda-go/issues/92)
tracks a small opt-in suite. Reuse existing evidence and select remaining cases
from the declared support scope. New calls require explicit authorization,
profile/account guards, a cost/resource plan and cleanup. These results do not
certify account-wide billing or universal Free Tier availability.
