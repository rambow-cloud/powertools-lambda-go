import { createRequire } from 'node:module';
import { writeFileSync } from 'node:fs';
const require = createRequire(import.meta.url);
const { kafkaConsumer } = require('@aws-lambda-powertools/kafka');
const cases = [];
for (const schema of [
  '{"type":', '{"type":"missing"}', '{"type":"array"}', '{"type":"map"}',
  '{"type":"fixed","name":"Broken","size":-1}',
  '{"type":"enum","name":"Broken","symbols":["A","A"]}',
  '{"type":"record","name":"Broken","fields":[{"name":"a","type":"int"},{"name":"a","type":"int"}]}',
  '{"type":"record","name":"Broken","fields":[{"name":"a","type":"Missing"}]}',
  '["null","null"]', '[["null","string"],"int"]',
]) {
  let error = null;
  try {
    await kafkaConsumer(async event => event.records[0].value, { value: { type: 'avro', schema } })({ records: { topic: [{ value: 'AA==', headers: [] }] } }, {});
  } catch (failure) {
    error = { name: failure.name, message: failure.message };
  }
  if (!error) throw new Error(`Invalid-schema reference unexpectedly accepted ${schema}`);
  cases.push({ schema, data: 'AA==', error });
}
writeFileSync(new URL('../../kafka/avro/testdata/boundaries-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', comparison: 'Rejection and public error category; native parser cause text is retained separately.', cases }, null, 2) + '\n');
console.log(`${cases.length} actual Avro invalid-schema boundaries`);
