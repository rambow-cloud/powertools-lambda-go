// Record numeric serialization, exact metric diagnostics and object-key behavior.
import { Metrics, MetricUnit } from '@aws-lambda-powertools/metrics';
import { writeFileSync } from 'node:fs';

const now = 1789992000000;
const originalNow = Date.now;
const originalWrite = process.stdout.write;
const keys = ['POWERTOOLS_METRICS_NAMESPACE', 'POWERTOOLS_SERVICE_NAME', 'POWERTOOLS_METRICS_DISABLED', 'POWERTOOLS_DEV', 'AWS_LAMBDA_MAX_CONCURRENCY'];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));
const cases = [];
const special = (number) => ({ number });
const number = (value) => typeof value === 'object' ? ({ NaN: NaN, Infinity: Infinity, '-Infinity': -Infinity, '-0': -0 })[value.number] : value;
function normalize(value) {
  const result = JSON.parse(JSON.stringify(value));
  if (Array.isArray(result?._aws?.CloudWatchMetrics)) for (const directive of result._aws.CloudWatchMetrics) {
    for (const dimensions of directive.Dimensions ?? []) dimensions.sort();
  }
  return result;
}
function run(name, options, steps) {
  for (const key of keys) delete process.env[key];
  process.env.POWERTOOLS_METRICS_DISABLED = String(options.disabled);
  const emitted = [], warnings = [], results = [];
  process.stdout.write = (chunk) => { emitted.push(normalize(JSON.parse(chunk.toString()))); return true; };
  const m = new Metrics({ namespace: 'Values', serviceName: 'orders', singleMetric: options.single, logger: { warn(message) { warnings.push(message); } } });
  m.setThrowOnEmptyMetrics(options.required);
  for (const [operation, ...args] of steps) {
    try {
      let value;
      if (operation === 'repeatMetric') {
        for (let i = 0; i < args[0]; i++) m.addMetric(args[1], args[2], number(args[3]));
      } else if (operation === 'fill') {
        for (let i = args[0]; i > 0; i--) m.addMetric(String(i), MetricUnit.Count, i);
      } else if (operation === 'addMetric') {
        value = m.addMetric(args[0], args[1], number(args[2]), ...args.slice(3));
      } else value = m[operation](...args);
      results.push({ value: operation === 'serializeMetrics' ? normalize(value) : operation === 'hasStoredMetrics' ? value : null, error: null });
    } catch (error) { results.push({ value: null, error: error.message }); }
  }
  cases.push({ name, ...options, steps, results, emitted, warnings });
}
try {
  Date.now = () => now;
  for (const disabled of [false, true]) for (const required of [false, true]) for (const single of [false, true]) {
    const options = { disabled, required, single };
    const finish = [['serializeMetrics'], ['publishStoredMetrics'], ['hasStoredMetrics'], ['serializeMetrics']];
    const names = ['', ' ', '\n', 'x'.repeat(255), 'x'.repeat(256), '😀'.repeat(127) + 'x', '😀'.repeat(128), '_aws', ...Object.getOwnPropertyNames(Object.prototype)];
    for (const [i, name] of names.entries()) run(`name-${i}`, options, [['addMetric', name, 'Count', 1], ...finish]);
    for (const [i, value] of [0, 1, -1, 0.5, 1e-7, 1e-6, 1e21, 1e308, 5e-324, 9007199254740992, special('NaN'), special('Infinity'), special('-Infinity'), special('-0')].entries()) {
      run(`number-${i}`, options, [['addMetric', 'Value', 'Count', value], ['serializeMetrics'], ['addMetric', 'Value', 'Count', 2, 1], ...finish]);
    }
    for (const unit of ['', 'bad', 'count', 'Count ', "'quote\n"]) run(`unit-${unit}`, options, [['addMetric', 'Value', unit, 1], ...finish]);
    for (const resolution of [-1, 0, 10, 256]) run(`resolution-${resolution}`, options, [['addMetric', 'Value', 'Count', 1, resolution], ...finish]);
    run('conflicting-unit', options, [['addMetric', 'line\n"quoted', 'Count', 1], ['addMetric', 'line\n"quoted', 'Bytes', 2], ...finish]);
    const ordered = ['10', '4294967295', '1', '01', '0', '2', '4294967294', '-0', 'a', '+1', '1.0', '000'];
    run('numeric-name-order', options, [...ordered.map((key) => ['addMetric', key, 'Count', 1]), ...finish]);
    for (const key of ['_aws', '__proto__', 'constructor', 'toString', 'prototype']) {
      const pair = Object.fromEntries([[key, 'dimension']]);
      for (const [source, add] of [
        ['metadata', ['addMetadata', key, { marker: 'metadata' }]],
        ['dimension', ['addDimension', key, 'dimension']],
        ['default', ['setDefaultDimensions', pair]],
        ['set', ['addDimensions', pair]],
      ]) run(`${source}-${key}`, options, [add, ['addMetric', 'Count', 'Count', 1], ...finish]);
    }
    run('reserved-collision', options, [['addMetadata', '_aws', 'metadata'], ['addDimension', '_aws', 'dimension'], ['addMetric', '_aws', 'Count', 1], ...finish]);
    run('hundred-values', options, [['repeatMetric', 100, 'Value', 'Count', special('NaN')], ...finish]);
    run('hundred-names-error', options, [['fill', 100], ['addMetric', 'constructor', 'Count', 1], ...finish]);
    run('hundred-names-reserved', options, [['fill', 100], ['addMetric', '_aws', 'Count', special('NaN')], ...finish]);
    run('retained-after-error', options, [['repeatMetric', 99, 'Value', 'Count', special('Infinity')], ['addMetric', 'Value', 'Bytes', 2], ['addMetric', 'Value', 'Count', special('-Infinity')], ...finish]);
  }
  for (const unit of Object.values(MetricUnit)) for (const resolution of [1, 60, 99]) run(`unit-resolution-${unit}-${resolution}`, { disabled: false, required: false, single: false }, [['addMetric', 'Value', unit, 1, resolution], ['serializeMetrics'], ['publishStoredMetrics']]);
} finally {
  Date.now = originalNow;
  process.stdout.write = originalWrite;
  for (const [key, value] of Object.entries(original)) { if (value === undefined) delete process.env[key]; else process.env[key] = value; }
}
writeFileSync(new URL('../../metrics/testdata/values-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', now, normalization: 'Inject Date.now and sort dimension-name arrays only. Preserve full metric definitions, values, warning strings and error messages. Special input tags represent non-finite numbers and negative zero.', cases }, null, 2) + '\n');
console.log(`${cases.length} Metrics value reference cases`);
