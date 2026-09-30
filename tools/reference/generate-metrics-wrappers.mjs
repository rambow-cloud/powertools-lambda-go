// Exercise the actual Middy hooks without adding a framework dependency.
import { Metrics } from '@aws-lambda-powertools/metrics';
import { logMetrics } from '@aws-lambda-powertools/metrics/middleware';
import { METRICS_KEY } from '@aws-lambda-powertools/commons';
import { writeFileSync } from 'node:fs';

const now = 1789992000000;
const originalNow = Date.now, originalWrite = process.stdout.write;
const keys = ['POWERTOOLS_METRICS_NAMESPACE', 'POWERTOOLS_SERVICE_NAME', 'POWERTOOLS_METRICS_FUNCTION_NAME', 'POWERTOOLS_METRICS_DISABLED', 'POWERTOOLS_DEV', 'AWS_LAMBDA_INITIALIZATION_TYPE'];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));
const cases = [];
const normalize = (value) => {
  for (const directive of value._aws.CloudWatchMetrics) for (const dimension of directive.Dimensions) dimension.sort();
  return value;
};
function run(targets, defaults, required, strict, disabled, mode) {
  for (const key of keys) delete process.env[key];
  process.env.POWERTOOLS_METRICS_DISABLED = String(disabled);
  // Cold invocation composition is covered by the Lambda runner; these hooks are warm.
  process.env.AWS_LAMBDA_INITIALIZATION_TYPE = 'provisioned-concurrency';
  const warnings = [], emitted = [];
  process.stdout.write = (chunk) => { emitted.push(normalize(JSON.parse(chunk.toString()))); return true; };
  const instances = [0, 1].map((index) => {
    const m = new Metrics({ namespace: `Wrapper${index}`, serviceName: 'orders', defaultDimensions: { environment: 'base' }, logger: { warn(message) { warnings.push(`${index}:${message}`); } } });
    m.setThrowOnEmptyMetrics(required);
    return m;
  });
  const options = { throwOnEmptyMetrics: strict, captureColdStartMetric: true, ...(defaults === null ? {} : { defaultDimensions: defaults }) };
  const middleware = logMetrics(targets.map((i) => instances[i]), options);
  const request = { context: { functionName: 'wrapper-function' }, internal: {} };
  let called = false, value = null, error = null, stage = 'before';
  try {
    middleware.before(request);
    if (mode === 'early') {
      stage = 'publish';
      request.internal[METRICS_KEY]();
      value = 7;
    } else {
      stage = 'handler';
      called = true;
      let businessError;
      try {
        for (const index of new Set(targets)) {
          if (mode !== 'empty' && (mode !== 'partial' || index === 1)) instances[index].addMetric('Count', 'Count', index + 1);
        }
        if (mode === 'error') throw new Error('business failure');
        value = 42;
      } catch (failure) { businessError = failure; }
      stage = 'publish';
      middleware[businessError ? 'onError' : 'after'](request);
      if (businessError) { stage = 'handler'; throw businessError; }
    }
    stage = null;
  } catch (failure) { error = failure.message; value = null; }
  cases.push({ targets, defaults, required, strict, disabled, mode, called, value, error, stage, warnings, emitted });
}
try {
  Date.now = () => now;
  const layouts = [[0], [0, 1], [1, 0], [0, 0], [0, 1, 0], []];
  const dimensions = [null, {}, { region: 'west' }, { environment: 'wrapper', service: 'changed' }, { bad: '\ufeff' }, Object.fromEntries(Array.from({ length: 29 }, (_, i) => [`d${i}`, 'value']).sort(([a], [b]) => a.localeCompare(b)))];
  for (const targets of layouts) for (const defaults of dimensions) for (const [required, strict] of [[false, false], [true, false], [false, true]]) for (const disabled of [false, true]) for (const mode of ['success', 'empty', 'error', 'partial']) run(targets, defaults, required, strict, disabled, mode);
  for (const targets of layouts) for (const strict of [false, true]) run(targets, { region: 'west' }, false, strict, false, 'early');
} finally {
  Date.now = originalNow; process.stdout.write = originalWrite;
  for (const [key, value] of Object.entries(original)) { if (value === undefined) delete process.env[key]; else process.env[key] = value; }
}
writeFileSync(new URL('../../metrics/testdata/wrappers-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', now, normalization: 'Inject Date.now and sort only dimension-name arrays. Execute actual before/after/onError/early-cleanup hooks. Go PropagateErrors selects publication-error precedence; provisioned initialization suppresses cold capture in these cases.', cases }, null, 2) + '\n');
console.log(`${cases.length} Metrics wrapper reference cases`);
