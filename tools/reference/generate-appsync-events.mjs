import { AppSyncEventsResolver, UnauthorizedException } from '@aws-lambda-powertools/event-handler/appsync-events';
import { createHash } from 'node:crypto';
import { writeFileSync } from 'node:fs';

const base = (path = '/default/foo', operation = 'PUBLISH') => ({ identity: null, result: null, request: { headers: {}, domainName: null }, error: null, prev: null, stash: {}, outErrors: [], events: operation === 'PUBLISH' ? [{ id: 'one', payload: { value: 1 } }, { id: 'two', payload: null }] : null, info: { channel: { path, segments: path.slice(1).split('/') }, channelNamespace: { name: 'default' }, operation } });
const normalize = (value) => {
  if (value === undefined) return null;
  if (typeof value === 'string' && value.length > 1000) return { utf8Bytes: Buffer.byteLength(value), sha256: createHash('sha256').update(value).digest('hex') };
  if (Array.isArray(value)) return value.map(normalize);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).filter(([, v]) => v !== undefined).map(([k, v]) => [k, normalize(v)]));
  return value;
};
const cases = [];
async function run(steps, warn = false) {
  const logs = [], results = [], calls = [];
  const app = new AppSyncEventsResolver({ warnOnLargePayload: warn, logger: { debug(message) { logs.push(['debug', message]); }, warn(message) { logs.push(['warn', message]); }, error(message, error) { logs.push(['error', message, error instanceof Error ? `${error.name} - ${error.message}` : 'An unknown error occurred']); } } });
  for (const step of steps) {
    if (step.type === 'register') {
      const handler = async (payload, event, context) => {
        if (step.kind === 'subscribe') { context = event; event = payload; payload = undefined; }
        calls.push({ tag: step.tag ?? '', payload: normalize(payload), path: event.info.channel.path, context: context.requestId });
        if (step.mode === 'error') throw new TypeError('handler failed');
        if (step.mode === 'unauthorized') throw new UnauthorizedException('denied');
        if (step.mode === 'unknown') throw 42;
        if (step.mode === 'undefined') return undefined;
        if (step.mode === 'null') return null;
        if (step.mode === 'large') return step.character.repeat(step.length);
        if (step.mode === 'largeAggregate') return [{ id: 'one', payload: step.character.repeat(step.length) }];
        if (step.mode === 'tag') return { tag: step.tag, value: payload };
        return payload;
      };
      if (step.kind === 'subscribe') app.onSubscribe(step.path, handler);
      else app.onPublish(step.path, handler, { aggregate: step.aggregate ?? false });
    } else {
      try { results.push({ value: normalize(await app.resolve(step.event, { requestId: 'request' })), error: null }); }
      catch (error) { results.push({ value: null, error: `${error.name} - ${error.message}` }); }
    }
  }
  cases.push({ steps, warn, results, calls, logs });
}
const register = (path, mode = 'tag', tag = path, kind = 'publish', aggregate = false) => ({ type: 'register', path, mode, tag, kind, aggregate });
const resolve = (event) => ({ type: 'resolve', event });
const paths = ['/*', '/default/*', '/default/foo', '/default/foo/*', '/default/f.o', '/default/a+b', '/default/😀', '/default/ ', '', '/', 'default/foo', '/default/', '/default//foo', '/default/f*', '/default/*/foo', '/default/*\n', '/default/foo\n'];
const queries = ['/default/foo', '/default/foo/bar', '/default/', '/other/foo', '/default/f.o', '/default/a+b', '/default/😀', '/default/ ', '/default/foo\n', '/default/foo\r\n', '/default/foo\u2028'];
for (const path of paths) for (const operation of ['PUBLISH', 'SUBSCRIBE']) {
  await run([register(path, 'tag', path, operation === 'PUBLISH' ? 'publish' : 'subscribe'), ...queries.map((q) => resolve(base(q, operation))), resolve(base('/missing', operation)), resolve(base('/missing', operation))]);
}
for (const mode of ['echo', 'tag', 'error', 'unauthorized', 'unknown', 'undefined', 'null']) for (const aggregate of [false, true]) {
  await run([register('/default/foo', mode, 'selected', 'publish', aggregate), resolve(base()), resolve({ ...base(), events: [] })]);
}
for (const mode of ['echo', 'error', 'unauthorized', 'unknown', 'undefined', 'null']) await run([register('/default/foo', mode, '', 'subscribe'), resolve(base('/default/foo', 'SUBSCRIBE'))]);
await run([register('/*'), register('/default/*'), register('/default/foo'), resolve(base()), register('/default/foo', 'tag', 'replacement'), resolve(base()), resolve(base('/default/bar')), register('/default/bar'), resolve(base('/default/bar'))]);
await run([resolve(base()), register('/default/foo'), resolve(base())]);
const invalid = [null, [], {}, 'event', 42];
for (const key of Object.keys(base())) { const event = base(); delete event[key]; invalid.push(event); }
for (const [path, value] of [[['request'], null], [['request', 'headers'], []], [['stash'], null], [['outErrors'], {}], [['info', 'channel', 'path'], 42], [['info', 'channel', 'segments'], [1]], [['info', 'channelNamespace'], {}], [['info', 'operation'], 'OTHER']]) {
  const event = base(); let target = event; for (const key of path.slice(0, -1)) target = target[key]; target[path.at(-1)] = value; invalid.push(event);
}
for (const event of invalid) await run([register('/*'), resolve(event)]);
for (const events of [null, {}, [{ id: 'one' }], [{ id: 1, payload: {} }], [null]]) await run([register('/*', 'error'), register('/*', 'tag', '', 'subscribe'), resolve({ ...base(), events })]);
for (const aggregate of [false, true]) for (const [character, length] of [['a', 245734], ['a', 245735], ['a', 245736], ['é', 122867], ['<', 245735], ['\u2028', 81912], ['\u2029', 81912], ['\\u2028', 40000]]) {
  const step = { ...register('/default/*', aggregate ? 'largeAggregate' : 'large', '', 'publish', aggregate), character, length };
  const event = { ...base(), events: [{ id: 'one', payload: 1 }] };
  await run([step, resolve(event), resolve(event), resolve({ ...event, info: { ...event.info, channel: { path: '/default/other', segments: ['default', 'other'] } } })], true);
}
await run([{ ...register('/default/foo', 'large'), character: 'a', length: 245736 }, resolve(base())], false);
writeFileSync(new URL('../../eventhandler/appsyncevents/testdata/typescript-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', normalization: 'Undefined top-level results map to null; omitted properties remain absent. Long string values are compared by exact UTF-8 length and SHA-256. Concurrent item calls/error logs are compared as multisets; route diagnostics and output order remain exact.', cases }, null, 2) + '\n');
console.log(`${cases.length} AppSync Events reference scenarios`);
