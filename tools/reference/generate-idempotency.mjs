// Generate evidence from the installed, pinned implementation, not a reimplementation.
import { mkdirSync, writeFileSync } from 'node:fs';
import { IdempotencyConfig, makeIdempotent } from '@aws-lambda-powertools/idempotency';
import { BasePersistenceLayer, IdempotencyRecord } from '@aws-lambda-powertools/idempotency/persistence';
import { IdempotencyItemAlreadyExistsError, IdempotencyItemNotFoundError } from '@aws-lambda-powertools/idempotency';
import { DynamoDBPersistenceLayer } from '@aws-lambda-powertools/idempotency/dynamodb';
import { deepSort } from './node_modules/@aws-lambda-powertools/idempotency/lib/esm/deepSort.js';

process.env.AWS_LAMBDA_FUNCTION_NAME = 'reference';
delete process.env.POWERTOOLS_IDEMPOTENCY_DISABLED;
let now = 1800000000250;
const originalNow = Date.now;
Date.now = () => now;
const originalWarn = console.warn;
console.warn = () => {};

class Memory extends BasePersistenceLayer {
  records = new Map();
  calls = [];
  async _putRecord(record) {
    this.calls.push('put');
    const previous = this.records.get(record.idempotencyKey);
    if (previous && !previous.isExpired() && !(previous.status === 'INPROGRESS' && previous.inProgressExpiryTimestamp && previous.inProgressExpiryTimestamp < now)) {
      throw new IdempotencyItemAlreadyExistsError('conflict', previous);
    }
    this.records.set(record.idempotencyKey, record);
  }
  async _getRecord(key) { this.calls.push('get'); const result = this.records.get(key); if (!result) throw new IdempotencyItemNotFoundError(); return result; }
  async _updateRecord(record) { this.calls.push('update'); this.records.set(record.idempotencyKey, record); }
  async _deleteRecord(record) { this.calls.push('delete'); this.records.delete(record.idempotencyKey); }
}

const rawInputs = [
  '{}', 'null', 'false', '0', '""', '[]', '[null,false,0,""]', '{"a":null,"b":false}',
  '{"z":1,"B":2,"a":3}', '{"a":1,"A":2}', '{"A":2,"a":1}',
  '{"10":"ten","2":"two","01":"one","b":2,"A":1}',
  '{"nested":{"z":1,"A":2},"array":[{"b":2,"a":1},3]}',
  '{"__proto__":{"polluted":true},"constructor":3,"ok":1}',
  '"<>&\\n\\t\\u2028\\u2029😀"', '{"😀":1,"\ue000":2,"Z":3}',
  '[-0,0.0000001,0.000001,1e20,1e21,9007199254740993]',
  '{"same":1,"same":2}', '{"array":[2,1]}', '{"array":[1,2]}',
];
const keys = [];
for (const raw of rawInputs) {
  const store = new Memory(); store.configure({ config: new IdempotencyConfig({}), keyPrefix: 'operation' });
  const input = JSON.parse(raw);
  keys.push({ raw, canonical: JSON.stringify(deepSort(input)), key: store.getHashedIdempotencyKey(input) });
}
for (const hashFunction of ['md5', 'sha1', 'sha256', 'sha384', 'sha512']) {
  const raw = '{"order":{"id":"123","total":7},"ignored":true}';
  const config = { eventKeyJmesPath: 'order.id', payloadValidationJmesPath: 'order.total', hashFunction };
  const store = new Memory(); store.configure({ config: new IdempotencyConfig(config), keyPrefix: 'operation' });
  const input = JSON.parse(raw);
  keys.push({ raw, config, key: store.getHashedIdempotencyKey(input), validation: store.getHashedPayload(input) });
}

