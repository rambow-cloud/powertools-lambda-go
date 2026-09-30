import { mkdirSync, writeFileSync } from 'node:fs';
import { getStringFromEnv, getBooleanFromEnv, getNumberFromEnv, isDevMode, isRunningInLambda, getServiceName } from '@aws-lambda-powertools/commons/utils/env';
import { fromBase64 } from '@aws-lambda-powertools/commons/utils/base64';
import { deepMerge } from '@aws-lambda-powertools/commons/utils/deep-merge';
import { LRUCache } from '@aws-lambda-powertools/commons/utils/lru-cache';
import { unmarshallDynamoDB } from '@aws-lambda-powertools/commons/utils/unmarshallDynamoDB';
import { getMetadata, clearMetadataCache } from '@aws-lambda-powertools/commons/utils/metadata';
import { isTruthy, getType, isIntegerNumber } from '@aws-lambda-powertools/commons/typeutils';

// Normalize cross-language values explicitly without erasing error categories.
const normalize = value => {
  if (typeof value === 'bigint') return { bigint: String(value) };
  if (typeof value === 'number' && !Number.isFinite(value)) return { number: String(value) };
  if (value instanceof Set || value instanceof Uint8Array) return [...value].map(normalize);
  if (Array.isArray(value)) return value.map(normalize);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, normalize(v)]));
  return value ?? null;
};
const outcome = fn => { try { return { value: normalize(fn()) }; } catch (error) { return { error: error.name }; } };
const env = [];
const original = process.env.PT_COMMONS_FIXTURE;
try {
  for (const raw of [null, '', '  hello  ', 'true', 'TRUE', ' yes ', 'n', 't', 'off', '1', '0', 'invalid', ' 2.5 ', '0x10', '0b11', '1e2', 'Infinity']) {
    if (raw === null) delete process.env.PT_COMMONS_FIXTURE; else process.env.PT_COMMONS_FIXTURE = raw;
    for (const mode of ['string', 'number', 'boolean', 'extended']) {
      const options = { key: 'PT_COMMONS_FIXTURE' };
      const read = () => mode === 'string' ? getStringFromEnv(options) : mode === 'number' ? getNumberFromEnv(options) : getBooleanFromEnv({ ...options, extendedParsing: mode === 'extended' });
      env.push({ raw, mode, ...outcome(read) });
    }
  }
} finally { if (original === undefined) delete process.env.PT_COMMONS_FIXTURE; else process.env.PT_COMMONS_FIXTURE = original; }
const base64 = [];
for (const input of ['', 'aGVsbG8=', 'aGVsbG8', 'aGVs bG8', '8J-SqQ==', '%%%=', 'AAA=', 'AA==', 'abcd', 'abcz']) {
  for (const encoding of [null, 'base64', 'utf8', 'hex', 'utf16le']) base64.push({ input, encoding, ...outcome(() => encoding === null ? fromBase64(input) : fromBase64(input, encoding)) });
}
const source = { nested: { b: 2 }, array: [{ b: 2 }, 9], nil: null };
const merged = deepMerge({ nested: { a: 1 }, array: [{ a: 1 }, 2, 3], nil: 'old' }, source);
const cyclic = { value: 1 }; cyclic.self = cyclic;
const circular = deepMerge({}, cyclic);
const unsafe = deepMerge({}, JSON.parse('{"__proto__":{"polluted":true},"constructor":1,"safe":2}'));
const lru = new LRUCache({ maxSize: 2 });
lru.add('a', 1); lru.add('b', 2); lru.get('a'); lru.add('c', 3);
const lruResult = { size: lru.size(), a: lru.has('a'), b: lru.has('b'), c: lru.get('c') };
const numbers = ['0', '2.5', '9007199254740991', '9007199254740992', '-9007199254740993', '9007199254740992.5', '1e20', 'Infinity', 'invalid'].map(input => ({ input, ...outcome(() => unmarshallDynamoDB({ n: { N: input } }).n) }));
const types = [null, '', ' ', 0, 1, 1.5, true, false, [], [1], {}, { a: 1 }].map(value => ({ value, type: getType(value), truthy: isTruthy(value), integer: isIntegerNumber(value) }));
const rawItem = { s: { S: 'text' }, b: { B: 'raw-binary' }, ss: { SS: ['a','a','b'] }, ns: { NS: ['1','1','9007199254740992'] }, flag: { BOOL: true }, nil: { NULL: true }, list: { L: [{ N: '2' }, { M: { nested: { S: 'yes' } } }] } };
const runtime = [];
const priorEnv = { ...process.env };
const oldFetch = globalThis.fetch;
const requests = [];
let metadata;
try {
  for (const initialization of [null, '', 'unknown', 'on-demand', 'provisioned-concurrency']) {
    for (const dev of ['', 'true', 'yes', 'false']) {
      if (initialization === null) delete process.env.AWS_LAMBDA_INITIALIZATION_TYPE; else process.env.AWS_LAMBDA_INITIALIZATION_TYPE = initialization;
      process.env.POWERTOOLS_DEV = dev;
      runtime.push({ initialization, dev, local: isDevMode(), lambda: isRunningInLambda() });
    }
  }
  process.env.AWS_LAMBDA_INITIALIZATION_TYPE = 'on-demand'; process.env.POWERTOOLS_DEV = 'false';
  process.env.AWS_LAMBDA_METADATA_API = 'metadata.local:9001'; process.env.AWS_LAMBDA_METADATA_TOKEN = 'fixture-token';
  globalThis.fetch = async (url, options) => { requests.push({ url, authorization: options.headers.Authorization }); return new Response(JSON.stringify({ AvailabilityZoneID: 'ape1-az1', future: { enabled: true } }), { status: 200 }); };
  clearMetadataCache();
  const first = structuredClone(await getMetadata()); const second = structuredClone(await getMetadata());
  clearMetadataCache(); const third = structuredClone(await getMetadata());
  delete process.env.AWS_LAMBDA_INITIALIZATION_TYPE; const local = await getMetadata();
  metadata = { first, second, third, local, requests };
} finally {
  globalThis.fetch = oldFetch; clearMetadataCache();
  for (const key of ['AWS_LAMBDA_INITIALIZATION_TYPE','POWERTOOLS_DEV','AWS_LAMBDA_METADATA_API','AWS_LAMBDA_METADATA_TOKEN']) { if (priorEnv[key] === undefined) delete process.env[key]; else process.env[key] = priorEnv[key]; }
}
mkdirSync('../../commons/testdata', { recursive: true });
writeFileSync('../../commons/testdata/typescript-v2.35.0.json', JSON.stringify({ version: '2.35.0', env, base64, merged, source, circular, unsafe, lru: lruResult, numbers, types, rawItem, unmarshalled: normalize(unmarshallDynamoDB(rawItem)), runtime, metadata }, null, 2) + '\n');
for (const [directory, value] of [
  ['dynamodb', { version: '2.35.0', numbers, rawItem, unmarshalled: normalize(unmarshallDynamoDB(rawItem)) }],
  ['metadata', { version: '2.35.0', runtime, metadata }],
]) {
  mkdirSync(`../../commons/${directory}/testdata`, { recursive: true });
  writeFileSync(`../../commons/${directory}/testdata/typescript-v2.35.0.json`, JSON.stringify(value, null, 2) + '\n');
}
console.log('Generated Commons and Metadata reference fixtures without network requests');
