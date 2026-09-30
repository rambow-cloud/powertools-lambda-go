// Preserve compilation timing and source keyword types, including ignored rules.
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
const values = [null, false, true, 0, 1, '', 'x', [], [true], {}, { type: 'string' }];
const payloads = [{}, { value: 1 }, [1], 'value'];
const keywords = ['minimum', 'maximum', 'exclusiveMinimum', 'exclusiveMaximum', 'multipleOf', 'minLength', 'maxLength', 'minItems', 'maxItems', 'minProperties', 'maxProperties', 'pattern', 'format', 'required', 'uniqueItems', 'properties', 'patternProperties', 'dependencies', 'items', 'additionalItems', 'additionalProperties', 'contains', 'propertyNames', 'not', 'if', 'then', 'else', 'allOf', 'anyOf', 'oneOf', 'enum', 'type', 'nullable', 'deprecated', 'contentSchema'];
for (const keyword of keywords) {
  for (const value of values) {
    const body = { [keyword]: value };
    const schemas = { root: body, used: { $defs: { value: body }, $ref: '#/$defs/value' }, unused: { $defs: { value: body } } };
    for (const [shape, schema] of Object.entries(schemas)) {
      for (const payload of payloads) add(`${keyword}-${shape}-${cases.length}`, schema, payload);
    }
  }
}
for (const value of values) {
  const contexts = {
    tuple: { items: [true], additionalItems: value },
    condition: { if: value, then: false },
    then: { if: true, then: value },
    else: { if: false, else: value },
    nullable: { type: 'string', nullable: value },
  };
  for (const [name, body] of Object.entries(contexts)) {
    for (const payload of payloads) add(`context-${name}-${cases.length}`, { $defs: { value: body }, $ref: '#/$defs/value' }, payload);
  }
}
for (const value of values) {
  const containers = { properties: { properties: { value } }, patterns: { patternProperties: { '^value$': value } }, dependencies: { dependencies: { value } }, tuple: { items: [value] }, allOf: { allOf: [value] }, anyOf: { anyOf: [value] }, oneOf: { oneOf: [value] } };
  for (const [name, body] of Object.entries(containers)) {
    for (const payload of payloads) add(`child-${name}-${cases.length}`, { $defs: { value: body }, $ref: '#/$defs/value' }, payload);
  }
}
for (const condition of [true, false, {}, { type: 'string' }, { typo: true }]) {
  for (const then of [true, false, {}, { typo: true }, { type: 'number' }]) {
    for (const otherwise of [true, false, {}, { typo: true }, { type: 'boolean' }]) {
      for (const payload of payloads) add(`branches-${cases.length}`, { if: condition, then, else: otherwise }, payload);
    }
  }
}
for (const value of values) {
  for (const payload of payloads) add(`direct-fragment-${cases.length}`, { $defs: { value }, $ref: '#/$defs/value' }, payload);
}
for (const dependency of [[null], [1], [true], ['missing', 1, 'missing'], [['a', 'b']], [{ a: 1 }]]) {
  const schema = { $defs: { value: { dependencies: { value: dependency } } }, $ref: '#/$defs/value' };
  for (const payload of [{}, { value: 1 }, { value: 1, '1': true, null: true, true: 1, 'a,b': null, '[object Object]': 3 }]) {
    add(`dependency-${cases.length}`, schema, payload);
    add(`nested-dependency-${cases.length}`, { $defs: schema.$defs, properties: { child: { $ref: schema.$ref } } }, { child: payload });
  }
}
for (const condition of [{ typo: true }, { pattern: '(?i)a' }, { format: 'unknown' }]) {
  for (const payload of payloads) add(`ignored-condition-${cases.length}`, { if: condition, then: true, else: { deprecated: true } }, payload);
}
for (const branch of [{ pattern: '(?i)a' }, { format: 'unknown' }, { $ref: '#/$defs/missing' }]) {
  for (const condition of [true, false]) {
    for (const payload of payloads) add(`unselected-branch-${cases.length}`, condition ? { if: true, then: true, else: branch } : { if: false, then: branch, else: true }, payload);
  }
}
writeFileSync('../../validation/testdata/shapes-v2.35.0.json', JSON.stringify({ version: '2.35.0', ajvVersion: '8.20.0', cases }, null, 2) + '\n');
console.log(`${cases.length} validation schema-shape cases`);
