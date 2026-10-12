// Capture actual pinned writers and read both their records and Go bridge output.
import { readFileSync, writeFileSync } from 'node:fs';
import { isDeepStrictEqual } from 'node:util';
import { IdempotencyConfig } from '@aws-lambda-powertools/idempotency';
import { CachePersistenceLayer } from '@aws-lambda-powertools/idempotency/cache';
import { DynamoDBPersistenceLayer } from '@aws-lambda-powertools/idempotency/dynamodb';
import { IdempotencyRecord } from '@aws-lambda-powertools/idempotency/persistence';
import { deepSort } from './node_modules/@aws-lambda-powertools/idempotency/lib/esm/deepSort.js';

const now = 1800000000250;
const originalNow = Date.now;
Date.now = () => now;
const config = new IdempotencyConfig({ payloadValidationJmesPath: 'amount' });
const options = custom => custom ? {
  statusAttr: 's', expiryAttr: 'e', inProgressExpiryAttr: 'ip', dataAttr: 'd', validationKeyAttr: 'v',
} : {};
const normalize = r => ({
  id: r.idempotencyKey, status: r.status, expiration: r.expiryTimestamp,
  ...(r.inProgressExpiryTimestamp !== undefined ? { in_progress_expiration: r.inProgressExpiryTimestamp } : {}),
  ...(r.responseData !== undefined ? { data: r.responseData } : {}),
  ...(r.payloadHash !== undefined ? { validation: r.payloadHash } : {}),
});

async function readCache(raw, custom) {
  const store = new CachePersistenceLayer({ client: { get: async () => raw }, ...options(custom) });
  store.configure({ config });
  return normalize(await store._getRecord('audit#key'));
}
async function readDynamoDB(item, custom) {
  const store = new DynamoDBPersistenceLayer({ tableName: 'records', ...options(custom) });
  store.configure({ config });
  store.client.send = async () => ({ Item: item });
  return normalize(await store._getRecord('audit#key'));
}

try {
  if (process.argv[2] === 'verify') {
    const directory = process.argv[3];
    if (!directory) throw new Error('Expected a Go bridge output directory');
    let assertions = 0;
    for (const backend of ['cache', 'dynamodb']) {
      const rows = JSON.parse(readFileSync(`${directory}/${backend}-go.json`, 'utf8'));
      if (rows.length !== 16) throw new Error(`Unexpected ${backend} bridge row count`);
      for (const row of rows) {
        const actual = backend === 'cache' ? await readCache(row.value, row.custom) : await readDynamoDB(row.item, row.custom);
        if (!isDeepStrictEqual(actual, row.expected)) throw new Error(`${backend} ${row.name}: ${JSON.stringify(actual)}`);
        assertions++;
      }
    }
    console.log(`${assertions} actual Go-to-TypeScript persistence reads passed`);
  } else {
    const keyStore = new CachePersistenceLayer({ client: {} });
    keyStore.configure({ config: new IdempotencyConfig({}), keyPrefix: 'operation' });
    const keys = [
      '{"4294967294":1,"4294967295":2,"0":3,"00":4}',
      '{"b":[],"a":{},"C":[null,false,0,""]}',
      '{"Ω":1,"é":2,"中":3,"😀":4}',
      '[5e-324,2.2250738585072014e-308,1.7976931348623157e308]',
      '[0.000001,0.0000009999999999999999,999999999999999900000,1e21]',
      '{"text":"\\u0000\\b\\f\\r\\n\\t\\u2028\\u2029<>&"}',
    ].map(raw => ({ raw, canonical: JSON.stringify(deepSort(JSON.parse(raw))), key: keyStore.getHashedIdempotencyKey(JSON.parse(raw)) }));
    const rows = [];
    for (const custom of [false, true]) {
      for (const [name, data] of [['null', null], ['false', false], ['zero', 0], ['string', ''], ['array', []], ['object', { nested: [null, 1.5, true], large: 9007199254740991 }], ['live', undefined], ['no-lease', undefined]]) {
        const r = new IdempotencyRecord({ idempotencyKey: 'audit#key', status: data === undefined ? 'INPROGRESS' : 'COMPLETED', expiryTimestamp: 1800003600, payloadHash: 'validation-hash', ...(name === 'live' ? { inProgressExpiryTimestamp: now + 5000 } : {}), ...(data !== undefined ? { responseData: data } : {}) });
        let cache;
        const cacheStore = new CachePersistenceLayer({ client: { set: async (_key, value) => { cache = value; return 'OK'; } }, ...options(custom) });
        cacheStore.configure({ config });
        const ddbStore = new DynamoDBPersistenceLayer({ tableName: 'records', ...options(custom) });
        ddbStore.configure({ config });
        let command;
        ddbStore.client.send = async c => { command = c.input; return {}; };
        let item;
        if (data === undefined) {
          await cacheStore._putRecord(r);
          await ddbStore._putRecord(r);
          item = command.Item;
        } else {
          await cacheStore._updateRecord(r);
          await ddbStore._updateRecord(r);
          item = { ...command.Key };
          for (const [alias, attribute] of Object.entries(command.ExpressionAttributeNames)) {
            const value = command.ExpressionAttributeValues[alias.replace('#', ':')];
            if (value === undefined) throw new Error(`Missing expression value ${alias}`);
            item[attribute] = value;
          }
        }
        rows.push({ name, custom, record: normalize(r), cache, item, cacheRead: await readCache(cache, custom), dynamodbRead: await readDynamoDB(item, custom) });
      }
    }
    // Preserve the upstream equality behavior; Go deliberately protects the
    // still-live lease to match its Manager and DynamoDB acquisition contract.
    let value = JSON.stringify({ status: 'INPROGRESS', expiration: 1800003600, in_progress_expiration: now });
    const equalSeed = value;
    const equalStore = new CachePersistenceLayer({ client: {
      get: async () => value,
      set: async (key, replacement, flags) => {
        if (key.endsWith(':lock')) return 'OK';
        if (flags.NX) return null;
        value = replacement; return 'OK';
      },
    } });
    equalStore.configure({ config });
    await equalStore._putRecord(new IdempotencyRecord({ idempotencyKey: 'audit#key', status: 'INPROGRESS', expiryTimestamp: 1800003600, inProgressExpiryTimestamp: now + 5000 }));
    writeFileSync(new URL('../../idempotency/testdata/boundaries-v2.35.0.json', import.meta.url), JSON.stringify({ version: '2.35.0', now, keys, rows, equalLease: { seed: equalSeed, referenceFinal: value } }, null, 2) + '\n');
    writeFileSync(new URL('../../idempotency/cache/testdata/boundaries-v2.35.0.json', import.meta.url), JSON.stringify({ version: '2.35.0', now, rows: rows.map(({ item, dynamodbRead, ...row }) => row), equalLease: { seed: equalSeed, referenceFinal: value } }, null, 2) + '\n');
    console.log(`${keys.length} canonical cases, ${rows.length} bidirectional persistence rows, one equality boundary`);
  }
} finally {
  Date.now = originalNow;
}
