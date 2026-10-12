// Exercise the pinned Powertools injection path with Ajv2019, preserving errors.
import { readFileSync, writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';
import Ajv2019 from 'ajv/dist/2019.js';
const ajvPackage = JSON.parse(readFileSync(new URL('./node_modules/ajv/package.json', import.meta.url)));
const powertoolsPackage = JSON.parse(readFileSync(new URL('./node_modules/@aws-lambda-powertools/validation/package.json', import.meta.url)));

if (ajvPackage.version !== '8.20.0' || powertoolsPackage.version !== '2.35.0') {
  throw new Error('This corpus requires AJV 8.20.0 and Powertools TypeScript 2.35.0');
}
const dialect = 'https://json-schema.org/draft/2019-09/schema';
const cases = [];
function add(name, schema, payload, references = [], nativeSuccess, nativeReason) {
  const item = { name, schema, payload, references };
  if (nativeSuccess !== undefined) {
    if (!nativeReason) throw new Error('Every native difference requires an explicit reason');
    item.nativeSuccess = nativeSuccess;
    item.nativeReason = nativeReason;
  }
  try {
    const ajv = new Ajv2019({ allErrors: true, logger: false, strictTypes: false, strictTuples: false });
    const value = validate({ schema: structuredClone(schema), payload: structuredClone(payload),
      externalRefs: structuredClone(references), ajv });
    item.expected = { success: true, value };
  } catch (error) {
    item.expected = { success: false, stage: error.name === 'SchemaValidationError' ? 'validation' : 'compilation',
      error: error.name, message: error.message };
    if (Array.isArray(error.cause)) item.expected.issues = error.cause;
  }
  cases.push(item);
}
function pair(name, schema, valid, invalid, references = []) {
  add(`${name}-valid`, schema, valid, references);
  add(`${name}-invalid`, schema, invalid, references);
}
pair('tuple-items', { $schema: dialect, type: 'array', items: [{ type: 'integer' }, { type: 'string' }], additionalItems: false }, [1, 'ok'], [1, 'ok', true]);
pair('tuple-type', { type: 'array', items: [{ type: 'integer' }], additionalItems: { type: 'string' } }, [1, 'ok'], ['bad', 2]);
pair('dependent-required', { type: 'object', dependentRequired: { card: ['address'] } }, { card: 1, address: 'here' }, { card: 1 });
pair('dependent-schemas', { type: 'object', dependentSchemas: { card: { required: ['address'], properties: { address: { type: 'string' } } } } }, { card: 1, address: 'here' }, { card: 1, address: 2 });
pair('all-of-evaluated', { type: 'object', allOf: [{ properties: { a: { type: 'integer' } } }, { properties: { b: { type: 'string' } } }], unevaluatedProperties: false }, { a: 1, b: 'ok' }, { a: 1, b: 'ok', extra: true });
pair('any-of-evaluated', { type: 'object', anyOf: [{ properties: { a: { type: 'integer' } }, required: ['a'] }, { properties: { b: { type: 'string' } }, required: ['b'] }], unevaluatedProperties: false }, { a: 1, b: 'ok' }, { a: 1, b: 2 });
pair('one-of-evaluated', { type: 'object', oneOf: [{ properties: { a: { type: 'integer' } }, required: ['a'] }, { properties: { b: { type: 'string' } }, required: ['b'] }], unevaluatedProperties: false }, { a: 1 }, { a: 1, b: 'ok' });
pair('conditional-evaluated', { type: 'object', properties: { mode: true }, if: { properties: { mode: { const: 'a' } }, required: ['mode'] }, then: { properties: { a: true } }, else: { properties: { b: true } }, unevaluatedProperties: false }, { mode: 'a', a: 1 }, { mode: 'a', b: 1 });
pair('failed-branch-evaluation', { type: 'object', anyOf: [{ properties: { a: { const: 1 } }, required: ['a'] }, { properties: { b: { const: 2 } }, required: ['b'] }], unevaluatedProperties: false }, { a: 1, b: 2 }, { a: 1, b: 3 });
const containsEvaluated = { type: 'array', contains: { type: 'integer' }, minContains: 1, maxContains: 2, unevaluatedItems: false };
add('contains-evaluated-valid', containsEvaluated, [1, 2], [], false,
  'Native 2019-09 contains does not mark array locations evaluated; pinned Ajv2019 marks them. The earlier draft leaves this interaction unspecified.');
add('contains-evaluated-invalid', containsEvaluated, [1, 'bad'], [], false,
  'Native 2019-09 contains does not mark array locations evaluated; pinned Ajv2019 marks them. The earlier draft leaves this interaction unspecified.');
pair('contains-bounds', { type: 'array', contains: { const: 1 }, minContains: 2, maxContains: 3 }, [1, 2, 1], [1]);
pair('tuple-evaluated', { type: 'array', items: [{ type: 'string' }], unevaluatedItems: false }, ['ok'], ['ok', 2]);
pair('ref-siblings', { $defs: { text: { type: 'string' } }, $ref: '#/$defs/text', minLength: 2 }, 'ok', 'x');
pair('external-evaluated', { type: 'object', $ref: 'https://example.test/common', unevaluatedProperties: false }, { id: 1 }, { id: 1, extra: true }, [{ $id: 'https://example.test/common', $schema: dialect, properties: { id: { type: 'integer' } } }]);
pair('nested-resource', { $id: 'https://example.test/root', $defs: { text: { $id: 'nested/text', type: 'string' } }, $ref: 'nested/text' }, 'ok', 2);
pair('external-nested-resource', { $ref: 'https://example.test/library/nested/text' }, 'ok', 2, [{ $id: 'https://example.test/library/', $defs: { text: { $id: 'nested/text', type: 'string' } } }]);
const tree = { $id: 'https://example.test/tree', $schema: dialect, $recursiveAnchor: true, type: 'object', properties: { data: true, children: { type: 'array', items: { $recursiveRef: '#' } } } };
pair('recursive-extension', { $id: 'https://example.test/strict-tree', $schema: dialect, $recursiveAnchor: true, $ref: tree.$id, unevaluatedProperties: false }, { data: 1, children: [{ data: 2 }] }, { data: 1, children: [{ data: 2, extra: true }] }, [tree]);
pair('unicode-pattern', { type: 'string', pattern: '^\\p{L}+$' }, '你好é', '123');
pair('astral-length', { type: 'string', minLength: 1, maxLength: 1 }, '😀', 'e\u0301');
pair('lookaround', { type: 'string', pattern: '^(?=a+$)a{2,}$' }, 'aa', 'ab');
pair('backreference', { type: 'string', pattern: '^(a+)\\1$' }, 'aaaa', 'aaa');
add('fractional-multiple-native-exact', { type: 'number', multipleOf: 0.1 }, 1.2, [], true,
  'Native exact JSON-number arithmetic accepts 1.2 as a multiple of 0.1; pinned AJV JavaScript division rejects it.');
add('empty-dependent-required', { type: 'object', dependentRequired: { a: [] } }, { a: 1 });
add('annotations-are-data', { type: 'string', default: { unknown: true, $schema: 'not-a-schema' }, examples: [{ typo: true }] }, 'ok');
add('false-schema', false, 'anything');
add('true-schema', true, { anything: true });
for (const [name, schema] of Object.entries({ unknown: { typo: true }, wrongDialect: { $schema: 'http://json-schema.org/draft-07/schema#' }, malformedTuple: { items: [5] }, malformedDependency: { dependentRequired: { a: 'b' } }, malformedPattern: { pattern: '[' }, missingReference: { $ref: 'https://example.test/not-registered' }, duplicateResource: { $defs: { a: { $id: 'https://example.test/duplicate' }, b: { $id: 'https://example.test/duplicate' } } }, duplicateAnchor: { $defs: { a: { $anchor: 'same' }, b: { $anchor: 'same' } } }, unknownVocabulary: { $schema: dialect, $vocabulary: { 'https://example.test/vocabulary': true } }, prefixItems: { prefixItems: [] }, recursiveAnchor: { $recursiveAnchor: 'node' } })) {
  add(`compile-${name}`, schema, {});
}
writeFileSync(new URL('../../validation/testdata/draft-2019-09-v2.35.0.json', import.meta.url), JSON.stringify({ version: '2.35.0', ajvVersion: '8.20.0', dialect, cases }, null, 2) + '\n');
console.log(`${cases.length} injected Draft 2019-09 cases`);
