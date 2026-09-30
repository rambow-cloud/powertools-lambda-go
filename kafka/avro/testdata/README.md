# Avro differential scenarios

Generate with `node generate-kafka-avro.mjs` in tools/reference. The 370 cases use the actual CommonJS Kafka v2.35.0 consumer and avro-js 1.12.1. Payloads are produced by that Avro library, except explicit malformed bytes and mathematical zigzag boundary inputs.

Tests compare complete decoded values and error names/messages without removing schema or payload text. Buffers map explicitly to Go byte slices through a tagged numeric byte array; negative zero, infinity and NaN have tags. Union branch wrappers remain part of the compared values. Logical annotations keep the reference's unconverted underlying values.

Coverage includes every primitive, named/fixed/enum/record/recursive schema, unions, maps, arrays, negative block counts, Base64 permissiveness, malformed UTF-8, truncation, trailing data and 160 additional signed-integer boundary cases. The latter retain the reference's floating-point zigzag behavior, including loss of the negative sign at certain unsafe boundaries. Native tests cover lazy schema failure, null bypass, 64 concurrent decodes, result isolation and cancellation.

These cases do not prove every invalid-schema diagnostic, arbitrary oversized/overlong wire value, file-based schema loading, JavaScript prototype behavior, native type mapping, service delivery or performance budget. Go rejects nesting deeper than 1024 levels. Do not treat a passing scoped corpus as complete Kafka parity.
