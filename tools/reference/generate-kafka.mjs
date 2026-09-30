import { createRequire } from 'node:module';
// Use the published CommonJS export: the ESM Protobuf loader imports a named
// BufferReader export that protobufjs 7.5.4 does not expose to Node's ESM loader.
const { kafkaConsumer } = createRequire(import.meta.url)('@aws-lambda-powertools/kafka');
import { writeFileSync, mkdirSync } from 'node:fs';
const normalize = (v) => {
  if (v === undefined) return { $: 'undefined' };
  if (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0))) return { $: Object.is(v, -0) ? '-0' : String(v) };
  if (Array.isArray(v)) return v.map(normalize);
  if (v && typeof v === 'object') return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, normalize(x)]));
  return v;
};
const errorValue = (e) => ({ name: e.name, message: e.message, ...(e.cause !== undefined ? { cause: normalize(e.cause) } : {}) });
const cases = [];
async function run(name, event, config, access = ['key', 'value', 'headers', 'value']) {
  const logs = [], calls = [], records = [];
  const options = config === null ? undefined : Object.fromEntries(Object.entries(config).map(([key, value]) => {
    const { parser, ...field } = value;
    if (parser) field.parserSchema = { '~standard': { version: 1, vendor: 'fixture', validate(input) {
      calls.push({ field: key, value: normalize(input) });
      if (parser === 'fail') return { issues: [{ message: 'invalid field' }] };
      if (parser === 'emptyIssues') return { issues: [] };
      if (parser === 'throw') throw new RangeError('parser failed');
      if (parser === 'transform') return { value: { parsed: input } };
      return { value: input };
    } } };
    return [key, field];
  }));
  let fields = null, error = null, entered = false;
  const original = console.error;
  console.error = (message) => logs.push(message);
  const raw = typeof event === 'string' ? event : JSON.stringify(event);
  try {
    await kafkaConsumer(async (output) => {
      entered = true;
      const { records: items, ...rest } = output;
      fields = normalize(rest);
      for (const record of items) {
        const { key, value, headers, ...descriptors } = Object.getOwnPropertyDescriptors(record);
        const metadata = Object.fromEntries(Object.entries(descriptors).map(([key, d]) => [key, normalize(d.value)]));
        const reads = [];
        for (const field of access) {
          try { reads.push({ field, value: normalize(record[field]), error: null }); }
          catch (error) { reads.push({ field, value: null, error: errorValue(error) }); }
        }
        records.push({ metadata, reads });
      }
    }, options)(JSON.parse(raw), { requestId: name });
  } catch (e) { error = errorValue(e); }
  finally { console.error = original; }
  cases.push({ name, event: raw, config, access, expected: { entered, fields, records, calls, logs, error } });
}
const b64 = (s) => Buffer.from(s).toString('base64');
const record = (value, extra = {}) => ({ topic: 'orders', partition: 1, offset: 10, timestamp: 123, timestampType: 'CREATE_TIME', headers: [], value, ...extra });
const event = (r) => ({ eventSource: 'aws:kafka', eventSourceArn: 'arn:test', bootstrapServers: 'a,b', records: { 'orders-1': [r] }, extra: { retained: true } });
for (const text of ['', 'hello', 'é😀', '\uFEFFhello', '\uFEFF\uFEFFhello', 'null', 'true', 'false', '0', '-0', '1e309', '-1e309', '9007199254740993', '1.5', '[1,null,"x"]', '{"a":1,"__proto__":{"safe":true}}', '{bad', 'undefined', '"text"', '1 2', '\u20281', ' 42 ', '{"n":-0}']) {
  for (const type of [null, 'json']) await run('text-' + cases.length, event(record(b64(text), { key: b64(text) })), type ? { key: { type }, value: { type } } : null);
}
for (const bytes of [[0xff], [0xe2, 0x82], [0xed, 0xa0, 0x80], [0xf0, 0x9f, 0x98, 0x80], [0xef, 0xbb, 0xbf], [0xc0, 0xaf], [0xe2, 0x28, 0xa1]]) {
  await run('bytes-' + cases.length, event(record(Buffer.from(bytes).toString('base64'), { headers: [{ bytes }] })), { value: { type: 'json' } });
}
for (const value of ['', 'a', 'YQ', 'YQ=', 'YQ==', 'YQ===', '====', 'AA=A', 'SGVs bG8=', '____', '-w==', '!!!!', 'AA==', 'AB==', '😀', 'éé', 'abcd😀', '你好', 'éééé', 42, false, {}, []]) {
  await run('base64-' + cases.length, event(record(value, { key: value })), { key: { type: 'json' }, value: { type: 'json' } });
}
for (const value of [undefined, null, '', b64('null'), b64('')]) for (const parser of ['transform', 'fail', 'emptyIssues', 'throw']) {
  await run('parser-' + cases.length, event(record(value, { key: value })), { key: { type: 'json', parser }, value: { type: 'json', parser } });
}
for (const headers of [null, undefined, [], [{ one: [239,187,191,65], two: [65,66] }], [{ h: [-1,256,65.9,null,true,'66','bad'] }], [{ h: 'text' }], [{ a: [] }, { a: [65] }], [null], [{}], [42], {}]) {
  await run('headers-' + cases.length, event(record(b64('x'), { headers })), null);
}
for (const type of ['bad', 'JSON', '', 'avro', 'protobuf']) {
  for (const access of [[], ['key', 'value']]) {
    await run('config-' + cases.length, event(record(b64('x'), { key: b64('x') })), { value: { type }, key: { type } }, access);
    await run('empty-' + cases.length, { records: {} }, { value: { type } }, access);
  }
}
for (const invalid of [null, [], {}, { records: null }, { records: [] }, { records: { x: null } }, { records: { x: {} } }, { records: { x: [], y: null } }, { records: { x: [null] } }]) await run('invalid-' + cases.length, invalid, null);
await run('self-managed', { eventSource: 'SelfManagedKafka', records: { x: [record(b64('x'))] } }, null);
await run('source-optional', { records: { x: [record(b64('x'))] } }, null);
await run('no-read-invalid-json', event(record(b64('{bad'))), { value: { type: 'json' } }, []);
await run('no-read-invalid-base64', event(record('!')), null, []);
await run('original-overwrite', event(record(b64('x'), { originalKey: 'discard', originalValue: 'discard', originalHeaders: 'discard', keySchemaMetadata: { schemaId: '1' }, valueSchemaMetadata: { dataFormat: 'JSON' } })), null);
await run('primitive-records', { records: { x: [42, false, 'abc', []] } }, null);
await run('topic-order', '{"records":{"z":[{"value":"eg==","headers":[]}],"10":[{"value":"MTA=","headers":[]}],"a":[{"value":"YQ==","headers":[]}],"2":[{"value":"Mg==","headers":[]}],"z":[{"value":"bGFzdA==","headers":[]}]}}', null);
// EventRecordFormat belongs to the event source mapping, not the delivered event.
// JSON-mode metadata still identifies the original schema format. Only selected
// attributes have been converted; the consumer must honor explicit field configs.
for (const source of ['aws:kafka', 'SelfManagedKafka']) {
  for (const dataFormat of ['AVRO', 'PROTOBUF', 'JSON']) {
    for (const selected of [['key'], ['value'], ['key', 'value']]) {
      const item = record(b64('plain-value'), { key: b64('plain-key') });
      const config = {};
      for (const field of selected) {
        item[field] = b64(JSON.stringify({ order_id: 'order-1', field }));
        item[field + 'SchemaMetadata'] = { dataFormat, schemaId: '17' };
        config[field] = { type: 'json' };
      }
      await run(`mode-json-${source}-${dataFormat}-${selected.join('-')}`,
        { ...event(item), eventSource: source }, config);
    }
  }
  await run(`mode-no-registry-${source}`,
    { ...event(record(b64('{"order_id":"order-1"}'), { key: b64('plain-key') })), eventSource: source },
    { value: { type: 'json' } });
}
for (const dataFormat of ['AVRO', 'PROTOBUF']) {
  const item = record(b64('{"order_id":"order-1"}'), {
    valueSchemaMetadata: { dataFormat, schemaId: '17' },
  });
  await run(`mode-no-inference-${dataFormat}`, event(item), null);
}
mkdirSync('../../kafka/testdata', { recursive: true });
writeFileSync('../../kafka/testdata/typescript-v2.35.0.json', JSON.stringify({ version: '2.35.0', cases }, null, 2) + '\n');
console.log('Wrote ' + cases.length + ' Kafka core scenarios');
