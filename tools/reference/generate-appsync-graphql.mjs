import { AppSyncGraphQLResolver, Router, ResolverNotFoundException, InvalidBatchResponseException, awsDate, awsTime, awsDateTime, awsTimestamp } from '@aws-lambda-powertools/event-handler/appsync-graphql';
import { writeFileSync } from 'node:fs';

const normalize = (value) => value === undefined ? null : JSON.parse(JSON.stringify(value));
const failure = (error) => error instanceof Error ? `${error.name} - ${error.message}` : 'An unknown error occurred';
const base = (field = 'hello', type = 'Query', args = { value: 1 }) => ({ arguments: args, identity: null, source: null, request: { headers: {}, domainName: null }, prev: null, info: { fieldName: field, parentTypeName: type, variables: {} }, stash: {} });
const cases = [];
async function run(steps) {
  const logs = [], calls = [], results = [];
  const logger = { debug(message) { logs.push(['debug', message]); }, warn(message) { logs.push(['warn', message]); }, error(message, error) { logs.push(['error', typeof message === 'string' ? message : '', failure(typeof message === 'string' ? error : message)]); } };
  const app = new AppSyncGraphQLResolver({ logger });
  const routers = { app, a: new Router({ logger }), b: new Router({ logger }) };
  for (const step of steps) {
    const target = routers[step.target ?? 'app'];
    if (step.op === 'register') {
      const handler = async (input, { event, context }) => {
        calls.push({ tag: step.tag ?? '', input: normalize(input), field: Array.isArray(event) ? 'batch' : event.info.fieldName, request: context.requestId });
        if (step.mode === 'error' || (step.mode === 'mixed' && input.fail)) throw new TypeError('handler failed');
        if (step.mode === 'unknown') throw 42;
        if (step.mode === 'missing') throw new ResolverNotFoundException('explicit missing');
        if (step.mode === 'invalid') throw new InvalidBatchResponseException('explicit invalid');
        if (step.mode === 'null') return null;
        if (step.mode === 'undefined') return undefined;
        if (step.mode === 'array') return ['short'];
        if (step.mode === 'tag') return { tag: step.tag, input };
        return input;
      };
      const options = { typeName: step.typeName, fieldName: step.field, aggregate: step.individual === undefined ? undefined : !step.individual, throwOnError: step.throwOnError };
      if (step.kind === 'batch') target.batchResolver(handler, options);
      else target.resolver(handler, options);
    } else if (step.op === 'exception') {
      const classes = step.names.map((name) => ({ [name]: class extends Error {} })[name]);
      target.exceptionHandler(classes, async (error) => {
        calls.push({ exception: error.name, tag: step.tag ?? '' });
        if (step.mode === 'error') throw new Error('exception failed');
        if (step.mode === 'unknown') throw 42;
        return { handled: error.message, tag: step.tag ?? '' };
      });
    } else if (step.op === 'include') app.includeRouter(step.routers.map((name) => routers[name]));
    else {
      try { results.push({ value: normalize(await app.resolve(step.event, { requestId: 'request' })), error: null }); }
      catch (error) { results.push({ value: null, error: failure(error) }); }
    }
  }
  cases.push({ steps, results, calls, logs });
}
const register = (mode = 'echo', kind = 'single', extra = {}) => ({ op: 'register', mode, kind, field: 'hello', ...extra });
const resolve = (event) => ({ op: 'resolve', event });
for (const typeName of ['Query', 'Mutation', 'Custom', '', 'a.b']) {
  await run([register('tag', 'single', { typeName, tag: 'first' }), resolve(base('hello', typeName)), resolve(base('missing', typeName)), register('tag', 'single', { typeName, tag: 'second' }), resolve(base('hello', typeName))]);
}
for (const mode of ['echo', 'tag', 'error', 'unknown', 'missing', 'invalid', 'null', 'undefined', 'array', 'mixed']) {
  await run([register(mode, 'single', { tag: 'one' }), resolve(base()), resolve(base('hello', 'Query', { fail: true }))]);
  for (const individual of [undefined, false, true]) for (const throwOnError of [false, true]) {
    await run([register(mode, 'batch', { individual, throwOnError, tag: 'batch' }), resolve([base(), base('other', 'Mutation', { fail: true }), base('last')])]);
  }
}
for (const mode of ['handled', 'error', 'unknown']) for (const names of [['TypeError'], ['Error'], ['TypeError', 'Error'], ['ResolverNotFoundException', 'InvalidBatchResponseException']]) {
  await run([{ op: 'exception', names, mode, tag: 'catch' }, register('error'), resolve(base()), register('missing'), resolve(base()), register('invalid'), resolve(base()), register('unknown'), resolve(base())]);
}
await run([register('echo'), register('array', 'batch'), resolve(base()), resolve([base()]), resolve([])]);
await run([resolve(base()), resolve([base()]), resolve([])]);
await run([register('tag', 'single', { typeName: 'a.b', field: 'c', tag: 'collision' }), resolve(base('b.c', 'a'))]);
await run([
  register('tag', 'single', { tag: 'original' }),
  register('error', 'single', { target: 'a' }), register('array', 'batch', { target: 'a' }),
  { op: 'exception', target: 'a', names: ['TypeError'], mode: 'handled', tag: 'a' },
  { op: 'exception', names: ['TypeError'], mode: 'error' },
  { op: 'include', routers: ['a'] }, resolve(base()), resolve([base()]),
  register('tag', 'single', { target: 'a', tag: 'changed' }), resolve(base()),
  { op: 'include', routers: ['a', 'app'] }, resolve(base()),
  register('tag', 'single', { target: 'b', tag: 'b' }), { op: 'include', routers: ['a', 'b'] }, resolve(base()),
]);
await run([{ op: 'exception', names: ['TypeError', 'TypeError'], mode: 'handled' }, register('error'), resolve(base())]);
const invalid = [null, {}, 1, 'event', [base(), null]];
for (const key of Object.keys(base())) { const event = base(); delete event[key]; invalid.push(event); }
for (const [path, value] of [[['arguments'], []], [['request', 'headers'], null], [['info', 'fieldName'], 1], [['info', 'parentTypeName'], null], [['info', 'variables'], []], [['stash'], null]]) {
  const event = base(); let target = event; for (const key of path.slice(0, -1)) target = target[key]; target[path.at(-1)] = value; invalid.push(event);
}
for (const event of invalid) await run([register(), resolve(event)]);
for (const key of ['domainName', 'headers']) { const event = base(); delete event.request[key]; await run([resolve(event)]); }
for (const event of [base(), { ...base(), identity: false, source: 42, prev: [] }]) await run([register(), resolve(event)]);

const scalars = [];
const RealDate = Date;
for (const millis of [0, -1, 1709251199123, 253402300799999, -62167219200000, 8640000000000000, -8640000000000000]) {
  globalThis.Date = class extends RealDate { constructor(...args) { super(...(args.length ? args : [millis])); } static now() { return millis; } };
  for (const offset of [0, -12, 14, 5.5, 5.75, -3.5, 0.1, 0.0000001, -12.01, 14.01, 'NaN', 'Infinity', '-Infinity']) {
    const value = typeof offset === 'string' ? Number(offset) : offset;
    const values = {};
    for (const [name, fn] of Object.entries({ date: awsDate, time: awsTime, datetime: awsDateTime })) {
      try { values[name] = { value: fn(value), error: null }; } catch (error) { values[name] = { value: null, error: failure(error) }; }
    }
    scalars.push({ millis, offset, timestamp: awsTimestamp(), values });
  }
}
globalThis.Date = RealDate;
writeFileSync('../../eventhandler/appsyncgraphql/testdata/typescript-v2.35.0.json', `${JSON.stringify({ version: '2.35.0', cases, scalars }, null, 2)}\n`);
console.log(`Generated ${cases.length} GraphQL scenarios and ${scalars.length} scalar cases`);