const scenarios = [
  { name: 'replay', steps: [{ event: { id: 'a' } }, { event: { id: 'a' } }] },
  { name: 'key-projection', config: { eventKeyJmesPath: 'id' }, steps: [{ event: { id: 'a', ignored: 1 } }, { event: { id: 'a', ignored: 2 } }] },
  { name: 'validation', config: { eventKeyJmesPath: 'id', payloadValidationJmesPath: 'amount' }, steps: [{ event: { id: 'a', amount: 1 } }, { event: { id: 'a', amount: 2 } }] },
  { name: 'missing-bypass', config: { eventKeyJmesPath: 'absent' }, steps: [{ event: {} }, { event: {} }] },
  { name: 'missing-strict', config: { eventKeyJmesPath: 'absent', throwOnNoIdempotencyKey: true }, steps: [{ event: {} }] },
  { name: 'false-is-key', config: { eventKeyJmesPath: 'id' }, steps: [{ event: { id: false } }, { event: { id: false } }] },
  { name: 'false-strict', config: { eventKeyJmesPath: 'id', throwOnNoIdempotencyKey: true }, steps: [{ event: { id: false } }] },
  { name: 'handler-error-retry', steps: [{ event: { id: 'a' }, fail: true }, { event: { id: 'a' } }] },
  { name: 'expired', config: { expiresAfterSeconds: 2 }, steps: [{ event: { id: 'a' } }, { event: { id: 'a' }, advance: 3000 }] },
  { name: 'local-cache', config: { useLocalCache: true }, steps: [{ event: { id: 'a' } }, { event: { id: 'a' } }] },
  { name: 'cache-expired', config: { useLocalCache: true, expiresAfterSeconds: 2 }, steps: [{ event: { id: 'a' } }, { event: { id: 'a' }, advance: 3000 }] },
  { name: 'completion-expiry', config: { expiresAfterSeconds: 2 }, steps: [{ event: { id: 'a' }, work: 1200 }, { event: { id: 'a' }, advance: 1000 }] },
  { name: 'inprogress', seed: 'live', steps: [{ event: { id: 'a' } }] },
  { name: 'orphan-recovery', seed: 'expired', steps: [{ event: { id: 'a' } }] },
];
const normalize = record => ({ id: record.idempotencyKey, status: record.status, expiration: record.expiryTimestamp, ...(record.inProgressExpiryTimestamp ? { in_progress_expiration: record.inProgressExpiryTimestamp } : {}), ...(record.responseData !== undefined ? { data: record.responseData } : {}), ...(record.payloadHash ? { validation: record.payloadHash } : {}) });
for (const scenario of scenarios) {
  now = 1800000000250;
  let calls = 0, current;
  const store = new Memory();
  const config = new IdempotencyConfig({ ...scenario.config, lambdaContext: { getRemainingTimeInMillis: () => 5000 } });
  store.configure({ config, keyPrefix: 'operation' });
  if (scenario.seed) {
    const key = store.getHashedIdempotencyKey({ id: 'a' });
    store.records.set(key, new IdempotencyRecord({ idempotencyKey: key, status: 'INPROGRESS', expiryTimestamp: 1800003600, inProgressExpiryTimestamp: now + (scenario.seed === 'live' ? 5000 : -1) }));
  }
  const handler = makeIdempotent(async event => {
    calls++;
    if (current.fail) throw new Error('business failure');
    now += current.work ?? 0;
    return { call: calls, event };
  }, { persistenceStore: store, config, keyPrefix: 'operation' });
  scenario.results = [];
  for (const step of scenario.steps) {
    current = step; now += step.advance ?? 0;
    try { scenario.results.push({ response: await handler(step.event) }); }
    catch (error) { scenario.results.push({ error: error.name, cause: error.cause?.name }); }
  }
  scenario.calls = calls;
  scenario.operations = store.calls;
  scenario.records = [...store.records.values()].map(normalize);
}

// Capture real SDK command payloads for the default and composite schemas.
const wire = [];
for (const composite of [false, true]) {
  now = 1800000000250;
  const options = composite ? { tableName: 'records', keyAttr: 'pk', sortKeyAttr: 'sk', staticPkValue: 'tenant', statusAttr: 's', expiryAttr: 'e', inProgressExpiryAttr: 'ip', dataAttr: 'd', validationKeyAttr: 'v' } : { tableName: 'records' };
  const store = new DynamoDBPersistenceLayer(options);
  store.configure({ config: new IdempotencyConfig({ eventKeyJmesPath: 'id', payloadValidationJmesPath: 'amount' }), keyPrefix: 'operation' });
  const commands = [];
  store.client.send = async command => { commands.push({ type: command.constructor.name, input: command.input }); return {}; };
  const event = { id: 'a', amount: 7 };
  await store.saveInProgress(event, 5000);
  await store.saveSuccess(event, { ok: true, amount: 1.5, nested: [null, 3] });
  await store.deleteRecord(event);
  wire.push({ composite, commands });
}
Date.now = originalNow;
console.warn = originalWarn;
mkdirSync('../../idempotency/testdata', { recursive: true });
writeFileSync('../../idempotency/testdata/typescript-v2.35.0.json', JSON.stringify({ version: '2.35.0', keys, scenarios, wire }, null, 2) + '\n');
console.log(`${keys.length} keys, ${scenarios.length} lifecycle scenarios, ${wire.length} DynamoDB command scenarios`);
