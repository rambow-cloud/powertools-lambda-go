# Kafka event mode reference corpus

Generate from tools/reference with `node generate-kafka-modes.mjs`. The generator executes the installed CommonJS export of Powertools TypeScript v2.35.0 with avro-js 1.12.1 and protobufjs 7.5.4. No upstream implementation is patched.

The 172 constructed events cover MSK and self-managed sources, SOURCE and JSON delivery, Glue/Confluent schema metadata, missing registry metadata, every key/value pair of text/JSON/Avro/Protobuf, parser transformations, repeated lazy reads, byte headers, retained original values and metadata, unknown fields, nulls, empty strings and missing fields. A FileDescriptorSet supplies the same proto2 schema to the official Go Protobuf runtime. Successful native messages compare protojson with protobufjs JSON; Avro bytes retain explicit byte tags. Errors compare complete names and messages, including the supplied Protobuf schema description.

The Go test executes every event both through WrapHandler directly and through lambda.NewHandler(...).Invoke, preserving application context in parsing callbacks. This proves the native Go SDK handler conversion boundary; it does not simulate Kafka polling, producer framing removal, registry validation or service retries. The public aws-lambda-go v1.55.0 KafkaEvent type separately demonstrates loss of schema metadata and empty/null field presence. WrapHandler accepts json.RawMessage and retains those fields. Production modules do not gain a dependency on the Lambda SDK or either unrelated binary codec from these development-module tests.

Full JavaScript-native objects, protobufjs wire decoder errors, arbitrary schema evolution, native defaults/serialization, actual registry/service acceptance and performance budgets remain open. No live service events are claimed.
