import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
const { DataMasking } = createRequire(import.meta.url)('@aws-lambda-powertools/data-masking');
const cases = [];
const normalize = value => {
  if (value === undefined) return { $: 'undefined' };
  if (Array.isArray(value)) return value.map(normalize);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, normalize(item)]));
  return value;
};
async function run(method, data, options = {}, ignoreMissing = false, provider = true) {
  const calls = [], warnings = [];
  // This deterministic provider tests orchestration only. It is not encryption.
  const mock = {
    async encrypt(data, context) {
      calls.push({ method: 'encrypt', data, context: context ?? null });
      if (data.includes('reject')) throw new Error('provider rejected');
      return 'cipher:' + data;
    },
    async decrypt(data, context) {
      calls.push({ method: 'decrypt', data, context: context ?? null });
      if (!data.startsWith('cipher:')) throw new Error('invalid test ciphertext');
      return data.slice(7);
    },
  };
  const masker = new DataMasking({ throwOnMissingField: !ignoreMissing, provider: provider ? mock : undefined });
  const original = console.warn;
  console.warn = message => warnings.push(message);
  let value = null, error = null;
  const before = JSON.stringify(data);
  try { value = normalize(await masker[method](data, options)); }
  catch (e) { error = { name: e.name, message: e.message }; }
  finally { console.warn = original; }
  if (JSON.stringify(data) !== before) throw new Error('Reference mutated input');
  const { maskingRules, ...plain } = options;
  cases.push({ name: method + '-' + cases.length, method, data: before, options: {
    ...plain, ...(maskingRules ? { rules: Object.entries(maskingRules).map(([field, rule]) => ({ field, rule })) } : {}),
  }, ignoreMissing, provider, value, error, warnings, calls });
}
const values = [null, true, false, 0, -0, 42, -1.25, 1e-7, 1e21, '', 'hello', 'é😀', [], [1, null, 'x'], {}, { z: 1, a: { secret: 'yes', nil: null }, list: [true, { token: 'x' }] }];
for (const data of values) {
  for (const options of [{}, { customMask: '' }, { customMask: '[removed]' }, { dynamicMask: true }, { dynamicMask: false }, { fields: [] }, { maskingRules: {} }]) await run('erase', data, options);
}
const nested = { z: 'last', users: [{ id: 1, secret: 'one' }, { id: 2, secret: null }], info: { secret: 'two', nested: { secret: 'three' } }, empty: [], nil: null };
for (const fields of [['users[*].secret'], ['users.*.id'], ['info.*'], ['*'], ['users.0.secret'], ['users[0].secret'], ['users.01.secret'], ['missing'], ['info.secret.x'], ['nil.x'], [''], ['.'], ['..info..secret.'], ['empty[*]'], ['users[*]secret'], ['info.secret', 'missing'], ['info.secret', 'info.secret'], ['info', 'info.secret']]) {
  for (const ignore of [false, true]) await run('erase', nested, { fields }, ignore);
}
for (const options of [
  { fields: ['users[*].secret'], dynamicMask: true },
  { fields: ['users[*].secret'], customMask: '' },
  { fields: ['info'], dynamicMask: true },
  { fields: ['users'], dynamicMask: true },
  { fields: ['info.*'], customMask: 'default', maskingRules: { 'info.secret': { customMask: 'specific' } } },
  { fields: ['users[*].secret'], maskingRules: { 'users.0.secret': { customMask: 'first' } } },
  { maskingRules: { missing: { customMask: 'ignored' } } },
  { maskingRules: { 'info.secret': {}, 'info.nested.secret': { dynamicMask: false } } },
  { maskingRules: { 'info.*': { customMask: 'first' }, 'info.secret': { dynamicMask: true } } },
  { maskingRules: { 'users[*].secret': { customMask: 'secret' } }, dynamicMask: true },
]) await run('erase', nested, options);
const reserved = JSON.parse('{"__proto__":{"secret":"a"},"constructor":{"secret":"b"},"prototype":"c","safe":"d"}');
for (const options of [{ fields: ['*'] }, { dynamicMask: true }, { fields: ['__proto__'] }, { fields: ['constructor.secret'] }, { fields: ['constructor.secret'], customMask: 'x' }]) await run('erase', reserved, options);
for (const options of [{ fields: ['length'] }, { fields: ['0'] }, { fields: ['-1'] }, { fields: ['[*]'] }]) await run('erase', [1, 2], options);
for (const data of values) {
  await run('encrypt', data);
  await run('encrypt', data, { fields: [], context: { tenant: 'example' } });
  await run('decrypt', 'cipher:' + JSON.stringify(data), { fields: ['ignored'], context: { tenant: 'example' } });
}
for (const fields of [['users[*].secret'], ['users.*.id'], ['info'], ['users'], ['missing'], ['info.secret', 'info.secret'], [''], ['nil'], ['users[*]']]) await run('encrypt', nested, { fields, context: { tenant: 'example' } });
const encrypted = { z: 'cipher:42', users: [{ secret: 'cipher:"one"' }, { secret: null }], info: { flag: false, count: 7, list: [], object: {}, nil: null } };
for (const fields of [undefined, [], ['users[*].secret'], ['info.*'], ['missing'], ['z', 'z'], ['z']]) await run('decrypt', encrypted, { fields });
for (const method of ['encrypt', 'decrypt']) for (const data of [null, {}, 'text']) await run(method, data, {}, false, false);
await run('encrypt', { secret: 'reject' }, { fields: ['secret'] });
await run('decrypt', { secret: 'not-ciphertext' }, { fields: ['secret'] });
await run('encrypt', JSON.parse('{"z":1,"10":"ten","a":2,"2":"two","z":3}'));
const timeline = [];
let release;
const pending = new Promise(resolve => { release = resolve; });
const asyncMasker = new DataMasking({ provider: { async encrypt(data) {
  timeline.push('start:' + data);
  if (data === '"reject"') throw new Error('first rejection');
  await pending;
  timeline.push('completed');
  return 'cipher:' + data;
} } });
try { await asyncMasker.encrypt({ a: 'reject', b: 'pending' }, { fields: ['*'] }); }
catch (e) { timeline.push('caught:' + e.message); }
timeline.push('release');
release();
await pending;
await Promise.resolve();
mkdirSync('../../datamasking/testdata', { recursive: true });
writeFileSync('../../datamasking/testdata/typescript-v2.35.0.json', JSON.stringify({ version: '2.35.0', cases, asyncFailure: timeline }, null, 2) + '\n');
console.log('Wrote ' + cases.length + ' Data Masking scenarios');
