// Execute the pinned cold-start APIs; preserve complete emitted documents.
import { Metrics } from '@aws-lambda-powertools/metrics';
import { writeFileSync } from 'node:fs';

const now = 1789992000000;
const originalNow = Date.now;
const originalWrite = process.stdout.write;
const keys = ['AWS_LAMBDA_INITIALIZATION_TYPE', 'POWERTOOLS_METRICS_FUNCTION_NAME', 'POWERTOOLS_METRICS_DISABLED', 'POWERTOOLS_METRICS_NAMESPACE', 'POWERTOOLS_SERVICE_NAME', 'POWERTOOLS_DEV'];
const environment = Object.fromEntries(keys.map((key) => [key, process.env[key]]));
const cases = [];
const names = [null, '', ' \ufeff', ' name ', '\u0085', '\u200b'];
const normalize = (value) => {
  const result = JSON.parse(JSON.stringify(value));
  for (const directive of result._aws.CloudWatchMetrics) for (const dimensions of directive.Dimensions) dimensions.sort();
  return result;
};
function run(initialization, disabled, envName, option, setter, argument, lifecycle = 'plain') {
  for (const key of keys) delete process.env[key];
  process.env.AWS_LAMBDA_INITIALIZATION_TYPE = initialization;
  process.env.POWERTOOLS_METRICS_DISABLED = String(disabled);
  if (envName !== null) process.env.POWERTOOLS_METRICS_FUNCTION_NAME = envName;
  const emitted = [];
  const warnings = [];
  process.stdout.write = (chunk) => { emitted.push(normalize(JSON.parse(chunk.toString()))); return true; };
  const metric = new Metrics({ namespace: 'ColdTest', serviceName: 'orders', defaultDimensions: { environment: 'test' }, ...(option === null ? {} : { functionName: option }), logger: { warn(message) { warnings.push(message); } } });
  metric.addDimension('request', 'parent');
  metric.addMetadata('context', 'parent');
  metric.setTimestamp(now - 3600000);
  metric.addMetric('Parent', 'Count', 2);
  if (setter !== null) metric.setFunctionName(setter);
  if (lifecycle === 'clear') metric.clearMetrics();
  const target = lifecycle === 'single' ? metric.singleMetric() : metric;
  target.captureColdStartMetric(...(argument === null ? [] : [argument]));
  if (lifecycle === 'setter-after') target.setFunctionName('late');
  target.captureColdStartMetric('second');
  const parent = normalize(metric.serializeMetrics());
  metric.publishStoredMetrics();
  cases.push({ initialization, disabled, envName, option, setter, argument, lifecycle, parent, emitted, warnings });
}
try {
  Date.now = () => now;
  for (const envName of [null, 'env', '\ufeffenv\u0085']) for (const option of names) {
    for (const setter of [null, '', ' \ufeff', ' set ', '\u0085']) for (const argument of [null, ' arg ', '\u0085']) {
      run('on-demand', false, envName, option, setter, argument);
    }
  }
  for (const initialization of ['on-demand', 'provisioned-concurrency', '', ' on-demand ']) for (const disabled of [false, true]) {
    for (const lifecycle of ['plain', 'single', 'clear', 'setter-after']) run(initialization, disabled, 'environment', 'configured', 'setter', 'argument', lifecycle);
  }
} finally {
  Date.now = originalNow;
  process.stdout.write = originalWrite;
  for (const [key, value] of Object.entries(environment)) {
    if (value === undefined) delete process.env[key]; else process.env[key] = value;
  }
}
writeFileSync(new URL('../../metrics/testdata/coldstart-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', now, normalization: 'Inject Date.now and sort dimension names; full timestamps, warning strings and values are exact.', cases }, null, 2) + '\n');
console.log(`${cases.length} Metrics cold-start reference cases`);
