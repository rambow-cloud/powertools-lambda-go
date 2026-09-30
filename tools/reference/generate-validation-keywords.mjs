// Capture keyword behavior outside Draft 7 metaschema traversal and numeric edges.
import { writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';
import Ajv from 'ajv';

const cases = [];
function add(name, schema, payload) {
  const item = { name, schema: structuredClone(schema), payload: structuredClone(payload) };
  try {
    const value = validate({ schema: structuredClone(schema), payload: structuredClone(payload), ajv: new Ajv({ allErrors: true, logger: false }) });
    item.expected = { success: true, value };
  } catch (error) {
    item.expected = { success: false, error: error.name, message: error.message };
    if (Array.isArray(error.cause)) item.expected.issues = error.cause;
  }
  cases.push(item);
}
const payloads = [null, false, 0, 1, '', 'a', 'aa', '😀', [], [1], [1, 2], {}, { a: 1 }, { a: 1, b: 2 }];
function shapes(body) {
  return {
    root: body,
    definition: { definitions: { value: body }, $ref: '#/definitions/value' },
    defs: { $defs: { value: body }, $ref: '#/$defs/value' },
    alias: { $defs: { value: body, alias: { $ref: '#/$defs/value' } }, $ref: '#/$defs/alias' },
  };
}
for (const keyword of ['minLength', 'maxLength', 'minItems', 'maxItems', 'minProperties', 'maxProperties']) {
  for (const limit of [-1, -0.5, 0, 0.5, 1.5, 2, 1e21]) {
    for (const [shape, schema] of Object.entries(shapes({ [keyword]: limit }))) {
      for (const payload of payloads) add(`size-${keyword}-${shape}-${cases.length}`, schema, payload);
    }
  }
}
for (const body of [{ allOf: [] }, { anyOf: [] }, { oneOf: [] }, { enum: [] }, { type: [] }, { type: ['string', 'string'] }, { type: 'string', nullable: 'yes' }]) {
  for (const [shape, schema] of Object.entries(shapes(body))) {
    for (const payload of payloads) add(`empty-${shape}-${cases.length}`, schema, payload);
  }
}
for (const required of [[1], [true], [null], ['a', 1, 'a'], [['a', 'b']], [{ a: 1 }]]) {
  for (const [shape, schema] of Object.entries(shapes({ required }))) {
    for (const payload of [{}, { a: 1 }, { '1': null, true: 1, null: false, 'a,b': 2, '[object Object]': 3 }]) add(`required-${shape}-${cases.length}`, schema, payload);
  }
}
for (const multipleOf of [0, -2, -0.1, 0.1, 0.2, 0.01, 1, 2, 1e-7, 1e-21, 1e21]) {
  for (const [shape, schema] of Object.entries(shapes({ multipleOf }))) {
    for (const payload of [null, '', 0, -0.3, 0.3, 0.6, 1, 2, 1e-7, 1e-21, 1e20, 1e21, -1e21]) add(`multiple-${shape}-${cases.length}`, schema, payload);
  }
}
const combined = { minLength: 1.5, maxLength: -1, minItems: 1.5, maxItems: -1, minProperties: 1.5, maxProperties: -1, required: ['a', 1], multipleOf: 0, anyOf: [], oneOf: [] };
for (const payload of payloads) {
  add(`combined-${cases.length}`, shapes(combined).defs, payload);
  add(`nested-${cases.length}`, { $defs: { value: combined }, properties: { child: { $ref: '#/$defs/value' } } }, { child: payload });
}
for (const length of [199, 200, 201]) {
  for (const last of [['a', 'b'], { a: 1 }, 1]) {
    const required = [...Array(length - 1).fill('present'), last];
    add(`required-threshold-${cases.length}`, shapes({ required }).defs, { present: true });
  }
}
for (const multipleOf of [1e-300, 1e300]) {
  for (const payload of [0, 1e-300, 1e300, -1e300]) add(`multiple-extreme-${cases.length}`, { multipleOf }, payload);
}
writeFileSync('../../validation/testdata/keywords-v2.35.0.json', JSON.stringify({ version: '2.35.0', ajvVersion: '8.20.0', cases }, null, 2) + '\n');
console.log(`${cases.length} validation keyword cases`);
