# Commons and Metadata reference fixture

Generated from the pinned TypeScript v2.35.0 distribution by `tools/reference/generate-commons.mjs`.

Coverage includes 68 environment input/mode cases, 50 Base64 input/encoding cases, 20 runtime/development combinations, deep/indexed merge and cycle filtering, LRU access/eviction, 12 type/truthiness cases, nine DynamoDB number cases, a nested raw DynamoDB item, and Metadata cached/cleared/local calls with authenticated request inputs.

Normalization maps bytes and Sets to arrays, BigInt to `{ "bigint": "decimal digits" }`, nonfinite numbers to `{ "number": "NaN|Infinity|-Infinity" }`, and undefined to null. Reference errors retain their JavaScript category; Go assertions require failure and typed errors where the public Go API exposes them. Error wording is not required to match across languages.

Metadata responses are snapshotted before clearing because the reference mutates its returned cache object in place. The fixture replaces global fetch temporarily and restores it; it makes no network requests. The Go test drives a loopback HTTP server with the same response and compares path, authorization, cache request count, and local behavior.

Regenerate with `node generate-commons.mjs` from `tools/reference`. Normal Go tests read the stored JSON directly and do not require Node. Passing these fixtures establishes their scoped contracts, not every JavaScript edge case.
