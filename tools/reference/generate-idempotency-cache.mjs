// Exercise the pinned cache adapter with deterministic Redis client responses.
import { mkdirSync, writeFileSync } from 'node:fs';
import { CachePersistenceLayer } from '@aws-lambda-powertools/idempotency/cache';
import { IdempotencyConfig } from '@aws-lambda-powertools/idempotency';
import { IdempotencyRecord } from '@aws-lambda-powertools/idempotency/persistence';

const now = 1800000000250;
const originalNow = Date.now;
Date.now = () => now;
const key = 'operation#key';
const record = { idempotencyKey: key, status: 'INPROGRESS', expiryTimestamp: 1800003600, inProgressExpiryTimestamp: now + 5000, payloadHash: 'validation-hash' };
const seed = (status, lease) => JSON.stringify({ status, expiration: 1800003600, ...(lease !== undefined ? { in_progress_expiration: lease } : {}), ...(status === 'COMPLETED' ? { data: { ok: true }, validation: 'validation-hash' } : {}) });
const scenarios = [
  { name: 'fresh', operation: 'put' },
  { name: 'completed-conflict', operation: 'put', seed: seed('COMPLETED') },
  { name: 'active-conflict', operation: 'put', seed: seed('INPROGRESS', now + 5000) },
  { name: 'expired-lease', operation: 'put', seed: seed('INPROGRESS', now - 1) },
  { name: 'missing-lease', operation: 'put', seed: seed('INPROGRESS') },
  { name: 'expired-record', operation: 'put', seed: JSON.stringify({ status: 'COMPLETED', expiration: 1799999999, data: 1 }) },
  { name: 'malformed-json', operation: 'put', seed: '{broken' },
  { name: 'locked-orphan', operation: 'put', seed: seed('INPROGRESS', now - 1), locked: true },
  { name: 'unknown-status', operation: 'put', seed: seed('UNKNOWN') },
  { name: 'disappeared', operation: 'put', seed: seed('COMPLETED'), disappear: true },
  { name: 'completion-omits-validation', operation: 'update' },
  { name: 'read-completed', operation: 'get', seed: seed('COMPLETED') },
  { name: 'read-missing', operation: 'get' },
  { name: 'read-malformed', operation: 'get', seed: '{broken' },
  { name: 'delete', operation: 'delete', seed: seed('COMPLETED') },
  { name: 'custom-attributes', operation: 'put', custom: true },
];

for (const scenario of scenarios) {
  const values = new Map();
  if (scenario.seed) values.set(key, scenario.seed);
  if (scenario.locked) values.set(key + ':lock', 'true');
  const calls = [];
  const client = {
    async get(key) { calls.push({ operation: 'get', key }); if (scenario.disappear) values.delete(key); return values.get(key) ?? null; },
    async set(key, value, options) {
      calls.push({ operation: 'set', key, value: JSON.parse(value), ttl: options.EX, nx: !!options.NX });
      if (options.NX && values.has(key)) return null;
      values.set(key, value); return 'OK';
    },
    async del(keys) { for (const key of keys) { calls.push({ operation: 'delete', key }); values.delete(key); } },
  };
  const attributes = scenario.custom ? { statusAttr: 's', expiryAttr: 'e', inProgressExpiryAttr: 'ip', dataAttr: 'd', validationKeyAttr: 'v' } : {};
  const store = new CachePersistenceLayer({ client, ...attributes });
  store.configure({ config: new IdempotencyConfig({ payloadValidationJmesPath: 'amount' }) });
  scenario.record = { id: key, status: record.status, expiration: record.expiryTimestamp, in_progress_expiration: record.inProgressExpiryTimestamp, validation: record.payloadHash };
  try {
    if (scenario.operation === 'put') await store._putRecord(new IdempotencyRecord(record));
    if (scenario.operation === 'update') {
      scenario.record.status = 'COMPLETED'; scenario.record.data = { ok: true, nested: [null, 1.5] };
      await store._updateRecord(new IdempotencyRecord({ ...record, status: 'COMPLETED', responseData: scenario.record.data }));
    }
    if (scenario.operation === 'delete') await store._deleteRecord(new IdempotencyRecord(record));
    if (scenario.operation === 'get') {
      const value = await store._getRecord(key);
      scenario.response = { id: value.idempotencyKey, status: value.status, expiration: value.expiryTimestamp, data: value.responseData, validation: value.payloadHash };
    }
  } catch (error) { scenario.error = error.name; }
  scenario.calls = calls;
  scenario.final = Object.fromEntries([...values].map(([k, v]) => { try { return [k, JSON.parse(v)]; } catch { return [k, v]; } }));
}
Date.now = originalNow;
mkdirSync('../../idempotency/cache/testdata', { recursive: true });
writeFileSync('../../idempotency/cache/testdata/typescript-v2.35.0.json', JSON.stringify({ version: '2.35.0', now, scenarios }, null, 2) + '\n');
console.log(`${scenarios.length} cache persistence scenarios`);
