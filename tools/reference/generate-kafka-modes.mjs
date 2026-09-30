import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
const require = createRequire(import.meta.url);
const { kafkaConsumer } = require('@aws-lambda-powertools/kafka');
const avro = require('avro-js');
const protobuf = require('protobufjs');
const descriptor = require('protobufjs/ext/descriptor');

const avroSchema = JSON.stringify({ type: 'record', name: 'Payload', fields: [
  { name: 'id', type: 'string' }, { name: 'count', type: 'int' }, { name: 'raw', type: 'bytes' },
] });
const avroType = avro.parse(avroSchema);
const root = protobuf.parse('syntax="proto2"; package modes; message Payload { optional string id=1; optional int32 count=2; optional bytes raw=3; }').root;
const protoType = root.lookupType('modes.Payload');
const normalize = value => {
  if (value === undefined) return { $: 'undefined' };
  if (Buffer.isBuffer(value)) return { $: 'bytes', data: [...value] };
  if (value && value.$type) return normalize(value.toJSON());
  if (Array.isArray(value)) return value.map(normalize);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, normalize(item)]));
  return value;
};
const cases = [];
function field(kind, name, registry, format) {
  if (kind === 'text') return { data: Buffer.from(name + '-text').toString('base64') };
  const input = { id: name + '-id', count: name === 'key' ? -7 : 42, raw: 'AP+A' };
  const config = { type: registry !== 'none' && format === 'json' ? 'json' : kind };
  const metadata = registry === 'none' ? undefined : {
    dataFormat: kind.toUpperCase(),
    schemaId: registry === 'glue' ? '00000000-0000-0000-0000-000000000000' : '17',
  };
  let bytes;
  if (config.type === 'json') bytes = Buffer.from(JSON.stringify(input));
  else if (kind === 'avro') {
    config.schema = avroSchema;
    bytes = avroType.toBuffer({ ...input, raw: Buffer.from(input.raw, 'base64') });
  } else {
    config.schema = protoType;
    bytes = Buffer.from(protoType.encode(protoType.fromObject(input)).finish());
    if (registry !== 'none') bytes = Buffer.concat([Buffer.from([0]), bytes]);
  }
  return { data: bytes.toString('base64'), config, metadata };
}
async function run(source, registry, format, keyKind, valueKind, special) {
  const key = field(keyKind, 'key', registry, format);
  const value = field(valueKind, 'value', registry, format);
  const item = {
    topic: 'orders', partition: 0, offset: 17, timestamp: 1720000000123, timestampType: 'CREATE_TIME',
    key: key.data, value: value.data, keySchemaMetadata: key.metadata, valueSchemaMetadata: value.metadata,
    headers: [{ name: [-61, -87] }], custom: { retained: true },
  };
  if (special === 'null') item.key = item.value = null;
  if (special === 'empty') item.key = item.value = '';
  if (special === 'missing') { delete item.key; delete item.value; }
  const event = { eventSource: source, bootstrapServers: 'broker:9092', records: { 'orders-0': [item] } };
  if (source === 'aws:kafka') event.eventSourceArn = 'arn:aws:kafka:ap-east-1:123456789012:cluster/example/id';
  else event.eventSource = 'SelfManagedKafka';
  const calls = [], config = {}, spec = {};
  for (const [name, data] of Object.entries({ key, value })) {
    if (!data.config) continue;
    const parse = cases.length % 2 === 0;
    spec[name] = { type: data.config.type, parse };
    config[name] = { ...data.config };
    if (parse) config[name].parserSchema = { '~standard': { version: 1, vendor: 'fixture', validate(input) {
      calls.push({ field: name, value: normalize(input) });
      return { value: { parsed: input } };
    } } };
  }
  const expected = await kafkaConsumer(async output => {
    const { records, ...fields } = output;
    const r = records[0];
    const reads = [];
    for (const name of ['value', 'key', 'headers', 'key', 'value']) {
      try { reads.push({ field: name, value: normalize(r[name]), error: null }); }
      catch (e) { reads.push({ field: name, value: null, error: { name: e.name, message: e.message } }); }
    }
    return {
      fields, reads, calls,
      originalKey: normalize(r.originalKey), originalValue: normalize(r.originalValue), originalHeaders: normalize(r.originalHeaders),
      keySchemaMetadata: normalize(r.keySchemaMetadata), valueSchemaMetadata: normalize(r.valueSchemaMetadata),
      custom: normalize(r.custom),
    };
  }, config)(event, {});
  cases.push({ name: [source, registry, format, keyKind, valueKind, special || 'present'].join('-'), event, config: spec, expected });
}
for (const source of ['aws:kafka', 'SelfManagedKafka']) {
  for (const registry of ['none', 'glue', 'confluent']) {
    for (const format of registry === 'none' ? ['source'] : ['source', 'json']) {
      for (const key of ['text', 'json', 'avro', 'protobuf']) {
        for (const value of ['text', 'json', 'avro', 'protobuf']) await run(source, registry, format, key, value);
      }
    }
  }
  for (const special of ['null', 'empty', 'missing']) {
    for (const kind of ['avro', 'protobuf']) await run(source, 'confluent', 'source', kind, kind, special);
  }
}
const schema = Buffer.from(descriptor.FileDescriptorSet.encode(root.toDescriptor('proto2')).finish()).toString('base64');
mkdirSync('../../integration/kafkamodes/testdata', { recursive: true });
writeFileSync('../../integration/kafkamodes/testdata/typescript-v2.35.0.json', JSON.stringify({
  version: '2.35.0', avroSchema, protobufSchema: schema, protobufDescription: protoType, cases,
}, null, 2) + '\n');
console.log('Wrote ' + cases.length + ' mixed Kafka event-mode scenarios');
