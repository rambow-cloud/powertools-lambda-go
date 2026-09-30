// Record the pinned public store lifecycle and runtime empty-metric policy.
import { Metrics } from '@aws-lambda-powertools/metrics';
import { mkdirSync, writeFileSync } from 'node:fs';

const now = 1789992000000;
const originalNow = Date.now;
const originalWrite = process.stdout.write;
const cases = [];
for (const key of ['POWERTOOLS_METRICS_NAMESPACE', 'POWERTOOLS_METRICS_DISABLED', 'POWERTOOLS_DEV', 'POWERTOOLS_SERVICE_NAME']) delete process.env[key];
const normalize = (value) => {
  const copy = JSON.parse(JSON.stringify(value));
  for (const directive of copy._aws.CloudWatchMetrics) for (const dimensions of directive.Dimensions) dimensions.sort();
  return copy;
};
Date.now = () => now;
try {
  for (const disabled of [false, true]) for (const required of [false, true]) {
    for (const clear of ['clearDimensions', 'clearMetadata', 'clearMetrics', 'clearDefaultDimensions']) {
      for (const timestamp of [false, true]) for (const policy of [null, false, true]) {
        const steps = [
          ['addDimension', 'request', 'one'], ['addDimensions', { stage: 'dev' }],
          ['addMetadata', 'context', 'kept'], ['addMetric', 'Count', 'Count', 1, 1],
          ['addMetric', 'Count', 'Count', 2, 60],
          ...(timestamp ? [['setTimestamp', now - 3600000]] : []),
          ...(policy === null ? [] : [['setThrowOnEmptyMetrics', policy]]),
          [clear], ['hasStoredMetrics'], ['serializeMetrics'],
          ['addMetric', 'Next', 'Count', 3], ['publishStoredMetrics'],
          ['hasStoredMetrics'], ['serializeMetrics'], ['setThrowOnEmptyMetrics', false], ['serializeMetrics'],
        ];
        cases.push(run(`${disabled}-${required}-${clear}-${timestamp}-${policy}`, disabled, required, steps));
      }
    }
    cases.push(run(`${disabled}-${required}-single`, disabled, required, [
      ['setThrowOnEmptyMetrics', true], ['singleMetric'], ['publishStoredMetrics'],
      ['hasStoredMetrics'], ['addMetric', 'Single', 'Count', 1], ['hasStoredMetrics'], ['serializeMetrics'],
    ]));
    cases.push(run(`${disabled}-${required}-deprecated`, disabled, required, [
      ['throwOnEmptyMetrics'], ['serializeMetrics'], ['publishStoredMetrics'],
      ['setThrowOnEmptyMetrics', false], ['serializeMetrics'],
    ]));
  }
} finally { Date.now = originalNow; process.stdout.write = originalWrite; }

function run(name, disabled, required, steps) {
  process.env.POWERTOOLS_METRICS_DISABLED = String(disabled);
  const emitted = [];
  process.stdout.write = (chunk) => { emitted.push(normalize(JSON.parse(chunk.toString()))); return true; };
  let metric = new Metrics({ namespace: 'StoreTest', serviceName: 'orders', defaultDimensions: { environment: 'test' }, logger: { warn() {} } });
  metric.setThrowOnEmptyMetrics(required);
  const results = [];
  for (const [operation, ...args] of steps) {
    try {
      const value = metric[operation](...args);
      if (operation === 'singleMetric') metric = value;
      results.push({ emptyError: false, value: operation === 'serializeMetrics' ? normalize(value) : operation === 'hasStoredMetrics' ? value : null });
    } catch (error) {
      if (!(error instanceof RangeError) || error.message !== 'The number of metrics recorded must be higher than zero') throw error;
      results.push({ emptyError: true, value: null });
    }
  }
  return { name, disabled, required, steps, results, emitted };
}
const directory = new URL('../../metrics/testdata/', import.meta.url);
mkdirSync(directory, { recursive: true });
writeFileSync(new URL('stores-v2.35.0.json', directory), JSON.stringify({ upstream: '2.35.0', now, normalization: 'Inject Date.now, sort dimension names, map the specific empty-metrics RangeError to emptyError.', cases }, null, 2) + '\n');
console.log(`${cases.length} Metrics store reference cases`);
