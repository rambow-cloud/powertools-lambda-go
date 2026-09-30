// Separate original-document metaschema checks from reachable compilation rules.
import { writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';
import Ajv from 'ajv';

const cases = [];
function add(name, schema, payload, options = {}) {
  const item = { name, schema: structuredClone(schema), payload, options: structuredClone(options) };
  try {
    const value = validate({ schema: structuredClone(schema), payload: structuredClone(payload),
      ...structuredClone(options), ajv: new Ajv({ allErrors: true, logger: false }) });
    item.expected = { success: true, value };
  } catch (error) {
    item.expected = { success: false, error: error.name, message: error.message };
    if (Array.isArray(error.cause)) item.expected.issues = error.cause;
  }
  cases.push(item);
}

const definitions = {
  unknown: { typo: true }, format: { format: 'email' }, type: { type: 'bogus' },
  nullable: { nullable: true }, nullableType: { type: 'string', nullable: 'yes' },
  negativeLength: { minLength: -1 }, pattern: { pattern: '(?i)a' },
  condition: { if: { const: 1 } }, then: { then: false },
  additionalItems: { additionalItems: false },
  nestedUnknown: { properties: { child: { typo: true } } },
  nestedPattern: { properties: { child: { pattern: '(?i)a' } } },
  nestedInvalidType: { properties: { child: { type: 'bogus' } } },
  valid: { type: 'string', minLength: 2 },
};
const payloads = ['value', '', 0, { child: 'value' }];
for (const container of ['definitions', '$defs']) {
  for (const [name, body] of Object.entries(definitions)) {
    for (const reach of ['unused', 'pointer', 'chain', 'id']) {
      const schema = { [container]: { value: structuredClone(body) } };
      if (reach === 'pointer') schema.$ref = `#/${container}/value`;
      if (reach === 'chain') {
        schema[container].alias = { $ref: `#/${container}/value` };
        schema.$ref = `#/${container}/alias`;
      }
      if (reach === 'id') {
        schema[container].value.$id = 'https://example.test/value';
        schema.$ref = 'https://example.test/value';
      }
      for (let i = 0; i < payloads.length; i++) add(`${container}-${name}-${reach}-${i}`, schema, payloads[i]);
    }
  }
}
for (const pattern of ['(?i)a', '[', '^a$']) {
  for (const body of [true, { title: 'Annotation' }, { type: 'string' }]) {
    for (const additionalProperties of [undefined, true, false, { type: 'number' }]) {
      const schema = { patternProperties: { [pattern]: body } };
      if (additionalProperties !== undefined) schema.additionalProperties = additionalProperties;
      for (const payload of [{}, { a: 'value' }, { b: 1 }]) add(`patterns-${cases.length}`, schema, payload);
    }
  }
}
for (const [name, body] of Object.entries(definitions)) {
  for (const unused of [true, false]) {
    // Invalid external document structure is rejected by addSchema outside the
    // reference's compilation catch. Keep that separate error contract explicit.
    if (['type', 'negativeLength', 'nestedInvalidType'].includes(name)) continue;
    const reference = { $id: 'https://example.test/external', ...structuredClone(body) };
    const schema = unused ? { type: 'string' } : { $ref: reference.$id };
    for (let i = 0; i < payloads.length; i++) add(`external-${name}-${unused}-${i}`, schema, payloads[i], { externalRefs: [reference] });
  }
}
for (const body of [{ type: 1 }, { minLength: -1 }, { minLength: 1.5 }, { required: 'child' }, { enum: [] }, { pattern: 1 }, { allOf: [] }]) {
  // Structural validation must still reject invalid unused Draft 7 definitions.
  add(`structure-${cases.length}`, { definitions: { value: body } }, 'value');
  add(`structure-ref-sibling-${cases.length}`, { definitions: { value: body, valid: true }, $ref: '#/definitions/valid' }, 'value');
}
writeFileSync('../../validation/testdata/setup-v2.35.0.json', JSON.stringify({ version: '2.35.0', ajvVersion: '8.20.0', cases }, null, 2) + '\n');
console.log(`${cases.length} validation schema-setup cases`);
