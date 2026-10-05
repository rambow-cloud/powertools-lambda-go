# Parameters reference fixture

`aws-http-scenarios.json` contains manually maintained, synthetic AWS HTTP
responses. Every scenario records official API contract URLs, the pinned service
SDK version and its review date. These are constructed contract examples, not
captured AWS traffic. Status, headers, raw body and the triggering request are
preserved separately. Provider expectations are asserted independently in Go.
The malformed-binary case deliberately violates the successful response contract.

The strict loopback server checks method, path, operation, selectors and pagination
tokens, then consumes the ordered responses exactly once. Cases cover SSM JSON,
throttling recovery, bounded server-error retries, missing parameters and pages;
Secrets Manager binary decoding/missing/malformed values; DynamoDB native values,
missing tables and pages; and AppConfig Data header tokens and unchanged bodies.
Retry delays are zero for deterministic tests while the real SDK retryer runs.
Authorization presence verifies signing was attempted; this server does not
validate signatures, IAM or service-side quotas. Existing inline provider replies
are also synthetic and check SDK/provider composition, without cloud provenance.

`typescript-v2.35.0.json` was generated from the installed, pinned TypeScript distribution using `tools/reference/generate-parameters.mjs`. The baseline is v2.35.0, commit `7bcc27b1574493f9452688673658f52b80c53847`.

The generator executes ten single-value/cache cases, automatic multiple transforms with permissive/strict failure handling, and three SSM batch calls. It records returned values, typed error names, retrieval counts, and SDK command inputs. The SSM SDK client is injected and its send method is replaced with deterministic local responses. No AWS service is contacted.

Normalization maps JavaScript undefined to JSON null, including failed transform and empty batch entries. Error text and stack traces are excluded. SDK comparison ignores an explicit false WithDecryption field from the Go serializer because omission has the same request meaning. The fixture preserves the pinned mixed-decryption cache/transform bypass behavior.

Regenerate from `tools/reference` with `node generate-parameters.mjs`. Ordinary Go tests read the checked-in JSON and do not require Node.js. These scenarios establish scoped behavioral compatibility, not exhaustive Parameters parity.
