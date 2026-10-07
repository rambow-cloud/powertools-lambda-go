# Additional Kafka wire modes

Planning acceptance for [issue #117](https://github.com/rambow-cloud/powertools-lambda-go/issues/117), reviewed on 2026-10-08. This design changes no API or supported service claim. The behavioral baseline remains Powertools TypeScript v2.35.0, commit `7bcc27b1574493f9452688673658f52b80c53847`.

## Delivery and decoding matrix

Lambda delivery and producer framing are different boundaries. AWS describes registry validation for Glue and Confluent, selected KEY/VALUE attributes, and Base64 delivery in [JSON and SOURCE modes](https://docs.aws.amazon.com/lambda/latest/dg/services-consume-kafka-events.html#services-consume-kafka-events-payload-format). Registry validation requires the supported event source mapping configuration; the consumer does not configure it.

| Input boundary | Existing Go behavior | Remaining acceptance |
| --- | --- | --- |
| MSK or self-managed event without registry, text/JSON | Core lazy text/JSON decoding, independent key/value options | Actual service delivery/retry evidence |
| Registry JSON, selected attributes | Configure `kafka.JSON`; metadata can still say AVRO/PROTOBUF | Service-captured converted values and numeric/name semantics |
| Registry JSON, unselected attributes | Remain in their producer representation; choose the appropriate explicit decoder | Mixed validated/unvalidated captures |
| Registry SOURCE Avro datum | Optional Avro adapter with an application-supplied schema | Live producer framing removal and schema evolution |
| Registry SOURCE Protobuf | Optional adapter reproduces the pinned reference's metadata-dependent prefix behavior | Compare actual delivered bytes with that behavior before claiming every SOURCE configuration |
| Raw Confluent version-0 Avro/Protobuf/JSON Schema frame without ESM conversion | No general magic-byte/schema-ID envelope parser | Explicit raw-frame adapter; supplied schema resolver |
| Raw Glue framed/compressed record without ESM conversion | No complete Glue header/compression parser | Version/status/UUID/compression validation and decoded-size bounds |
| Confluent schema GUID in Kafka headers | Headers are retained, but no GUID resolver or wire-mode selection | Separate producer-version fixtures and schema identity rules |

JSON conversion must be selected by deployment configuration, not inferred from `dataFormat`. AWS SOURCE delivery removes producer metadata, so do not strip a raw producer envelope a second time. Existing constructed fixtures are evidence of consumer behavior, not evidence of AWS transformation.

## Concrete gaps and proposed priority

[Confluent's wire specification](https://docs.confluent.io/platform/current/schema-registry/fundamentals/serdes-develop/index.html#wire-format) defines a version byte, four-byte big-endian schema ID and, for Protobuf, a variable-length zigzag message-index array with an optimized zero case. `kafka/protobuf.Decoder` currently reads a count and skips that many **bytes**, with an adaptive int32/sint32 preference. It does not resolve schema IDs or select nested message descriptors from a decoded index path. A multi-byte index is a concrete missing case. The long-schema-ID Glue branch reads one uint32; it is not a general Glue envelope parser. Avro ignores metadata and expects a single datum.

Prioritize an explicit Confluent version-0 raw-frame adapter if implementation is requested. It should parse framing once, resolve a supplied schema identity and reuse the existing Avro traversal or native Protobuf descriptor decoder. Keep the current adapters and defaults unchanged. Glue framing/compression and newer GUID-header modes are separate additions; the latter are documented upstream but have no pinned v2.35.0 parity claim.

## Ownership and contracts

- Put framing and optional dependencies outside the dependency-free Kafka core. Do not add a broker client, registry credentials, network lookup or a mandatory SDK dependency.
- Let the application supply an immutable schema/descriptor catalog or a context-aware resolver. Registry URL, authentication, transport, retries and caching remain application-owned. A schema ID alone is not globally unique: include registry namespace and format in any resolver cache key.
- Select the raw-frame mode explicitly. Do not guess Glue versus Confluent from schema-ID length, reinterpret malformed data as another format, or use mutable global preference for the new parser.
- Resolve only when `Key(ctx)` or `Value(ctx)` is read. Preserve independent key/value configuration, repeated lazy reads, originals, metadata, null/empty/missing distinctions and parser timing. Each successful native Protobuf result must be fresh.
- Return typed deserialization failures with the underlying cause; unknown schemas fail before payload decoding. Cancellation propagates unchanged. Never include registry credentials or complete sensitive payloads in new diagnostics.
- Bound header/index counts, varint length, payload/decompressed size and nesting before allocation. Resolver callbacks must support concurrency; do not hold framing locks while calling them.

## Acceptance cases before implementation

| Proposed case group | Required result and reference |
| --- | --- |
| Valid raw Confluent frames | Avro datum; JSON Schema payload; Protobuf zero, nested and multi-byte index paths; exact schema ID and descriptor selection, using the producer wire specification |
| Malformed framing | Empty/truncated header, unknown version, incomplete/overflowing varints, negative or excessive index counts, out-of-range paths and trailing bytes fail deterministically |
| Unknown/wrong schema | Resolver miss/error/cancellation and incompatible format never call the payload parser; no fallback to an unrelated descriptor |
| Mixed delivery | Both event sources, KEY/VALUE/both selection, JSON beside SOURCE/text, no-registry events; never decode converted JSON as Avro |
| Lazy lifecycle | Zero resolver calls before access or for absent/null fields; independent repeated reads and failures; cancellation and overlapping invocations preserve context |
| Isolation | Same ID in different registries, key/value catalogs, concurrent resolver calls and retained-input mutation policy cannot cross-contaminate results |
| Glue follow-up | Pinned producer version, full header/compression fixtures, bad status/version/UUID, truncation and compressed-size limits |
| GUID follow-up | Pinned producer version, header/payload precedence, absent/invalid GUID, schema references and Protobuf index paths |

Use the existing pinned generators and corpora for unchanged behavior: `tools/reference/generate-kafka*.mjs`, `kafka/testdata`, both adapter `testdata` directories and `integration/kafkamodes/testdata`. Their recorded counts are 165 core, 370 Avro, 220 prefix, nine native Protobuf and 172 mixed-event cases. New wire behavior needs independently generated producer fixtures; the old consumer's output cannot prove a new framing contract.

Run focused packaged checks with `GOWORK=off`, `CGO_ENABLED=0`, independent consumers and local SDK integration. Build Linux amd64/arm64 on `provided.al2023`. Live registry, ESM transformation and retry acceptance require a separately authorized cloud task with explicit account/profile; no cloud calls are part of this planning issue.

## Decision

The planning matrix and implementation boundary are complete. Additional raw-frame support remains proposed, with no API/dependency approval inferred from closing this issue. Existing KAF-MODES, binary parity and service gates remain open in [the Kafka plan](KAFKA_PLAN.md).
