// Capture exact diagnostics and EMF from the pinned Metrics implementation.
import { Metrics } from '@aws-lambda-powertools/metrics';
import { mkdirSync, writeFileSync } from 'node:fs';

const now = 1789992000000;
const originalNow = Date.now;
const originalWrite = process.stdout.write;
const cases = [];
for (const key of ['POWERTOOLS_METRICS_NAMESPACE', 'POWERTOOLS_METRICS_DISABLED', 'POWERTOOLS_DEV', 'POWERTOOLS_SERVICE_NAME']) delete process.env[key];
const normalize = (value) => {
  const result = JSON.parse(JSON.stringify(value));
  for (const directive of result._aws.CloudWatchMetrics) for (const dimensions of directive.Dimensions) dimensions.sort();
  return result;
};
function run(name, options, steps) {
  process.env.POWERTOOLS_METRICS_DISABLED = String(options.disabled);
  const warnings = [], emitted = [], results = [];
  process.stdout.write = (chunk) => { emitted.push(normalize(JSON.parse(chunk.toString()))); return true; };
  const metric = new Metrics({ namespace: options.namespace, serviceName: 'orders', defaultDimensions: options.defaults, logger: { warn(message) { warnings.push(message); } } });
  metric.setThrowOnEmptyMetrics(options.required);
  for (const [operation, ...args] of steps) {
    try {
      const value = metric[operation](...args);
      results.push({ value: operation === 'serializeMetrics' ? normalize(value) : null, error: null });
    } catch (error) {
      results.push({ value: null, error: error.message });
    }
  }
  cases.push({ name, ...options, steps, results, warnings, emitted });
}
Date.now = () => now;
try {
  for (const disabled of [false, true]) for (const required of [false, true]) for (const namespace of ['', 'Warnings']) {
    const options = { disabled, required, namespace, defaults: {} };
    const prefix = `${disabled}-${required}-${namespace}`;
    run(`${prefix}-empty`, options, [['serializeMetrics'], ['publishStoredMetrics'], ['publishStoredMetrics']]);
    for (const [index, value] of [' ', '\t\n', '\ufeff', '\u00a0', '\u2003', '\u0085', '\u200b', ' value '].entries()) {
      run(`${prefix}-whitespace-${index}`, options, [['addDimension', 'input', value], ['addDimension', value, 'name'], ['addMetric', 'Count', 'Count', 1], ['serializeMetrics']]);
    }
    run(`${prefix}-invalid`, options, [['addDimension', '', 'x'], ['addDimension', 'empty', ''], ['addDimension', 'space', ' '], ['addDimensions', { a: '', b: 'valid' }], ['setDefaultDimensions', { c: '', d: 'valid' }], ['addMetric', 'Count', 'Count', 1], ['serializeMetrics'], ['publishStoredMetrics']]);
    run(`${prefix}-duplicate`, options, [['addDimension', 'service', 'request'], ['addDimension', 'service', 'again'], ['addDimensions', { service: 'set' }], ['setDefaultDimensions', { service: 'default' }], ['addMetric', 'Count', 'Count', 1], ['serializeMetrics'], ['publishStoredMetrics']]);
    run(`${prefix}-collision`, options, [['addMetadata', 'service', 'context'], ['addDimension', 'service', 'request'], ['addDimensions', { service: 'set1' }], ['addDimensions', { service: 'set2' }], ['addMetric', 'Count', 'Count', 1], ['serializeMetrics'], ['publishStoredMetrics']]);
    for (const source of ['metadata', 'default', 'dimension', 'set']) {
      const add = { metadata: ['addMetadata', 'Count', 'context'], default: ['setDefaultDimensions', { Count: 'default' }], dimension: ['addDimension', 'Count', 'request'], set: ['addDimensions', { Count: 'set' }] }[source];
      run(`${prefix}-metric-${source}`, options, [add, ['addMetric', 'Count', 'Count', 1], ['serializeMetrics'], ['publishStoredMetrics'], ['serializeMetrics']]);
    }
    for (const offset of [-1209600001, -1209600000, -1209599999, 0, 7199999, 7200000, 7200001]) {
      run(`${prefix}-timestamp-${offset}`, options, [['setTimestamp', now + offset], ['addMetric', 'Count', 'Count', 1], ['serializeMetrics'], ['publishStoredMetrics'], ['serializeMetrics']]);
    }
    for (const defaults of [{ a: '', b: '' }, { service: '' }]) {
      run(`${prefix}-constructor-${Object.keys(defaults).join('-')}`, { ...options, defaults }, [['addMetric', 'Count', 'Count', 1], ['serializeMetrics']]);
    }
  }
  const defaults = Object.fromEntries(Array.from({ length: 28 }, (_, i) => [`d${String(i).padStart(2, '0')}`, 'default']));
  for (const operation of ['addDimension', 'addDimensions', 'setDefaultDimensions']) {
    const action = operation === 'addDimension' ? [operation, 'overflow', 'value'] : [operation, { invalid: '', overflow: 'value', service: 'duplicate' }];
    run(`limit-${operation}`, { disabled: false, required: false, namespace: 'Warnings', defaults }, [action, ['addMetric', 'Count', 'Count', 1], ['serializeMetrics']]);
  }
} finally { Date.now = originalNow; process.stdout.write = originalWrite; }
const directory = new URL('../../metrics/testdata/', import.meta.url);
mkdirSync(directory, { recursive: true });
writeFileSync(new URL('warnings-v2.35.0.json', directory), JSON.stringify({ upstream: '2.35.0', now, normalization: 'Inject Date.now and sort dimension-name arrays. Compare warnings, errors and full documents exactly.', cases }, null, 2) + '\n');
console.log(`${cases.length} Metrics diagnostic reference cases`);
