// Observe constructor precedence, custom getter calls and single-metric reconstruction.
import { Metrics } from '@aws-lambda-powertools/metrics';
import { writeFileSync } from 'node:fs';

const now = 1789992000000;
const keys = ['POWERTOOLS_METRICS_NAMESPACE', 'POWERTOOLS_SERVICE_NAME', 'POWERTOOLS_METRICS_FUNCTION_NAME', 'POWERTOOLS_METRICS_DISABLED', 'POWERTOOLS_DEV', 'AWS_LAMBDA_INITIALIZATION_TYPE'];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));
const originalWrite = process.stdout.write;
const originalNow = Date.now;
const cases = [];
function apply(env) {
  for (const [key, value] of Object.entries(env)) {
    if (value === null) delete process.env[key]; else process.env[key] = value;
  }
}
function normalize(value) {
  const result = JSON.parse(JSON.stringify(value));
  for (const directive of result._aws.CloudWatchMetrics) for (const dimensions of directive.Dimensions) dimensions.sort();
  return result;
}
function run(input) {
  for (const key of keys) delete process.env[key];
  process.env.AWS_LAMBDA_INITIALIZATION_TYPE = 'on-demand';
  apply(input.env ?? {});
  const result = { calls: [], warnings: [], emitted: [], error: null, stage: 'construct', parent: null, child: null, parentDisabled: null, childDisabled: null };
  process.stdout.write = (chunk) => { result.emitted.push(normalize(JSON.parse(chunk.toString()))); return true; };
  const custom = input.custom && Object.fromEntries(['getNamespace', 'getServiceName', 'getFunctionName'].map((method) => [method, () => {
    result.calls.push(method);
    if (input.custom.fail === method || method === 'getFunctionName') throw new Error(`configuration failure: ${method}`);
    apply(input.custom.mutate ?? {});
    return input.custom[method] ?? '';
  }]));
  try {
    const metric = new Metrics({
      ...(input.namespace == null ? {} : { namespace: input.namespace }),
      ...(input.service == null ? {} : { serviceName: input.service }),
      ...(input.defaults == null ? {} : { defaultDimensions: input.defaults }),
      ...(custom ? { customConfigService: custom } : {}),
      singleMetric: input.optionSingle ?? false,
      logger: { warn(message) { result.warnings.push(message); } },
    });
    result.parentDisabled = metric.isDisabled();
    metric.setTimestamp(now - 3600000);
    metric.addMetric('Parent', 'Count', 2);
    result.parent = normalize(metric.serializeMetrics());
    if (input.single) {
      if (input.clear) metric.clearDefaultDimensions();
      apply(input.after ?? {});
      result.stage = 'single';
      const child = metric.singleMetric();
      result.childDisabled = child.isDisabled();
      result.child = normalize(child.serializeMetrics());
      child.addMetric('Child', 'Count', 3);
      child.captureColdStartMetric('child-fallback');
    }
    result.stage = 'flush';
    metric.publishStoredMetrics();
    result.stage = 'done';
  } catch (error) { result.error = error.message; }
  cases.push({ input, result });
}
try {
  Date.now = () => now;
  const values = [null, '', ' ', ' option ', '\u0085'];
  const custom = [null, {}, { getNamespace: 'custom-ns', getServiceName: 'custom-svc' }, { fail: 'getNamespace' }, { fail: 'getServiceName' }];
  for (const namespace of values) for (const service of values) for (const config of custom) {
    for (const env of [{}, { POWERTOOLS_METRICS_NAMESPACE: ' env-ns ', POWERTOOLS_SERVICE_NAME: ' env-svc ' }]) run({ namespace, service, custom: config, env });
  }
  for (const disabled of [null, '', 'true', 'false', '1', '0', 'y', 'n', 'yes', 'no', 'on', 'off', ' t ', ' f ', 'TRUE', '\ufefftrue', 'wat', '\u0085true']) {
    for (const dev of [null, '', 'true', 'false', 'on', 'off', 'invalid', '\ufefftrue']) run({ env: { POWERTOOLS_METRICS_DISABLED: disabled, POWERTOOLS_DEV: dev }, custom: { getNamespace: 'EnvTest', getServiceName: 'orders' } });
  }
  const dimensions = (count) => ({ service: ' ', ...Object.fromEntries(Array.from({ length: count }, (_, i) => [`d${String(i).padStart(2, '0')}`, 'value'])) });
  for (const after of [{}, { POWERTOOLS_METRICS_DISABLED: 'true' }, { POWERTOOLS_METRICS_DISABLED: 'false' }, { POWERTOOLS_METRICS_DISABLED: '' }, { POWERTOOLS_DEV: 'true', POWERTOOLS_METRICS_DISABLED: null }, { POWERTOOLS_METRICS_NAMESPACE: 'new-ns', POWERTOOLS_SERVICE_NAME: 'new-svc', POWERTOOLS_METRICS_FUNCTION_NAME: 'new-function' }, { POWERTOOLS_SERVICE_NAME: ' ' }, { POWERTOOLS_METRICS_DISABLED: 'bad' }]) {
    for (const namespace of ['', 'Parent']) for (const clear of [false, true]) for (const defaults of [null, { service: '' }, dimensions(28), dimensions(29)]) run({ namespace, clear, defaults, single: true, after });
  }
  for (const single of [false, true]) run({ single, env: { POWERTOOLS_METRICS_NAMESPACE: 'cached-ns', POWERTOOLS_SERVICE_NAME: 'cached-svc' }, custom: { mutate: { POWERTOOLS_METRICS_NAMESPACE: 'changed-ns', POWERTOOLS_SERVICE_NAME: 'changed-svc', POWERTOOLS_METRICS_DISABLED: 'true' } } });
  for (const optionSingle of [false, true]) for (const disabled of ['false', 'true']) for (const defaults of [null, { service: 'configured' }]) run({ optionSingle, defaults, namespace: 'Immediate', env: { POWERTOOLS_METRICS_DISABLED: disabled } });
} finally {
  process.stdout.write = originalWrite;
  Date.now = originalNow;
  for (const [key, value] of Object.entries(original)) {
    if (value === undefined) delete process.env[key]; else process.env[key] = value;
  }
}
writeFileSync(new URL('../../metrics/testdata/config-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', now, normalization: 'Inject Date.now and sort dimension names. Compare getter order, warnings, errors and complete documents exactly.', cases }, null, 2) + '\n');
console.log(`${cases.length} Metrics configuration reference cases`);
