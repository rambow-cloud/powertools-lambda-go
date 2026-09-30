import { Metrics, MetricUnit, MetricResolution } from '@aws-lambda-powertools/metrics';
import { mkdirSync, writeFileSync } from 'node:fs';

for (const key of ['POWERTOOLS_METRICS_NAMESPACE', 'POWERTOOLS_METRICS_DISABLED', 'POWERTOOLS_DEV', 'POWERTOOLS_SERVICE_NAME', 'POWERTOOLS_METRICS_FUNCTION_NAME']) delete process.env[key];
const documents = [];
const stdout = process.stdout.write;
process.stdout.write = (chunk, encoding, callback) => {
  const document = JSON.parse(chunk.toString());
  delete document._aws.Timestamp;
  for (const directive of document._aws.CloudWatchMetrics) for (const keys of directive.Dimensions) keys.sort();
  documents.push(document);
  if (typeof encoding === 'function') encoding();
  if (typeof callback === 'function') callback();
  return true;
};
const cases = {};
const run = (name, action) => {
  const start = documents.length;
  action(new Metrics({ namespace: 'Example', serviceName: 'orders', defaultDimensions: { environment: 'test' }, logger: { warn() {} } }));
  cases[name] = documents.slice(start);
};
try {
  run('dimensions_and_flush', m => {
    m.addDimension('region', 'hk');
    m.addDimensions({ stage: 'prod' });
    m.addMetadata('request_id', 'request-1');
    m.addMetric('Requests', MetricUnit.Count, 1);
    m.addMetric('Latency', MetricUnit.Milliseconds, 10, MetricResolution.High);
    m.addMetric('Latency', MetricUnit.Milliseconds, 20);
    m.publishStoredMetrics();
    m.addMetric('Next', MetricUnit.Count, 2).publishStoredMetrics();
  });
  run('metric_limit', m => {
    m.addDimension('request', 'first');
    for (let i = 0; i < 101; i++) m.addMetric(`Metric${i}`, MetricUnit.Count, i);
    m.publishStoredMetrics();
  });
  run('value_limit', m => {
    m.addDimension('request', 'first');
    for (let i = 0; i < 101; i++) m.addMetric('Value', MetricUnit.Count, i);
    m.publishStoredMetrics();
  });
  run('single_metric', m => {
    m.addDimension('request', 'parent');
    m.addMetric('Parent', MetricUnit.Count, 1);
    m.singleMetric().addMetric('Child', MetricUnit.Count, 2);
    m.publishStoredMetrics();
  });
} finally { process.stdout.write = stdout; }
mkdirSync(new URL('../../metrics/testdata/', import.meta.url), { recursive: true });
writeFileSync(new URL('../../metrics/testdata/typescript-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', normalization: 'Remove timestamps and sort keys inside each dimension set only.', cases }, null, 2) + '\n');
