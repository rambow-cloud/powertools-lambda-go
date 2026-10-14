import { DataMasking } from '@aws-lambda-powertools/data-masking';
import { writeFileSync } from 'node:fs';

const cases = [];
for (const [name, method, fields, data] of [
  ['whole undefined', 'encrypt', null, undefined],
  ['selected undefined', 'encrypt', [], undefined],
  ['decrypt undefined', 'decrypt', [], undefined],
  ['duplicate selectors', 'encrypt', ['secret', 'secret'], { secret: 'original' }],
]) {
  const calls = [];
  const provider = {
    async encrypt(value) { calls.push({ type: typeof value, value: value ?? null }); return `cipher:${value}`; },
    async decrypt(value) { calls.push({ type: typeof value, value }); return value; },
  };
  const masker = new DataMasking({ provider });
  const value = await masker[method](data, fields === null ? {} : { fields });
  cases.push({ name, method, fields, value: value === undefined ? { $: 'undefined' } : value, calls,
    classification: name === 'whole undefined' ? 'Deliberate Go difference: string-only providers reject undefined plaintext before the call.' : 'Verified marker, selector and captured-plaintext boundary.' });
}
writeFileSync(new URL('../../datamasking/testdata/boundaries-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', cases }, null, 2) + '\n');
console.log(`${cases.length} exact masking boundary observations`);
