// Exercise inactive condition graphs without losing actively referenced nodes.
import { writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';
import Ajv from 'ajv';

const cases = [];
function add(name, schema, payload, options = {}) {
  const item = { name, schema: structuredClone(schema), payload: structuredClone(payload), options: structuredClone(options) };
  try {
    const value = validate({ schema: structuredClone(schema), payload: structuredClone(payload), ...structuredClone(options), ajv: new Ajv({ allErrors: true, logger: false }) });
    item.expected = { success: true, value };
  } catch (error) {
    item.expected = { success: false, error: error.name, message: error.message };
    if (Array.isArray(error.cause)) item.expected.issues = error.cause;
  }
  cases.push(item);
}
const payloads = [null, 0, 'value', {}, { value: 1 }, { value: 'bad' }];
const conditions = [
  { $ref: '#/missing' }, { $ref: 'https://example.test/missing' },
  { $schema: 'https://example.test/dialect' },
  { properties: { value: { $ref: '#/missing' } } },
  { $schema: 'https://json-schema.org/draft/2020-12/schema', type: 'number' },
  { pattern: '(?i)a' }, { format: 'unknown' }, { typo: true },
];
for (const condition of conditions) {
  for (const then of [true, false, { type: 'number' }]) {
    for (const otherwise of [true, false]) {
      for (const explicit of [false, true]) {
        const schema = { if: condition, then, else: otherwise };
        if (explicit) schema.$schema = 'http://json-schema.org/draft-07/schema#';
        for (const payload of payloads) add(`condition-${cases.length}`, schema, payload);
      }
    }
  }
}
for (const dialect of ['', 'http://json-schema.org/schema', 'http://json-schema.org/draft-07/schema#', 'https://json-schema.org/draft-07/schema', 'https://example.test/dialect', null, 1, {}]) {
  const value = { $schema: dialect, type: 'number' };
  const schemas = {
    root: value,
    definition: { definitions: { value }, $ref: '#/definitions/value' },
    defs: { $defs: { value }, $ref: '#/$defs/value' },
    id: { $id: 'https://example.test/root', $defs: { value: { ...value, $id: 'value' } }, $ref: 'value' },
    property: { properties: { value } },
  };
  for (const [shape, schema] of Object.entries(schemas)) {
    for (const payload of payloads) add(`dialect-${shape}-${cases.length}`, schema, payload);
  }
}
for (const target of ['if', 'then', 'else', 'if/properties/value']) {
  for (const definition of [{ type: 'number' }, { $ref: '#/missing' }, { format: 'unknown' }]) {
    const schema = { if: { properties: { value: definition } }, then: { title: 'Then annotation' }, else: { title: 'Else annotation' } };
    if (target === 'if') schema.if = definition;
    const referenced = target === 'then' || target === 'else' ? { type: 'number' } : definition;
    if (target === 'then' || target === 'else') schema[target].$defs = { value: referenced };
    const reference = target === 'then' || target === 'else' ? `#/${target}/$defs/value` : `#/${target}`;
    schema.properties = { value: { $ref: reference } };
    for (const payload of payloads) add(`active-${target}-${cases.length}`, schema, payload);
  }
}
for (const declaration of ['if', 'then', 'else']) {
  const schema = { $id: 'https://example.test/root', if: true, then: true, else: true };
  schema[declaration] = { $id: 'condition', $schema: 'https://example.test/dialect', type: 'number' };
  schema.properties = { value: { $ref: 'condition' } };
  for (const payload of payloads) add(`resource-${declaration}-${cases.length}`, schema, payload);
}
const external = { $id: 'https://example.test/external', $schema: 'http://json-schema.org/draft-07/schema#', if: { $id: 'condition', $schema: 'https://example.test/dialect', type: 'number' }, then: true, else: true };
for (const reference of [external.$id, `${external.$id}#/if`, 'https://example.test/condition']) {
  for (const payload of payloads) add(`external-${cases.length}`, { $ref: reference }, payload, { externalRefs: [external] });
}
for (const key of ['value', 'a/b~c', 'space % ü']) {
  for (const identity of ['child', '../child', 'nested/child']) {
    const document = {
      $id: 'https://example.test/schemas/document',
      $defs: { [key]: { $id: 'parent/', $defs: {
        target: { $id: identity, type: 'number', minimum: 2 },
        link: { $id: 'link', $ref: identity },
      } } },
    };
    const address = new URL(identity, 'https://example.test/schemas/parent/').href;
    for (const reference of [address, 'https://example.test/schemas/parent/link']) {
      for (const payload of [...payloads, 1, 2]) add(`nested-resource-${cases.length}`, { $ref: reference }, payload, { externalRefs: [document] });
    }
  }
}
const source = { $id: 'https://example.test/source', $defs: { value: { $id: 'number', type: 'number' } } };
const bridge = { $id: 'https://example.test/bridge', $defs: { value: { $id: 'alias', $ref: 'number' } } };
for (const refs of [[source, bridge], [bridge, source]]) {
  for (const payload of payloads) add(`cross-document-${cases.length}`, { $ref: 'https://example.test/alias' }, payload, { externalRefs: refs });
}
for (const type of ['number', 'string']) {
  const documents = [
    { $id: 'https://example.test/first', $defs: { value: { $id: 'shared', type: 'number' } } },
    { $id: 'https://example.test/second', $defs: { value: { $id: 'shared', type } } },
  ];
  for (const refs of [documents, [...documents].reverse()]) {
    for (const payload of payloads) add(`ordered-alias-${cases.length}`, { $ref: 'https://example.test/shared' }, payload, { externalRefs: refs });
  }
}
writeFileSync('../../validation/testdata/graphs-v2.35.0.json', JSON.stringify({ version: '2.35.0', ajvVersion: '8.20.0', cases }, null, 2) + '\n');
console.log(`${cases.length} validation graph cases`);
