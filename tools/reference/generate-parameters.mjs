import { writeFileSync, mkdirSync } from 'node:fs';
import { BaseProvider } from '@aws-lambda-powertools/parameters/base';
import { SSMProvider } from '@aws-lambda-powertools/parameters/ssm';
import { SSMClient } from '@aws-sdk/client-ssm';

// Execute the pinned implementation with deterministic stores; no AWS requests.
class FixtureProvider extends BaseProvider {
  calls = 0;
  constructor() { super({}); }
  async _get(name) {
    this.calls++;
    return { plain: 'hello', json: '{"enabled":true,"count":2}', binary: 'aGVsbG8=', missing: undefined, invalid: '{', empty: '', null: 'null' }[name];
  }
  async _getMultiple() {
    this.calls++;
    return { 'flags.JSON': '{"enabled":true}', 'text.binary': 'aGVsbG8=', plain: 'untouched', 'broken.json': '{' };
  }
}
const provider = new FixtureProvider();
const cases = [];
for (const [name, options] of [
  ['plain', {}], ['plain', {}], ['plain', { forceFetch: true }],
  ['json', { transform: 'json' }], ['binary', { transform: 'binary' }],
  ['empty', {}], ['null', { transform: 'json' }], ['missing', {}],
  ['missing', { throwOnMissing: true }], ['invalid', { transform: 'json' }],
]) {
  try { cases.push({ name, options, value: (await provider.get(name, options)) ?? null, calls: provider.calls }); }
  catch (error) { cases.push({ name, options, error: error.name, calls: provider.calls }); }
}
const multiple = await provider.getMultiple('/fixture', { transform: 'auto' });
// Go nil represents JavaScript undefined; retain the failed key explicitly.
for (const key of Object.keys(multiple)) multiple[key] ??= null;
let strictError;
try { await provider.getMultiple('/fixture', { transform: 'auto', forceFetch: true, throwOnTransformError: true }); }
catch (error) { strictError = error.name; }

const sdkCalls = [];
const fakeClient = new SSMClient({ region: 'ap-east-1', credentials: { accessKeyId: 'LOCALTEST', secretAccessKey: 'local-test-only' } });
fakeClient.send = async (command) => {
  sdkCalls.push({ operation: command.constructor.name, input: command.input });
  if (command.constructor.name === 'GetParameterCommand') return { Parameter: { Name: command.input.Name, Value: '{"secret":true}' } };
  return { Parameters: command.input.Names.filter(name => name !== 'missing').map(Name => ({ Name, Value: Name === 'empty' ? '' : '{"enabled":true}' })), InvalidParameters: command.input.Names.includes('missing') ? ['missing'] : [] };
};
const ssm = new SSMProvider({ awsSdkV3Client: fakeClient });
const batches = [];
for (const names of [{ plain: {}, secret: { decrypt: true, transform: 'json' } }, { plain: {}, secret: { decrypt: true, transform: 'json' } }, { empty: {}, missing: {} }]) {
  const values = await ssm.getParametersByName(names, { transform: 'json', throwOnError: false });
  for (const key of Object.keys(values)) values[key] ??= null;
  batches.push(values);
}
mkdirSync('../../parameters/testdata', { recursive: true });
writeFileSync('../../parameters/testdata/typescript-v2.35.0.json', JSON.stringify({ version: '2.35.0', cases, multiple, strictError, batches, sdkCalls }, null, 2) + '\n');
console.log('Generated Parameters cache, transform, and SSM batch reference fixtures');
