// Use the actual TypeScript persistence implementation against the Docker Valkey
// service. docker exec avoids publishing a database port on the host.
import { execFileSync } from 'node:child_process';
import { isDeepStrictEqual } from 'node:util';
import { CachePersistenceLayer } from '@aws-lambda-powertools/idempotency/cache';
import { IdempotencyConfig, makeIdempotent } from '@aws-lambda-powertools/idempotency';

const [mode, container] = process.argv.slice(2);
if (!['seed', 'verify'].includes(mode) || !container?.startsWith('ptgo-local-')) throw new Error('Expected a disposable test container');
const command = (...args) => execFileSync('docker', ['exec', container, 'valkey-cli', '--raw', ...args.map(String)], { encoding: 'utf8', windowsHide: true }).replace(/\r?\n$/, '');
const client = {
  async get(key) { const value = command('GET', key); return value === '' ? null : value; },
  async set(key, value, options) {
    const result = command('SET', key, value, 'EX', options.EX, ...(options.NX ? ['NX'] : []));
    if (result === 'OK') return result;
    if (result === '') return null;
    throw new Error(result);
  },
  async del(keys) { return Number(command('DEL', ...keys)); },
};
const config = new IdempotencyConfig({ eventKeyJmesPath: 'id', lambdaContext: { getRemainingTimeInMillis: () => 5000 } });
const store = new CachePersistenceLayer({ client });
store.configure({ config, keyPrefix: 'cache-interop' });
if (mode === 'seed') {
  const event = { id: 'typescript' };
  await store.saveInProgress(event, 5000);
  await store.saveSuccess(event, { owner: 'typescript', nested: [1, null, true] });
  console.log(JSON.stringify({ seeded: true }));
} else {
  let calls = 0;
  const handler = makeIdempotent(async () => { calls++; throw new Error('Go record was not replayed'); }, { persistenceStore: store, config, keyPrefix: 'cache-interop' });
  const result = await handler({ id: 'go' });
  const passed = calls === 0 && isDeepStrictEqual(result, { nested: [1, null, true], owner: 'go' });
  if (!passed) throw new Error(`Unexpected stored response: ${JSON.stringify(result)}`);
  console.log(JSON.stringify({ passed, calls, response: result }));
}
