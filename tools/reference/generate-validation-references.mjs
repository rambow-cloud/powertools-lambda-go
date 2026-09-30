// Exercise rule-group ordering and reference locations without normalizing errors.
import { writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';
import Ajv from 'ajv';

const cases = [];
function add(name, schema, payload, options = {}) {
  const item = { name, schema: structuredClone(schema), payload, options: structuredClone(options) };
  try {
    const value = validate({ schema: structuredClone(schema), payload: structuredClone(payload), ...structuredClone(options),
      formats: options.formats ? { startsA: value => value.startsWith('a'), even: { type: 'number', validate: value => value % 2 === 0 } } : undefined,
      ajv: new Ajv({ allErrors: true, logger: false }) });
    item.expected = { success: true, value };
  } catch (error) {
    item.expected = { success: false, error: error.name, message: error.message };
    if (Array.isArray(error.cause)) item.expected.issues = error.cause;
  }
  cases.push(item);
}
const inputs = [null, false, 0, 1, 1.5, '', 'bad', [], [false], {}, { child: false }, { child: { child: false } }];
const rules = { const: 'allowed', enum: ['allowed'], minimum: 5, multipleOf: 2, minLength: 4, pattern: '^a', minItems: 2, items: { type: 'integer' }, minProperties: 2, required: ['missing'], properties: { child: { type: 'string' } } };
for (const type of ['number', 'integer', 'string', 'array', 'object', 'boolean', 'null']) {
  for (const declaration of [type, [type], [type, 'null']]) {
    for (const nullable of [undefined, true, false]) {
      const schema = { type: declaration, ...rules };
      if (nullable !== undefined) schema.nullable = nullable;
      for (let i = 0; i < inputs.length; i++) add(`mixed-${cases.length}`, schema, inputs[i]);
    }
  }
}
for (const type of ['number', 'integer', 'string', 'array', 'object']) {
  for (const format of ['startsA', 'even']) {
    for (const payload of inputs) add(`format-${cases.length}`, { type, ...rules, format }, payload, { formats: true });
  }
}

const refs = {
  local: { $defs: { value: { type: 'string', minLength: 4 } }, $ref: '#/$defs/value' },
  rootId: { $id: 'https://example.test/root', type: 'object', properties: { child: { type: 'string' } } },
  rootIdLocal: { $id: 'https://example.test/root', $defs: { value: { type: 'string' } }, $ref: '#/$defs/value' },
  nestedId: { $defs: { value: { $id: 'https://example.test/value', type: 'string', minLength: 4 } }, $ref: 'https://example.test/value' },
  nestedRelativeId: { $id: 'https://example.test/root', $defs: { value: { $id: 'value', type: 'string', minLength: 4 } }, $ref: 'value' },
  pointerNestedId: { $defs: { value: { $id: 'https://example.test/value', type: 'string' } }, $ref: '#/$defs/value' },
  escapedPointer: { $defs: { 'value/%~ α': { type: 'string' } }, $ref: '#/$defs/value~1%25~0%20%CE%B1' },
  percentName: { properties: { '%2F': { type: 'string' }, 'a%20b': { type: 'string' } } },
  chain: { $defs: { first: { $ref: '#/$defs/second' }, second: { type: 'string', minLength: 4 } }, $ref: '#/$defs/first' },
  chainSiblings: { $defs: { first: { $ref: '#/$defs/second', const: 'allowed' }, second: { type: 'string', minLength: 4 } }, $ref: '#/$defs/first', enum: ['allowed'] },
  recursive: { type: 'object', properties: { child: { $ref: '#' } }, required: ['child'] },
  recursiveId: { $id: 'https://example.test/tree', type: 'object', properties: { child: { $ref: 'https://example.test/tree' } }, required: ['child'] },
  recursiveDef: { $defs: { node: { type: 'object', properties: { child: { $ref: '#/$defs/node' } }, required: ['child'] } }, $ref: '#/$defs/node' },
  relativePointer: { $id: 'https://example.test/root', $defs: { value: { type: 'string' } }, $ref: 'root#/$defs/value' },
};
for (const [name, schema] of Object.entries(refs)) {
  for (let i = 0; i < inputs.length; i++) add(`${name}-${i}`, schema, inputs[i]);
}
for (const external of [
  { $id: 'https://example.test/value', type: 'string', minLength: 4 },
  { $id: 'https://example.test/value', $defs: { value: { type: 'string' } }, $ref: '#/$defs/value' },
  { $id: 'https://example.test/value', type: 'object', properties: { child: { $ref: 'https://example.test/value' } }, required: ['child'] },
]) {
  for (const payload of inputs) add(`external-${cases.length}`, { properties: { child: { $ref: 'https://example.test/value' } } }, { child: payload }, { externalRefs: [external] });
}
add('percent-property-names', refs.percentName, { '%2F': false, 'a%20b': false });
const library = {
  $id: 'https://example.test/library',
  $defs: {
    leaf: { type: 'string', minLength: 4 },
    alias: { $ref: '#/$defs/leaf' },
    constrained: { $ref: '#/$defs/leaf', const: 'allowed' },
  },
};
for (const target of ['#/$defs/leaf', 'https://example.test/library#/$defs/leaf', 'https://example.test/library#/$defs/alias', 'https://example.test/library#/$defs/constrained', 'https://example.test/leaf']) {
  for (const sibling of [{}, { const: 'allowed' }, { description: 'Alias metadata' }]) {
    const schema = { $id: 'https://example.test/root', $defs: { leaf: { type: 'string' }, alias: { $ref: target, ...sibling } }, properties: { child: { $ref: '#/$defs/alias' } } };
    for (const payload of inputs) add(`chain-matrix-${cases.length}`, schema, { child: payload }, { externalRefs: [library, { $id: 'https://example.test/leaf', type: 'string' }] });
  }
}
for (const annotation of [
  { const: { $ref: 'data' } },
  { examples: [{ $ref: 'data' }] },
  { default: { $ref: 'data' } },
  { $defs: { unused: { $ref: '#/$defs/leaf' } } },
]) {
  const schema = { $defs: { leaf: { type: 'string' }, value: { type: 'string', ...annotation } }, $ref: '#/$defs/value' };
  add(`reference-annotation-${cases.length}`, schema, false);
}
for (const name of ['%2F', '%252F', '#', 'then', 'α/~', '']) {
  const escaped = encodeURIComponent(name.replace(/~/g, '~0').replace(/\//g, '~1'));
  add(`reference-name-${cases.length}`, { $defs: { [name]: { type: ['string'], minLength: 4 } }, $ref: `#/$defs/${escaped}` }, false);
}
writeFileSync('../../validation/testdata/references-v2.35.0.json', JSON.stringify({ version: '2.35.0', ajvVersion: '8.20.0', cases }, null, 2) + '\n');
console.log(`${cases.length} validation rule-group and reference cases`);
