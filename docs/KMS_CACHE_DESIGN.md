# Bounded KMS materials caching

Planning acceptance for [issue #116](https://github.com/rambow-cloud/powertools-lambda-go/issues/116), reviewed on 2026-10-08. The optional `datamasking/kms` provider stays uncached; no cache implementation or cryptographic defaults change here.

## Upstream and current API findings

The installed Powertools TypeScript v2.35.0 provider constructs `NodeCachingMaterialsManager` from `@aws-crypto/client-node` v5.0.2. Its defaults are capacity 100, age 300 seconds, maximum encrypted messages 4,294,967,296 and maximum encrypted bytes `Number.MAX_SAFE_INTEGER`. These are reference defaults, not proposed safe defaults for every Go application.

| Boundary | Finding |
| --- | --- |
| Current Go provider | Uses Go Encryption SDK/MPL v0.4.0; each operation builds a context-bound KMS multi-keyring; encryption obtains fresh materials |
| Go SDK injection seam | `EncryptInput` and `DecryptInput` accept `ICryptographicMaterialsManager`; the provider currently supplies a keyring |
| MPL constructors | Exports default/required-context CMMs, a cryptographic materials cache and an AWS KMS hierarchical keyring; no Node-style caching CMM constructor in the pinned API |
| Hierarchical alternative | AWS documents Go support, but it requires a key store/branch-key model and changes which keyring can decrypt the message |
| Compatibility conclusion | AWS supports caching mechanisms; this project's provider has no bounded-cache feature. The available MPL cache is not a drop-in NodeCachingMaterialsManager |

Source inspection covers `datamasking/kms/provider.go`, `context.go`, the installed pinned TypeScript provider and Go v0.4.0 generated client/input types. [AWS caching guidance](https://docs.aws.amazon.com/encryption-sdk/latest/developer-guide/data-key-caching.html) identifies hierarchical keyrings as an alternative and states their ciphertext must be decrypted with a hierarchical keyring. Do not silently replace the existing KMS multi-keyring.

## Proposed feature contract

Propose opt-in materials caching within the optional module, retaining uncached operation as the default. Do not cache plaintext application data, final ciphertext or arbitrary SDK request results. Before implementation, select and review an upstream-compatible CMM or a narrowly scoped reviewed CMM adapter; the absence of a convenience constructor does not justify hand-writing AES, key derivation, signatures or message formats.

| Setting | Proposed behavior |
| --- | --- |
| Capacity | Positive, bounded entry count; reject invalid values; deterministic eviction policy |
| Maximum age | Positive duration; expire at the configured boundary, without extending lifetime on a hit; monotonic local timing, including time spent idle/frozen |
| Maximum messages | Positive bounded integer; atomically reserve each attempted encryption before returning materials |
| Maximum bytes | Positive bounded integer; count the actual UTF-8 plaintext bytes sent to the SDK, with overflow-safe accounting |
| Unknown message size | Bypass caching; never waive the byte threshold |
| Disabled configuration | Use existing uncached path; no new network request merely to construct a cache |

Require explicit finite limits for opt-in use rather than automatically copying the reference's very large maxima. Specify and test inclusive limits against [AWS security thresholds](https://docs.aws.amazon.com/encryption-sdk/latest/developer-guide/thresholds.html): age at the deadline is expired; an encryption reservation may reach a configured message/byte maximum but may not exceed it. Failed attempts conservatively retain reservations. Do not recycle a failed operation's allowance across concurrent callers.

## Isolation, ownership and invalidation

Keep cache ownership per provider by default. Its immutable partition identifies ordered generator/additional key configuration, client/credential ownership and algorithm/commitment policy. Encryption requests additionally match the full canonical encryption context, including empty versus absent values. Decryption matches authenticated context and encrypted data keys as well. [AWS cache matching](https://docs.aws.amazon.com/encryption-sdk/latest/developer-guide/data-caching-details.html) specifies algorithm, context, partition and decryption encrypted-key boundaries.

Do not share entries across credentials, regions, key configurations or unrelated providers by accident. Snapshot caller maps and returned materials; callers cannot mutate internal keys or counters. Cache locks protect lookup/reservation/eviction only; no lock is held during KMS, callbacks or encryption. A miss result can be published only if its partition/generation remains current. Bound concurrent misses; a cancelled waiter must not cancel another invocation's independent request.

Provide explicit invalidation/disposal semantics before implementation. Invalidation increments a generation and prevents late miss results from re-entering the cleared cache. In-flight operations may already own materials; clearing a cache cannot retract them. Reject new cached operations after disposal, remove references to secret material, and document Go's limitations on guaranteed memory erasure. Never retain a request context or the current context-bound client supplier in a reusable cache entry; preserve `context.go`'s cancellation/signing/retry bridge on each miss.

Cached keys can outlive a KMS permission/key-state change until expiry or explicit invalidation. A cache hit does not recheck authorization with KMS. Applications requiring immediate reauthorization must stay uncached. Do not cache KMS errors, cancelled results or partially authenticated decryption materials. Keep strict key commitment, the existing algorithm and post-decrypt expected-context verification.

## Acceptance before implementation

- Differential Node v5.0.2 fixtures: matching/mismatching contexts, reordered context keys, generator/additional key changes, independent partitions, encrypt/decrypt separation and KMS call counts. Pin TypeScript v2.35.0; bypass live KMS with the existing local fixture transport.
- Boundary cases: capacity one and eviction, just before/at/after expiry, warm-idle elapsed time, message/byte limits reached exactly and exceeded, multibyte UTF-8, empty data, large counters/overflow and unknown size. Use a controllable clock for tests, not sleeps.
- Concurrent acceptance: final available reservation, overlapping misses, cancellation of one waiter, callback reentry, invalidation during a miss, late completion after disposal and fresh mutable result ownership. With CGO disabled, verify invariants through synchronization and exact call/counter assertions.
- Interoperability: Node-encrypted messages decrypt in Go and Go-encrypted messages decrypt in Node with caching enabled/disabled. Ciphertexts need not be byte-identical; verify authentication, commitment, algorithm and context. Keep hierarchical ciphertext acceptance separate if that alternative is proposed later.
- Failure paths: KMS denied/throttled/timeout/disabled key, failed encryption, malformed/tampered ciphertext, wrong expected context, cancellation and retries cannot bypass limits or populate a bad cache entry.
- Run the KMS Linux-only packaged checks and affected integration checks with `GOWORK=off`, `CGO_ENABLED=0`; independent consumers and Linux amd64/arm64 builds. Existing 39 forward ciphertext cases, five rejection cases and 108 reverse assertions remain regression coverage, not evidence of caching.

## Decision

Design acceptance is complete. Keep production uncached until the CMM choice, opt-in API and limit tests are reviewed in an implementation issue. Hierarchical keyrings require a separate feature scope because they alter persistence and decryption compatibility. No dependency upgrade, AWS calls or broader encryption parity claim is approved by this planning issue; see [the remaining masking gates](DATAMASKING_PLAN.md).
