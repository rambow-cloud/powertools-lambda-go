// Execute the pinned router and Metrics implementation without AWS calls.
import { mkdirSync, writeFileSync } from 'node:fs';
import { Router, BadRequestError } from '@aws-lambda-powertools/event-handler/http';
import { metrics as middleware } from '@aws-lambda-powertools/event-handler/http/middleware/metrics';
import { cors } from '@aws-lambda-powertools/event-handler/http/middleware';
import { Metrics } from '@aws-lambda-powertools/metrics';

for (const key of ['POWERTOOLS_METRICS_NAMESPACE', 'POWERTOOLS_METRICS_DISABLED', 'POWERTOOLS_DEV', 'POWERTOOLS_SERVICE_NAME']) delete process.env[key];
const cases = [];
const logger = { debug() {}, warn() {}, error() {} };
function event(kind, action, metadata) {
  const method = action === 'preflight' ? 'OPTIONS' : action === 'method' ? 'TRACE' : 'GET';
  const path = action === 'missing' ? '/missing' : '/items/id%20one';
  const headers = metadata === 'full' ? { 'user-agent': 'fixture-agent', 'x-forwarded-for': ' 198.51.100.2 , 198.51.100.3' } : {};
  if (action === 'preflight') Object.assign(headers, { origin: 'https://app.test', 'access-control-request-method': 'GET' });
  const sourceIp = metadata === 'full' ? '192.0.2.1' : '';
  const requestContext = { domainName: kind === 'url' ? 'id.lambda-url.ap-east-1.on.aws' : 'api.example.test' };
  if (metadata === 'full') Object.assign(requestContext, { requestId: 'request-1', apiId: 'api-1', extendedRequestId: 'extended-1' });
  if (metadata === 'empty') Object.assign(requestContext, { requestId: '', apiId: '', extendedRequestId: '' });
  if (metadata === 'null') Object.assign(requestContext, { requestId: null, apiId: null, extendedRequestId: null });
  if (kind === 'v2' || kind === 'url') return { version: '2.0', routeKey: '$default', rawPath: path, rawQueryString: 'secret=omitted', headers, requestContext: { ...requestContext, http: { method, sourceIp } }, isBase64Encoded: false };
  return { httpMethod: method, path, resource: path, headers, requestContext: kind === 'alb' ? { elb: {} } : { ...requestContext, identity: { sourceIp } }, body: null, isBase64Encoded: false, pathParameters: null, queryStringParameters: null, multiValueQueryStringParameters: null, stageVariables: null };
}
for (const kind of ['v1', 'v2', 'alb', 'url']) {
  for (const action of ['ok', 'created', 'redirect', 'client', 'server', 'http-error', 'error', 'missing', 'preflight', 'method', 'business']) {
    for (const metadata of ['full', 'empty', 'missing', 'null']) {
      const input = event(kind, action, metadata);
      const documents = [];
      const output = process.stdout.write;
      process.stdout.write = (chunk) => {
        const value = JSON.parse(chunk.toString());
        if (typeof value.latency !== 'number' || value.latency < 0) throw new Error('Invalid latency');
        delete value.latency;
        delete value._aws.Timestamp;
        for (const directive of value._aws.CloudWatchMetrics) for (const dimensions of directive.Dimensions) dimensions.sort();
        documents.push(value);
        return true;
      };
      try {
        const metric = new Metrics({ namespace: 'HTTPTest', serviceName: 'orders', defaultDimensions: { environment: 'test' }, logger });
        const app = new Router({ logger });
        app.use(middleware(metric));
        app.use(cors());
        app.get('/items/:id', () => {
          if (action === 'http-error') throw new BadRequestError('bad input');
          if (action === 'error') throw new Error('business failure');
          if (action === 'business') metric.addMetric('Orders', 'Count', 2).addMetadata('custom', 'value');
          return new Response('ok', { status: { created: 201, redirect: 302, client: 422, server: 503 }[action] ?? 200 });
        });
        const response = await app.resolve(input, {});
        cases.push({ name: `${kind}-${action}-${metadata}`, action, event: input, status: response.statusCode, documents });
      } finally { process.stdout.write = output; }
    }
  }
}
const directory = new URL('../../eventhandler/http/metrics/testdata/', import.meta.url);
mkdirSync(directory, { recursive: true });
writeFileSync(new URL('metrics-v2.35.0.json', directory), JSON.stringify({ upstream: '2.35.0', normalization: 'Remove timestamps and nonnegative numeric latency values; sort dimension keys only.', cases }, null, 2) + '\n');
console.log(`${cases.length} HTTP Metrics reference cases`);
