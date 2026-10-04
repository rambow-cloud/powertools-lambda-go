// Capture the pinned UTF-8 contracts across the three text-decoding utilities.
import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';
import { z } from 'zod';
import { Base64Encoded } from '@aws-lambda-powertools/parser/helpers';
import { search } from '@aws-lambda-powertools/jmespath';
import { PowertoolsFunctions } from '@aws-lambda-powertools/jmespath/functions';
import { transformValue } from './node_modules/@aws-lambda-powertools/parameters/lib/esm/base/transformValue.js';

// An optional output root allows reference dependencies to stay in an isolated directory.
const root = process.argv[2] ? resolve(process.argv[2]) : fileURLToPath(new URL('../..', import.meta.url));
const samples = [
  ['plain', Buffer.from('hello')],
  ['valid-unicode', Buffer.from('你好😀')],
  ['invalid-bytes', Buffer.from([0xff, 0xfe])],
  ['truncated-sequence', Buffer.from([0xe2, 0x82])],
  ['surrogate', Buffer.from([0xed, 0xa0, 0x80])],
  ['bom-text', Buffer.from('\ufeffhello')],
  ['double-bom', Buffer.from('\ufeff\ufeffhello')],
  ['bom-json', Buffer.from('\ufeff{"id":1}')],
  ['invalid-json-string', Buffer.from([34, 0xff, 0xfe, 34])],
  ['partial-json-string', Buffer.from([34, 0xe2, 0x82, 34])],
];
const cases = {parser: [], jmespath: [], parameters: []};
for (const [name, bytes] of samples) {
  const input = bytes.toString('base64');
  const gzip = gzipSync(bytes).toString('base64');
  cases.parser.push({name, kind: 'plain', input, expected: Base64Encoded(z.unknown()).parse(input)});
  cases.parser.push({name, kind: 'gzip', input: gzip, expected: Base64Encoded(z.unknown()).parse(gzip)});
  cases.jmespath.push({name, kind: 'base64', input, expected: search('powertools_base64(@)', input, {customFunctions: new PowertoolsFunctions()})});
  cases.jmespath.push({name, kind: 'gzip', input: gzip, expected: search('powertools_base64_gzip(@)', gzip, {customFunctions: new PowertoolsFunctions()})});
  cases.parameters.push({name, kind: 'binary', input, expected: transformValue(input, 'binary', true, 'fixture')});
  if (name.endsWith('json-string') || name === 'bom-json') {
    cases.parameters.push({name, kind: 'json-bytes', input, expected: transformValue(new Uint8Array(bytes), 'json', true, 'fixture')});
  }
}
for (const [name, bytes] of samples.filter(([name]) => ['invalid-bytes', 'bom-text', 'double-bom'].includes(name))) {
  const raw = Buffer.from('\ufeff' + bytes.toString('base64'));
  cases.parameters.push({name: 'bom-base64-input-' + name, kind: 'binary-bytes', input: raw.toString('base64'), expected: transformValue(new Uint8Array(raw), 'binary', true, 'fixture')});
}
for (const [module, entries] of Object.entries(cases)) {
  writeFileSync(resolve(root, module, 'testdata', 'utf8-v2.35.0.json'), JSON.stringify({reference: '2.35.0', cases: entries}, null, 2) + '\n');
  console.log(`${module}: ${entries.length} pinned UTF-8 cases`);
}
