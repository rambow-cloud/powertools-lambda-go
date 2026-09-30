import { BedrockAgentFunctionResolver, BedrockFunctionResponse } from '@aws-lambda-powertools/event-handler/bedrock-agent';
import { writeFileSync } from 'node:fs';

const normalize = (value) => {
  if (value === undefined) return { $: 'undefined' };
  if (typeof value === 'number' && (!Number.isFinite(value) || Object.is(value, -0))) return { $: Object.is(value, -0) ? '-0' : String(value) };
  if (Array.isArray(value)) return value.map(normalize);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, normalize(v)]));
  return value;
};
const decode = (value) => {
  if (value && typeof value === 'object' && !Array.isArray(value) && '$' in value) {
    if (value.$ === 'undefined') return undefined;
    return Number(value.$);
  }
  if (Array.isArray(value)) return value.map(decode);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, decode(v)]));
  return value;
};
const errorText = (value) => value instanceof Error ? `${value.name} - ${value.message}` : String(value);
const base = (parameters = []) => ({ messageVersion: '1.0', actionGroup: 'orders', function: 'tool', agent: { name: 'agent', id: 'id', alias: 'alias', version: '1' }, parameters, inputText: 'input', sessionId: 'session', sessionAttributes: { saved: 'session' }, promptSessionAttributes: { saved: 'prompt' } });
const param = (name, type, value) => ({ name, type, value });
const cases = [];
async function run(steps) {
  const calls = [], logs = [], results = [];
  const app = new BedrockAgentFunctionResolver({ logger: {
    debug(message) { logs.push(['debug', message]); }, warn(message) { logs.push(['warn', message]); },
    error(message, error) { logs.push(error === undefined ? ['error', message] : ['error', message, errorText(error)]); },
  } });
  for (const step of steps) {
    if (step.op === 'register') {
      app.tool(async (params, { event, context }) => {
        calls.push({ params: normalize(params), keys: Object.keys(params), tag: step.tag ?? '', request: context.requestId, session: event.sessionId });
        if (step.mode === 'error') throw new TypeError('tool failed');
        if (step.mode === 'throw') throw decode(step.value);
        if (step.mode === 'return') return decode(step.value);
        if (step.mode === 'explicit') return new BedrockFunctionResponse(decode(step.response));
        if (step.mode === 'mutate') {
          event.sessionAttributes.changed = true;
          event.sessionAttributes = { replaced: true };
          event.promptSessionAttributes = { replaced: true };
          return 'mutated';
        }
        if (step.mode === 'params') { params.extra = 'added'; delete params.remove; return params; }
        if (step.mode === 'tag') return step.tag;
        return params;
      }, { name: step.name ?? 'tool', description: step.description });
    } else if (step.op === 'build') {
      const response = new BedrockFunctionResponse(decode(step.response));
      results.push({ value: JSON.parse(JSON.stringify(response.build({ actionGroup: 'group', func: 'function' }))), error: null });
    } else {
      const event = structuredClone(step.event);
      try { results.push({ value: JSON.parse(JSON.stringify(await app.resolve(event, { requestId: 'request' }))), error: null }); }
      catch (error) { results.push({ value: null, error: errorText(error) }); }
    }
  }
  cases.push({ steps, results, calls, logs });
}
const register = (mode = 'echo', extra = {}) => ({ op: 'register', mode, ...extra });
const resolve = (event = base()) => ({ op: 'resolve', event });
const strings = ['', ' ', '\uFEFF', '0', '-0', '+0', '1', '-1', '1.5', '01', '0x10', '-0x10', '0b11', '0o10', '0x', '1e3', '1e309', '-1e309', '1e-324', '5e-324', '0.000001', '0.0000001', '1e20', '1e21', '9007199254740993', 'NaN', 'Infinity', '+Infinity', '-Infinity', 'inf', 'true', 'TRUE', 'false', ' true ', 'null', 'undefined', '[1,2]', 'abc', '\u00851\u0085', '\u20281\u2029', '<>&', 'é😀', '\u2028\u2029', '\\u2028'];
for (const type of ['string', 'number', 'integer', 'boolean', 'array', 'unknown']) for (const value of strings) {
  await run([register(), resolve(base([param('value', type, value)]))]);
}
for (const parameters of [[], [param('z', 'string', 'last'), param('a', 'string', 'first')], [param('10', 'number', '10'), param('2', 'number', '2'), param('a', 'string', 'a'), param('0', 'string', 'zero')], [param('__proto__', 'string', 'ignored'), param('constructor', 'string', 'own'), param('toString', 'boolean', 'false')], [param('same', 'string', 'before'), param('other', 'number', '2'), param('same', 'boolean', 'true')], [param('remove', 'string', 'gone')]]) {
  await run([register(), resolve(base(parameters)), register('params'), resolve(base(parameters))]);
}
for (const value of [null, '', 'hello', '<>&\u2028\u2029', '\\u2028', true, false, 0, { $: '-0' }, { $: 'NaN' }, { $: 'Infinity' }, { $: '-Infinity' }, { $: 'undefined' }, [1, { $: 'undefined' }, { $: 'Infinity' }], { a: 1, b: { $: 'undefined' }, c: '\u2028' }]) {
  await run([register('return', { value }), resolve()]);
}
for (const value of [42, false, 'boom', {}, [1, null, 'x'], { $: 'Infinity' }]) await run([register('throw', { value }), resolve()]);
await run([register('error'), resolve()]);
await run([register('mutate'), resolve()]);
for (const field of ['responseState', 'sessionAttributes', 'promptSessionAttributes', 'knowledgeBasesConfiguration']) for (const value of [null, false, 0, '', {}, [], { key: 'value' }, 'REPROMPT', 'FAILURE']) {
  const response = { body: 'explicit', [field]: value };
  await run([register('explicit', { response }), resolve({ ...base(), knowledgeBasesConfiguration: { inherited: true } }), { op: 'build', response }]);
}
for (const body of ['raw', '', null, false, 0, { a: 1 }, { $: 'undefined' }]) await run([{ op: 'build', response: { body } }]);
await run([resolve(), register('tag', { tag: 'first' }), resolve(), register('tag', { tag: 'second' }), resolve(), resolve({ ...base(), actionGroup: 'different', messageVersion: '2.0' })]);
for (const name of ['', '工具', 'a"b', 'a\nb', '__proto__']) await run([register('tag', { name, tag: name }), resolve({ ...base(), function: name }), register('tag', { name, tag: 'replacement' }), resolve({ ...base(), function: name })]);
const missingParameters = base(); delete missingParameters.parameters;
await run([register(), resolve(missingParameters), resolve({ ...base(), sessionAttributes: {}, promptSessionAttributes: {}, knowledgeBasesConfiguration: [] })]);
const invalid = [null, [], {}, 42, 'event'];
for (const key of Object.keys(base()).filter((key) => key !== 'parameters')) { const event = base(); delete event[key]; invalid.push(event); }
for (const key of ['name', 'id', 'alias', 'version']) { const event = base(); delete event.agent[key]; invalid.push(event); }
for (const [path, value] of [[['parameters'], null], [['parameters'], {}], [['parameters'], [null]], [['parameters'], [{ name: 'x', type: 'number', value: 1 }]], [['parameters'], [{ name: 'x', value: '1' }]], [['sessionAttributes'], []], [['promptSessionAttributes'], null], [['agent'], []], [['function'], false], [['actionGroup'], 1], [['agent', 'version'], 1]]) {
  const event = base(); let target = event; for (const key of path.slice(0, -1)) target = target[key]; target[path.at(-1)] = value; invalid.push(event);
}
for (const event of invalid) await run([register(), resolve(event)]);
writeFileSync('../../eventhandler/bedrock/testdata/typescript-v2.35.0.json', `${JSON.stringify({ version: '2.35.0', cases }, null, 2)}\n`);
console.log(`Generated ${cases.length} Bedrock scenarios`);
