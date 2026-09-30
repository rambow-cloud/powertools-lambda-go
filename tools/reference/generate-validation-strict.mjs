// Record strict compilation decisions independently of Unicode payload matching.
import { writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';
import Ajv from 'ajv';

const cases = [];
function add(name, schema, payload) {
  const item = { name, schema: structuredClone(schema), payload };
  try {
    const value = validate({ schema: structuredClone(schema), payload: structuredClone(payload), ajv: new Ajv({ allErrors: true, logger: false }) });
    item.expected = { success: true, value };
  } catch (error) {
    item.expected = { success: false, error: error.name, message: error.message };
    if (Array.isArray(error.cause)) item.expected.issues = error.cause;
  }
  cases.push(item);
}
const patterns = ['^.$', '^..$', '^😀$', '^[😀]$', '^😀+$', '^😀{2}$', '^\\uD83D\\uDE00$', '^\\u{1F600}$', '^\\p{Letter}+$', '^\\P{Letter}+$', '^[\\p{Letter}]+$', '^[\\P{Letter}]+$', '^\\p{ASCII}$', '^\\u{61}$', '^\\u{0061}$', '^(?<word>.)\\k<word>$', '^(?<word>😀)\\k<word>$', '^(?<𐐀>.)\\k<𐐀>$', '^.$|^aa$', '^[😀-😁]$', '^\\s$', '^\\w$', '^\\d$', '^a\\.$', '^a[.]$', '^a$', '^a?$', '(?<=a)b', '^(a)\\1$'];
const names = ['', 'a', 'aa', '😀', '😀😀', '😁', 'ab', 'p{Letter}', 'P{Letter}', 'p', 'P', 'u{1F600}', 'u', 'a.', '1', '\n', '\r', '\u2028', '\u2029', '\uFEFF', '𐐀'];
for (const pattern of patterns) {
  for (const name of names) {
    for (const constraint of [true, { type: 'integer' }, { title: 'Annotation only' }]) {
      const schema = { properties: { [name]: { type: 'string' } }, patternProperties: { [pattern]: constraint } };
      add(`overlap-${cases.length}`, schema, { [name]: 'bad' });
    }
  }
}
for (const schema of [
  { properties: { a: true }, patternProperties: { '^a': true, '^z': { type: 'string' } } },
  { properties: { a: true }, patternProperties: { '^a': { title: 'Unused assertion' } } },
]) add(`always-valid-${cases.length}`, schema, { a: 1 });
for (const pattern of patterns) {
  for (const properties of [undefined, {}]) {
    const schema = { patternProperties: { [pattern]: { type: 'integer' } } };
    if (properties !== undefined) schema.properties = properties;
    for (const name of names) add(`no-named-properties-${cases.length}`, schema, { [name]: 'bad' });
  }
}
writeFileSync('../../validation/testdata/strict-v2.35.0.json', JSON.stringify({ version: '2.35.0', ajvVersion: '8.20.0', cases }, null, 2) + '\n');
console.log(`${cases.length} validation strict-pattern cases`);
