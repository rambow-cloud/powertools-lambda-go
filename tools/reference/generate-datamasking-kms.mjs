import { createServer } from 'node:http';
import { randomBytes } from 'node:crypto';
import { createRequire } from 'node:module';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { isDeepStrictEqual } from 'node:util';
const key = 'arn:aws:kms:ap-east-1:111122223333:key/11111111-1111-4111-8111-111111111111';
const otherKey = 'arn:aws:kms:us-east-1:111122223333:key/22222222-2222-4222-8222-222222222222';
let requests = 0;
// Wrapped keys deliberately contain plaintext test material. This endpoint
// tests actual ESDK message encryption, not real KMS wrapping or authorization.
const server = createServer(async (request, response) => {
  requests++;
  let body = '';
  for await (const chunk of request) body += chunk;
  try {
    const input = JSON.parse(body);
    const operation = request.headers['x-amz-target']?.split('.').at(-1);
    let output;
    if (operation === 'GenerateDataKey' || operation === 'Encrypt') {
      const plaintext = operation === 'GenerateDataKey' ? randomBytes(input.NumberOfBytes).toString('base64') : input.Plaintext;
      const wrapped = { key: input.KeyId, plaintext, context: input.EncryptionContext };
      output = { KeyId: input.KeyId, CiphertextBlob: Buffer.from(JSON.stringify(wrapped)).toString('base64'), ...(operation === 'GenerateDataKey' ? { Plaintext: plaintext } : {}) };
    } else if (operation === 'Decrypt') {
      const wrapped = JSON.parse(Buffer.from(input.CiphertextBlob, 'base64').toString());
      if (wrapped.key !== input.KeyId || !isDeepStrictEqual(wrapped.context, input.EncryptionContext)) throw new Error('invalid fixture wrapped key or context');
      output = { KeyId: wrapped.key, Plaintext: wrapped.plaintext, EncryptionAlgorithm: 'SYMMETRIC_DEFAULT' };
    } else throw new Error('unexpected KMS fixture operation');
    response.writeHead(200, { 'content-type': 'application/x-amz-json-1.1' }).end(JSON.stringify(output));
  } catch (error) {
    response.writeHead(400, { 'content-type': 'application/x-amz-json-1.1' }).end(JSON.stringify({ __type: 'InvalidCiphertextException', message: error.message }));
  }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
Object.assign(process.env, { AWS_ENDPOINT_URL_KMS: `http://127.0.0.1:${server.address().port}`, AWS_ACCESS_KEY_ID: 'fixture', AWS_SECRET_ACCESS_KEY: 'fixture', AWS_REGION: 'ap-east-1', AWS_EC2_METADATA_DISABLED: 'true' });
const require = createRequire(import.meta.url);
const { AWSEncryptionSDKProvider } = require('@aws-lambda-powertools/data-masking/providers/kms');
const { buildEncrypt, KmsKeyringNode } = require('@aws-crypto/client-node');
const fixturePath = '../../integration/maskingkms/testdata/typescript-v2.35.0.json';
const outcome = async operation => {
  try { return { value: await operation() }; }
  catch (error) { return { error: { name: error.name, message: error.message } }; }
};
try {
  if (process.argv[2] === '--verify-go') {
    const fixture = JSON.parse(readFileSync(fixturePath, 'utf8'));
    const executable = resolve(process.argv[3]);
    const command = process.platform === 'win32' ? ['uv', ['run', 'python', fileURLToPath(new URL('../run-linux-test.py', import.meta.url)), executable]] : [executable, []];
    const execution = spawnSync(command[0], command[1], { input: JSON.stringify(fixture.cases.filter(item => !item.raw)), encoding: 'utf8', maxBuffer: 8 * 1024 * 1024 });
    if (execution.status !== 0) throw new Error(execution.stderr || execution.error || 'Go bridge failed');
    const results = JSON.parse(execution.stdout);
    const checks = [];
    for (const item of results) {
      const provider = new AWSEncryptionSDKProvider({ keys: item.keys });
      const value = await provider.decrypt(item.ciphertext, item.context);
      checks.push({ name: item.name + ': Go to TypeScript authenticated decryption', passed: value === item.expected });
      const header = Buffer.from(item.ciphertext, 'base64');
      checks.push({ name: item.name + ': committed signed default suite', passed: header[0] === 2 && header.readUInt16BE(1) === 0x0578 });
      const wrong = await outcome(() => provider.decrypt(item.ciphertext, { missing: '' }));
      checks.push({ name: item.name + ': context mismatch', passed: wrong.error?.name === 'DataMaskingEncryptionError' && wrong.error.message === "Encryption context mismatch for key 'missing'" });
    }
    const report = { scope: 'Actual AWS Encryption SDK messages through local KMS fixtures; no real KMS/cache acceptance', typescript: '2.35.0', clientNode: '5.0.2', goSDK: '0.4.0', completed: checks.every(check => check.passed), requests, checks };
    report.execution = Object.fromEntries([
      ['source_sha', process.env.GITHUB_SHA], ['run_id', process.env.GITHUB_RUN_ID],
      ['run_attempt', process.env.GITHUB_RUN_ATTEMPT],
    ].filter(([, value]) => value));
    const reportPath = process.env.POWERTOOLS_ACCEPTANCE_DIR
      ? resolve(process.env.POWERTOOLS_ACCEPTANCE_DIR, 'DATAMASKING_KMS_INTEROP.json')
      : '../../docs/DATAMASKING_KMS_INTEROP.json';
    mkdirSync(dirname(reportPath), { recursive: true });
    writeFileSync(reportPath, JSON.stringify(report, null, 2) + '\n');
    if (!report.completed) throw new Error('Go/TypeScript interoperability failed');
    console.log(`${checks.length}/${checks.length} Go-to-TypeScript checks passed`);
  } else {
    const cases = [];
    for (const keys of [[key], [key, otherKey]]) for (const context of [null, {}, { tenant: 'alpha', label: '世界' }]) for (const text of ['', 'hello', 'αβ 😀 世界', '\ufeffBOM', 'line\r\n\u2028next', 'é😀'.repeat(1000)]) {
      const provider = new AWSEncryptionSDKProvider({ keys });
      const ciphertext = await provider.encrypt(text, context ?? undefined);
      const expected = await provider.decrypt(ciphertext, context ?? undefined);
      const checks = [];
      for (const wanted of [null, {}, context, { missing: '' }, { tenant: 'wrong' }]) checks.push({ context: wanted, ...await outcome(() => provider.decrypt(ciphertext, wanted ?? undefined)) });
      cases.push({ name: 'kms-' + cases.length, keys, context, text, ciphertext, expected, checks });
    }
    const keyring = new KmsKeyringNode({ generatorKeyId: key });
    for (const bytes of [[0xef, 0xbb, 0xbf, 65], [0xf0, 0x9f], [0xff, 65, 0xc0, 0xaf]]) {
      const { result } = await buildEncrypt().encrypt(keyring, Buffer.from(bytes));
      const provider = new AWSEncryptionSDKProvider({ keys: [key] });
      const ciphertext = result.toString('base64');
      cases.push({ name: 'raw-' + cases.length, keys: [key], context: null, raw: true, ciphertext, expected: await provider.decrypt(ciphertext) });
    }
    const first = cases[0];
    const malformed = ['', '!', first.ciphertext.slice(0, 30), Buffer.from([0, 1, 2]).toString('base64')];
    const damaged = Buffer.from(first.ciphertext, 'base64'); damaged[damaged.length - 1] ^= 1; malformed.push(damaged.toString('base64'));
    const provider = new AWSEncryptionSDKProvider({ keys: [key] });
    const failures = [];
    for (const ciphertext of malformed) failures.push({ ciphertext, ...await outcome(() => provider.decrypt(ciphertext)) });
    mkdirSync('../../integration/maskingkms/testdata', { recursive: true });
    writeFileSync(fixturePath, JSON.stringify({ version: '2.35.0', clientNode: '5.0.2', cases, failures }, null, 2) + '\n');
    console.log(`Wrote ${cases.length} encrypted-message cases and ${failures.length} rejection cases`);
  }
} finally { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); }
