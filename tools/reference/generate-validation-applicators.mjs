// Preserve the pinned utility's complete diagnostics and declaration order.
import { writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';
import Ajv from 'ajv';

const cases = [];
function add(name, schema, payload) {
  const item = { name, schema, payload };
  try {
    const value = validate({ schema, payload: structuredClone(payload), ajv: new Ajv({ allErrors: true, logger: false }) });
    item.expected = { success: true, value };
  } catch (error) {
    item.expected = { success: false, error: error.name, message: error.message };
    if (Array.isArray(error.cause)) item.expected.issues = error.cause;
  }
  cases.push(item);
}

const schemas = {
  dependency: { dependencies: { card: ['zip', 'name'] } },
  dependencyOrder: { dependencies: { zebra: ['last', 'first'], alpha: ['left', 'right'] } },
  schemaDependency: { dependencies: { card: { required: ['zip'], properties: { zip: { type: 'integer' } } } } },
  mixedDependencies: { dependencies: { zebra: { required: ['tail'] }, card: ['zip', 'name'], alpha: { const: {} } } },
  propertyNames: { propertyNames: { minLength: 3, pattern: '^[a-z]+$' } },
  propertyNamesFalse: { propertyNames: false },
  propertyNamesComposite: { propertyNames: { anyOf: [{ const: 'allowed' }, { pattern: '^x', minLength: 4 }] } },
  objectOrder: { type: 'object', required: ['missing'], propertyNames: { pattern: '^[a-z]+$' }, additionalProperties: false,
    dependencies: { zebra: ['tail'] }, properties: { zebra: { type: 'string', minLength: 2 }, alpha: { type: 'integer' } } },
  patternOrder: { properties: { zebra: { type: 'integer' }, alpha: { type: 'integer' } },
    patternProperties: { 'a$': { type: 'string', minLength: 4 }, '^a': { const: 'allowed' } } },
  tuple: { items: [{ type: 'integer' }, { type: 'string' }], additionalItems: false },
  tupleAdditional: { items: [{ const: 1 }], additionalItems: { type: 'number', minimum: 5 } },
  arrayOrder: { maxItems: 1, items: [{ type: 'integer' }, { type: 'string' }], additionalItems: false,
    contains: { type: 'number', minimum: 10 }, uniqueItems: true },
  conditional: { if: { type: 'number' }, then: { minimum: 5 }, else: { type: 'string', minLength: 3 } },
  conditionalFalse: { if: { type: 'number' }, then: false, else: false },
  conditionalNested: { if: { type: 'object' }, then: { if: { required: ['card'] }, then: { required: ['zip'] }, else: false }, else: { type: 'null' } },
  conditionalOrder: { const: 'ok', anyOf: [{ type: 'string' }, { type: 'number' }],
    if: { type: 'string' }, then: { minLength: 5 }, else: { minimum: 9 }, type: 'string', minLength: 4 },
  allOf: { allOf: [{ type: 'string', minLength: 4 }, { type: 'number', minimum: 5 }], enum: ['allowed'] },
  anyOf: { anyOf: [{ required: ['left'], properties: { zebra: { type: 'integer' } } }, { type: 'string', minLength: 4 }], const: 'ok' },
  oneOf: { oneOf: [{ type: 'string' }, { type: 'number' }, { type: 'integer' }] },
  nested: { properties: { zebra: { propertyNames: { pattern: '^x' }, dependencies: { card: ['zip', 'name'] } }, alpha: { items: [{ type: 'number' }], additionalItems: false } } },
  refConditional: { definitions: { branch: { if: { type: 'number' }, then: { minimum: 5 }, else: false } }, $ref: '#/definitions/branch' },
};
const inputs = [null, false, 0, 6, '', 'a', 'allowed', [], [false, 1, 1], [1, 'ok'], [1, 2, 3], {},
  { card: true }, { card: true, zip: 'wrong' }, { zebra: false, alpha: false },
  { zebra: '', alpha: 'bad', 'BAD/~': 1 }, { '': 0 }, { '2': 1, '10': 1, '01': 1, 'Z': 1 },
  { zebra: { card: true, 'BAD/~': 1 }, alpha: [false, 2] }];
for (const [name, schema] of Object.entries(schemas)) {
  for (let i = 0; i < inputs.length; i++) add(`${name}-${i}`, schema, inputs[i]);
  for (let i = 0; i < inputs.length; i++) {
    add(`${name}-nested-${i}`, { properties: { 'name/~': schema } }, { 'name/~': inputs[i] });
    add(`${name}-items-${i}`, { items: schema }, [inputs[i], inputs[(i + 1) % inputs.length]]);
  }
}
add('pattern-declaration-order', { patternProperties: { 'a$': { minLength: 4 }, '^a': { const: 'allowed' } } }, { zebra: '', alpha: 'bad' });
add('pattern-always-valid', { properties: { a: { type: 'integer' } }, patternProperties: { '^a': true } }, { a: 'bad' });
add('oneOf-failures-between-matches', { oneOf: [true, { type: 'string' }, { const: 99 }, true, false] }, 1);
add('oneOf-no-failures-after-second-match', { oneOf: [true, true, false] }, 1);
add('nested-property-names', { items: { properties: { zebra: { propertyNames: false }, alpha: { propertyNames: false } } } }, [{ zebra: { a: 1 }, alpha: { b: 1 } }, { zebra: { c: 1 }, alpha: { d: 1 } }]);
add('reference-reused', { definitions: { value: { if: { type: 'string' }, then: { minLength: 3 }, else: false } }, properties: { zebra: { $ref: '#/definitions/value' }, alpha: { $ref: '#/definitions/value' } } }, { alpha: 1, zebra: '' });
for (const schema of [{ if: true }, { then: false }, { else: false }, { additionalItems: false }]) {
  add(`strict-${cases.length}`, schema, []);
}
writeFileSync('../../validation/testdata/applicators-v2.35.0.json', JSON.stringify({ version: '2.35.0', ajvVersion: '8.20.0', cases }, null, 2) + '\n');
console.log(`${cases.length} validation applicator cases`);
