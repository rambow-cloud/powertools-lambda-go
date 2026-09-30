import { mkdirSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
const { DataMasking } = createRequire(import.meta.url)('@aws-lambda-powertools/data-masking');
if (process.versions.unicode !== '16.0') throw new Error('The reference requires Unicode 16.0');
// Code units preserve lone surrogates that JSON decoders would otherwise replace.
const units = text => Array.from({ length: text.length }, (_, index) => text.charCodeAt(index));
const cases = [];
function run(pattern, flags, input, format, lastIndex = 0) {
  const expression = new RegExp(pattern, flags);
  expression.lastIndex = lastIndex;
  const result = input.replace(expression, format);
  cases.push({ pattern, flags, input: units(input), format: units(format), lastIndex, result: units(result), finalIndex: expression.lastIndex });
}
const patterns = ['', '.', '.*', '.+?', '^|$', '^.$', '(a)?(b)', '(?<first>a)(b)?', '(?<x>a)|(?<y>b)', '(?<=a)b', '(?<!a)b', '(?=a)', '(a*)', '[^]', '[a-z]+', '\\b\\w+\\b', '(a)\\1', '\\uD83D\\uDE00', '\\d+'];
const inputs = ['', 'a', 'aba b', 'aaab12', '😀a😀', '\r\na\u2028b\u2029c', '\ud800a\udfff'];
const flags = ['', 'g', 'u', 'gu', 'i', 'gi', 'gm', 'gs', 'y', 'gy', 'uy'];
const formats = ['#', '$$:$&:$`:$\'', '$1/$2/$01/$10/$99/$0', '$<first>:$<missing>:$<x>:$<y>', '😀$&\ud800'];
for (const pattern of patterns) for (const flag of flags) for (const input of inputs) for (const format of formats) run(pattern, flag, input, format, 1);
for (const flag of ['y', 'uy', 'gy', 'guy', '', 'g']) for (const index of [-3, 0, 1, 2, 3, 4, 5, 99]) for (const pattern of ['.', '', 'a']) run(pattern, flag, '😀ab', '$&!', index);
for (const pattern of ['\\p{Script=Greek}+', '\\P{ASCII}+', '(?<letter>\\p{Letter}+)', '\\u{1F600}', '[\\u{1F600}-\\u{1F64F}]']) for (const flag of ['u', 'gu', 'giu']) run(pattern, flag, 'Aαβ😀ſK', '[$&:$<letter>]');
for (const flag of ['', 'g', 'u', 'gu', 'i', 'iu', 'gi', 'giu']) for (const pattern of ['[a-z]+', '\\w+', '\\b', '\\B', '[sSkK]']) run(pattern, flag, 'ſKSskK', '#');
for (const pattern of ['\\141', '\\x61', '\\u0061', '\\cA', '\\8', '\\u{61}', '[^a-z]', '[a-z\\W]', '[^a-z\\W]', '(a)\\1', '(?<word>a)\\k<word>', 'ß|ẞ|σ|ς|Σ|ı|i|I|İ', '\\p{ASCII}', '(a)?b\\1']) for (const flag of ['', 'i', 'gi']) for (const input of ['aAaBbb8u{61}', '\x01ßẞσςΣıiIİ', 'ſKéÉ😀']) run(pattern, flag, input, '$&/$1/$<word>');
for (const count of [1, 9, 10, 99]) run('(a)'.repeat(count), 'd', 'a'.repeat(count), '$01/$09/$10/$99/$100/$00/$<1>');
for (const escape of ['A', 'Z', 'z', 'G', 'R', 'e', 'K', 'C', 'q']) for (const flag of ['', 'i', 'gi']) run('\\' + escape, flag, 'AZzGReKCqazgrekcq\n\r', '#');
const invalid = ['(', '[', '(?>a)', '(?i)a', '(?#x)a', '(?P<x>a)', '\\p{NoSuchProperty}', '\\8', '[\\d-a]', 'a{', '\\q'].map(pattern => ({ pattern, flags: 'u' }));
invalid.push(...['gg', 'uu', 'z', 'uv'].map(flags => ({ pattern: '.', flags })));
invalid.push(...['(?>a)', '(?i)a', '(?#x)a', '(?P<x>a)'].map(pattern => ({ pattern, flags: '' })));
for (const spec of invalid) {
  try { new RegExp(spec.pattern, spec.flags); throw new Error('Expected invalid pattern'); }
  catch (error) { if (!(error instanceof SyntaxError)) throw error; }
}
const masking = [];
for (const flag of ['', 'g', 'y', 'gy', 'uy']) for (const pattern of ['.', '(?<word>[a-z]+)', '(?=a)']) for (const fields of [false, true]) {
  const expression = new RegExp(pattern, flag);
  expression.lastIndex = 1;
  const data = { first: 'abc', nested: ['abc', 'ba', 12, null, false], last: 'abc' };
  const rule = { regexPattern: expression, maskFormat: '<$&:$<word>>', customMask: 'ignored', dynamicMask: true };
  const options = fields ? { fields: ['first', 'nested[*]'], maskingRules: { first: rule, 'nested[*]': rule } } : rule;
  const result = new DataMasking().erase(data, options);
  masking.push({ pattern, flags: flag, lastIndex: 1, format: rule.maskFormat, data, fields, result, finalIndex: expression.lastIndex });
}
mkdirSync('../../commons/regex/testdata', { recursive: true });
writeFileSync('../../commons/regex/testdata/node.json', JSON.stringify({ node: process.versions.node, unicode: process.versions.unicode, cases, invalid }) + '\n');
mkdirSync('../../integration/maskingregex/testdata', { recursive: true });
writeFileSync('../../integration/maskingregex/testdata/typescript-v2.35.0.json', JSON.stringify({ version: '2.35.0', cases: masking }) + '\n');
console.log(`Wrote ${cases.length} replacements, ${invalid.length} invalid patterns and ${masking.length} Data Masking scenarios`);

const canonical = {};
for (let unit = 0; unit <= 0xffff; unit++) {
  const upper = String.fromCharCode(unit).toUpperCase();
  if (upper.length === 1 && (unit < 128 || upper.charCodeAt(0) >= 128) && upper.charCodeAt(0) !== unit) canonical[unit] = upper.charCodeAt(0);
}
writeFileSync('../../commons/regex/legacy_fold.json', JSON.stringify(canonical) + '\n');
