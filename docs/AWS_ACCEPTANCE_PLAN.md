# Opt-in AWS service contract acceptance

Planning acceptance for [issue #92](https://github.com/rambow-cloud/powertools-lambda-go/issues/92), reviewed on 2026-10-08. Deferred stage 4 remains separate from local stages and PR CI. This document does not implement a runner, authorize AWS calls or certify free usage.

## Proposed first suite

Reuse [historical service evidence](AWS_SERVICE_ACCEPTANCE.md) with its original artifact/date limits. Select a small set of missing contracts rather than rerunning all historical examples. Pin the tested release or exact source SHA; an old pre-JSON-v2 sample is not current-source acceptance.

| Case | Observable result | Disposable resources and permissions |
| --- | --- | --- |
| IAM denial | A restricted assumed role is denied a direct uncached DynamoDB/SSM operation; the utility returns an operational failure rather than treating it as missing/invalid payload | Suite role and narrowly scoped inline policy; resource-specific DynamoDB GetItem and SSM GetParameter allows/denies |
| Real KMS masking | A small payload encrypts/decrypts through the existing uncached provider; authenticated context checks pass and a wrong expected context fails | One symmetric single-Region test key; GenerateDataKey/Decrypt on that key, plus explicit key policy |
| KMS denial | Missing required encryption context or explicit denied Decrypt produces a service failure; no plaintext result or success cache entry | Restrict only the suite role/key policy; do not alter account or application policies |
| SecureString Parameters | Fresh read with decryption returns the expected value; no-decrypt policy and KMS denial remain distinct from JSON-transform errors | At most two standard SecureString parameters under a suite prefix, Get/Put/DeleteParameter and the suite key |
| DynamoDB service boundary | An expired item can still be read physically; Idempotency's logical expiry/conditional claims remain correct | One minimal provisioned table, GetItem/PutItem/UpdateItem/DeleteItem and needed table/TTL setup permissions |

Inspect actual SDK/Encryption SDK error wrapping before choosing exact outer error assertions. Record service category and retained cause where exposed; do not assume every bridge preserves `smithy.APIError` identically. IAM changes are [eventually consistent](https://docs.aws.amazon.com/IAM/latest/UserGuide/troubleshoot.html#troubleshoot_general_eventual-consistency): use a bounded readiness phase distinct from the denied-operation assertion, and record failures rather than retrying until any result looks successful.

Physical TTL deletion is not a short-run assertion: use a fresh future item and a deliberately logically expired item, record the physical observation, and delete both during cleanup. Do not wait for background TTL removal. No stress, throttling flood, distributed topology or exhaustive policy coverage is proposed.

## Authorization and configuration

Before any network preflight, require an explicit request for the selected suite, nonempty `AWS_PROFILE` and `POWERTOOLS_TEST_ACCOUNT`, and region `ap-east-1`. Keep private values in ignored local configuration. Never fall back to the default profile, infer the intended account from current credentials or silently switch regions.

After authorization, an STS identity check must match the configured account before provisioning or test calls. STS itself is an AWS call; a dry/local mode does not make it. Reject account mismatch, missing credentials, unsupported region/service, unapproved resource reuse or unavailable cleanup permissions before creating resources. Dry-run planning must enumerate operations/resources without contacting AWS.

Separate provisioning/cleanup privileges from the restricted test role. Pass only the suite role and resources. The provisioner may create/delete the scoped table, parameters, key/alias and role, attach/detach that role's policies and assume it; use `iam:PassRole` only if a later authorized Lambda case actually requires it. Test credentials have only the per-case data-plane rights in the matrix. No production resource, account-wide policy, billing setting or default infrastructure is modified.

Ordinary PR CI runs local SDK fixtures and DynamoDB Local without credentials. A future cloud runner must be an explicit separate entry point with a default-off switch; no test merely detecting credentials may opt itself into cloud mode.

## Resource, time and cost bounds

Propose one table, one symmetric KMS key/alias, two standard SecureString parameters, one suite role and no Lambda/CloudWatch resources for this first suite. Use a unique run prefix and ownership tags plus an ignored local resource manifest. Verify names/tags/manifest identity before every cleanup mutation; existing resources are never adopted implicitly.

Bound the run to 20 minutes, at most 100 data-plane request attempts including retries, one concurrent case and payloads at most 1 KiB. Use low fixed table capacity, no autoscaling, indexes, backups, streams, advanced parameters, custom metrics, VPC/NAT, EC2, Managed Instances or durable executions. These are proposed controls to implement and test later, not a currently enforced budget.

Prepare a dated ap-east-1 cost estimate and get an explicit monetary ceiling before a cloud run. Count request attempts and stop new work when time/request bounds are reached; reserve time for cleanup. AWS budgets/alerts and eventual billing data are not immediate hard spending caps, and bounded calls do not guarantee zero charges.

[KMS pricing](https://aws.amazon.com/kms/pricing/) includes prorated customer-managed key storage and a request Free Tier subject to operation exclusions and account-wide usage. A free request allowance does not remove key-storage cost. Scheduling deletion stops scheduled-key storage charges under the stated pricing rules, but [key deletion takes a 7–30 day waiting period](https://docs.aws.amazon.com/kms/latest/developerguide/deleting-keys.html). If zero additional payment is mandatory, omit the key-creation/KMS suite unless the owner explicitly authorizes an existing disposable key and confirms the cost position; do not substitute an AWS-managed key for a custom key-policy test.

Review current regional pricing, account plan/credits and shared usage before execution; retain billing confirmation as an owner check. Closing this planning issue does not promise that future suites qualify for Free Tier.

## Cleanup and recovery contract

Register resources in the ignored manifest immediately after successful creation, before the next action. Cleanup runs on success, assertion failure, cancellation and provisioning failure; an interrupted process leaves a manifest that a separately authorized cleanup command can resume. Never recursively delete arbitrary cloud resources by prefix alone.

1. Stop new work and release restricted assumed-role sessions; do not depend on credentials being revoked instantly.
2. Delete only created parameters and wait for confirmed absence. Delete the created table and confirm removal through service state.
3. Remove the suite alias, schedule deletion of the created key with the approved waiting period, and confirm `PendingDeletion` plus the scheduled date. Report that pending deletion is not immediate physical deletion. Never schedule deletion of a supplied existing key unless separately authorized.
4. Remove suite role inline/attached policies, then delete the created role. Leave unrelated roles/policies intact.
5. Report each resource as removed, pending deletion or cleanup failed. A successful assertion run with failed cleanup is incomplete acceptance; retain the private manifest for recovery.

For any later CloudFormation-managed resources, use stack status and outputs as infrastructure evidence. Do not perform local DNS checks. This first suite requires no DNS or browser HTTP probe. Browser-visible deployment results should be verified by the owner using exact pages and steps.

## Implementation acceptance and public evidence

Before a runner is merged, local tests must prove default-off behavior, missing/mismatched account guards, no implicit credential fallback, bounded retries, request/time accounting, least-privilege role selection and cleanup after each partial provisioning stage. Reuse existing SDK interfaces and fixture infrastructure; no new mock framework is needed. Keep Go commands/subprocesses `CGO_ENABLED=0`, packaged modules `GOWORK=off` and Lambda binaries on Linux amd64/arm64 `provided.al2023` if later included.

Before claiming service acceptance, require actual per-case results for the approved artifact and record cleanup states. A sanitized public summary contains source/release identity, Go/module versions, date, region, scenarios, request totals, limitations and cleanup outcome. Omit account/profile identifiers, credentials, raw policies/endpoints, secret values, ciphertext samples and raw cloud responses; those stay ignored. Do not mix historical and fresh evidence or infer full parity from this suite.

## Decision

The services, permissions, resource lifetime, cost controls and cleanup ownership are specified for the deferred planning scope. Runner implementation and each cloud execution remain separate authorization decisions. This completes #92's plan; local stages and ordinary CI remain independently sufficient for their documented scope.
