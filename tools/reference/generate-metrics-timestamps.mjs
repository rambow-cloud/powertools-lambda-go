// Compare numeric and Date timestamp conversion, warnings and lazy clock reads.
import { Metrics } from '@aws-lambda-powertools/metrics';
import { writeFileSync } from 'node:fs';

const now = 1789992000000;
const originalNow = Date.now;
const originalWrite = process.stdout.write;
const keys = ['POWERTOOLS_METRICS_NAMESPACE', 'POWERTOOLS_SERVICE_NAME', 'POWERTOOLS_METRICS_DISABLED', 'POWERTOOLS_DEV'];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));
const cases = [];
const special = (number) => ({ number });
const numeric = (value) => typeof value === 'object' ? ({ NaN: NaN, Infinity: Infinity, '-Infinity': -Infinity, '-0': -0 })[value.number] : value;
const normalize = (value) => {
  const result = JSON.parse(JSON.stringify(value));
  for (const directive of result._aws.CloudWatchMetrics) for (const dimension of directive.Dimensions) dimension.sort();
  return result;
};
function run(clock, kind, value, disabled, required, lifecycle) {
  for (const key of keys) delete process.env[key];
  process.env.POWERTOOLS_METRICS_DISABLED = String(disabled);
  let clockCalls = 0;
  Date.now = () => { clockCalls++; return clock; };
  const warnings = [], emitted = [], results = [];
  process.stdout.write = (chunk) => { emitted.push(normalize(JSON.parse(chunk.toString()))); return true; };
  const parent = new Metrics({ namespace: 'Timestamp', serviceName: 'orders', logger: { warn(message) { warnings.push(message); } } });
  parent.setThrowOnEmptyMetrics(required);
  let metric = parent;
  const input = kind === 'number' ? numeric(value) : kind === 'date' ? new Date(numeric(value)) : new Date(value);
  const steps = ['setTimestamp', 'addMetric', 'serializeMetrics', ...(lifecycle === 'none' ? [] : [lifecycle]), 'serializeMetrics', 'addMetric', 'publishStoredMetrics', 'serializeMetrics', 'publishParent'];
  for (const operation of steps) {
    try {
      let result;
      if (operation === 'setTimestamp') metric.setTimestamp(input);
      else if (operation === 'addMetric') metric.addMetric('Count', 'Count', 1);
      else if (operation === 'singleMetric') metric = metric.singleMetric();
      else if (operation === 'publishParent') parent.publishStoredMetrics();
      else result = metric[operation]();
      results.push({ value: operation === 'serializeMetrics' ? normalize(result) : null, error: null, clockCalls });
    } catch (error) { results.push({ value: null, error: error.message, clockCalls }); }
  }
  cases.push({ clock, kind, value, disabled, required, lifecycle, steps, results, warnings, emitted });
}
try {
  const numbers = [now, now - 1209600001, now - 1209600000, now - 1209599999, now + 7199999, now + 7200000, now + 7200001, 0, special('-0'), 0.5, -0.5, now + 0.5, 1.0000000000000002, 5e-324, special('NaN'), special('Infinity'), special('-Infinity'), -8640000000000001, -8640000000000000, -8639999999999999, 8639999999999999, 8640000000000000, 8640000000000001, 9007199254740991, 9007199254740992, 1e100, -1e100, Number.MAX_VALUE];
  for (const kind of ['number', 'date']) for (const value of numbers) for (const disabled of [false, true]) for (const required of [false, true]) {
    for (const lifecycle of ['none', 'clearMetrics', 'clearDimensions', 'singleMetric']) run(now, kind, value, disabled, required, lifecycle);
  }
  for (const value of ['0001-01-01T00:00:00Z', '1969-12-31T23:59:59.9995Z', '1970-01-01T00:00:00.0009Z', '2026-09-21T20:00:00.123456789+08:00']) {
    for (const disabled of [false, true]) for (const required of [false, true]) run(now, 'iso', value, disabled, required, 'none');
  }
  for (const kind of ['number', 'date']) for (const value of [0, special('-0'), 0.5, -0.5, special('NaN'), special('Infinity'), special('-Infinity')]) {
    for (const disabled of [false, true]) for (const required of [false, true]) run(0, kind, value, disabled, required, 'none');
  }
} finally {
  Date.now = originalNow;
  process.stdout.write = originalWrite;
  for (const [key, value] of Object.entries(original)) { if (value === undefined) delete process.env[key]; else process.env[key] = value; }
}
writeFileSync(new URL('../../metrics/testdata/timestamps-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', normalization: 'Inject Date.now and sort dimension names only. Compare full timestamps, errors, warnings and cumulative clock calls. Invalid Date inputs map to out-of-range Go instants.', cases }, null, 2) + '\n');
console.log(`${cases.length} Metrics timestamp reference cases`);
