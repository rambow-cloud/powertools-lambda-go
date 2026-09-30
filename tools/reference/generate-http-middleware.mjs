// Capture actual pinned CORS/compression behavior. No network requests are made.
import { writeFileSync } from 'node:fs';
import { gunzipSync, inflateSync } from 'node:zlib';
import { Router, BadRequestError } from '@aws-lambda-powertools/event-handler/http';
import { cors, compress } from '@aws-lambda-powertools/event-handler/http/middleware';

process.env.POWERTOOLS_DEV = 'false';
const cases = [];
const middleware = (items = []) => items.map(({ type, options }) => type === 'cors' ? cors(options) : compress(options));
function event(kind, method, headers) {
  if (kind === 'v2' || kind === 'url') return {
    version: '2.0', routeKey: '$default', rawPath: '/items', rawQueryString: '', headers,
    requestContext: { domainName: kind === 'url' ? 'id.lambda-url.ap-east-1.on.aws' : 'api.example.test', http: { method } }, isBase64Encoded: false,
  };
  if (kind === 'alb') return { httpMethod: method, path: '/items', headers, requestContext: { elb: {} }, body: null, isBase64Encoded: false };
  return { httpMethod: method, path: '/items', resource: '/items', headers, requestContext: { domainName: 'api.example.test' },
    body: null, isBase64Encoded: false, pathParameters: null, queryStringParameters: null, multiValueQueryStringParameters: null, stageVariables: null };
}
async function add(name, kind, spec) {
  const input = event(kind, spec.method ?? 'GET', spec.headers ?? {});
  const app = new Router({ logger: { debug() {}, warn() {}, error() {} } });
  let calls = 0;
  for (const item of middleware(spec.middleware)) app.use(item);
  app.route(() => {
    calls++;
    if (spec.failure) throw new BadRequestError('handler failed');
    return new Response(spec.body === undefined ? 'hello' : spec.body, { status: spec.status ?? 200, headers: spec.responseHeaders });
  }, { method: spec.routeMethod ?? 'GET', path: '/items', middleware: middleware(spec.routeMiddleware) });
  const response = await app.resolve(input, {});
  const encoding = response.headers['content-encoding'];
  let decoded;
  if (response.isBase64Encoded && ['gzip', 'deflate'].includes(encoding) && !Object.hasOwn(spec.responseHeaders ?? {}, 'content-encoding')) {
    const bytes = Buffer.from(response.body, 'base64');
    decoded = (encoding === 'gzip' ? gunzipSync(bytes) : inflateSync(bytes)).toString('base64');
  }
  cases.push({ name: `${kind}-${name}`, event: input, ...spec, expected: { response, calls, decoded } });
}

for (const kind of ['v1', 'v2', 'alb', 'url']) {
  const configurations = [
    {}, { origin: 'https://a.test' }, { origin: ['https://a.test', 'https://b.test'] },
    { origin: [] }, { origin: ['https://a.test', '*'], credentials: true },
    { origin: '', allowMethods: [], allowHeaders: [] },
    { origin: ['https://a.test'], credentials: true, allowMethods: ['get', 'pOsT'], allowHeaders: ['X-Custom'], exposeHeaders: ['X-One', 'X-Two'], maxAge: 0 },
    { origin: '*', allowMethods: ['OPTIONS', 'GET'], allowHeaders: ['*'], maxAge: 12.5 },
  ];
  const requests = [
    {}, { headers: { origin: 'https://a.test' } }, { headers: { origin: 'https://b.test' } },
    { headers: { origin: 'null' } }, { headers: { origin: '' } },
    ...[undefined, 'GET', 'post', 'OPTIONS', 'BAD'].map(method => ({ method: 'OPTIONS', headers: { origin: 'https://a.test', ...(method === undefined ? {} : { 'access-control-request-method': method }) } })),
    ...['', 'Content-Type, Authorization', 'X-Custom', '*', 'X-Custom,', 'Unknown', 'X-Custom, x-custom'].map(headers => ({ method: 'OPTIONS', headers: { origin: 'https://a.test', 'access-control-request-method': 'GET', 'access-control-request-headers': headers } })),
    { failure: true, headers: { origin: 'https://a.test' } },
  ];
  for (const [i, options] of configurations.entries()) for (const [j, request] of requests.entries()) {
    await add(`cors-${i}-${j}`, kind, { middleware: [{ type: 'cors', options }], ...request });
  }
  await add('cors-route-override', kind, { headers: { origin: 'https://a.test' }, middleware: [{ type: 'cors', options: {} }], routeMiddleware: [{ type: 'cors', options: { origin: ['https://a.test'], exposeHeaders: ['X-Route'] } }], responseHeaders: { vary: 'Accept-Language', 'access-control-expose-headers': 'X-Handler' } });
  await add('cors-route-preflight', kind, { method: 'OPTIONS', routeMethod: 'OPTIONS', headers: { origin: 'https://a.test', 'access-control-request-method': 'GET' }, routeMiddleware: [{ type: 'cors', options: {} }] });
  await add('cors-global-preflight-precedence', kind, { method: 'OPTIONS', routeMethod: 'OPTIONS', headers: { origin: 'https://a.test', 'access-control-request-method': 'GET' }, middleware: [{ type: 'cors', options: {} }], routeMiddleware: [{ type: 'cors', options: { origin: 'https://a.test' } }] });
  for (const maxAge of [-0, -1, 1e-7, 1e-6, 1e20, 1e21]) await add(`cors-max-age-${maxAge}`, kind, { method: 'OPTIONS', headers: { origin: 'https://a.test', 'access-control-request-method': 'GET' }, middleware: [{ type: 'cors', options: { maxAge } }] });
  for (const encoding of ['gzip', 'deflate']) {
    for (const accepted of [undefined, '', 'gzip', 'deflate', 'br', '*', 'gzip;q=0', 'identity, gzip', 'IDENTITY, gzip', 'GZIP', 'xgzipx', '*;q=0']) {
      for (const length of [1024, 1025]) await add(`compress-${encoding}-${accepted}-${length}`, kind, { body: 'a'.repeat(length), headers: accepted === undefined ? {} : { 'accept-encoding': accepted }, middleware: [{ type: 'compress', options: { encoding } }] });
    }
    for (const responseHeaders of [
      {}, { 'cache-control': 'public, NO-TRANSFORM , max-age=10' }, { 'cache-control': 'no-transform=1' },
      { 'cache-control': 'x-no-transform' }, { 'content-encoding': '' }, { 'content-encoding': 'gzip' },
      { 'transfer-encoding': 'chunked' }, { 'transfer-encoding': 'Chunked' }, { 'transfer-encoding': 'gzip' },
      ...['', '0', '0x100', ' 12 ', 'broken', 'Infinity', '-1'].map(value => ({ 'content-length': value })),
      { 'content-type': 'image/png', vary: 'Origin' },
    ]) await add(`compress-headers-${encoding}-${cases.length}`, kind, { body: 'hello λ 世界', responseHeaders, middleware: [{ type: 'compress', options: { encoding, threshold: 0 } }] });
    for (const body of [null, '', 'hello']) for (const threshold of [-1, 0, 5, 5.5]) await add(`compress-body-${encoding}-${cases.length}`, kind, { body, middleware: [{ type: 'compress', options: { encoding, threshold } }] });
    await add(`compress-head-${encoding}`, kind, { method: 'HEAD', routeMethod: 'HEAD', body: 'a'.repeat(2048), middleware: [{ type: 'compress', options: { encoding } }] });
    await add(`compress-error-${encoding}`, kind, { failure: true, middleware: [{ type: 'compress', options: { encoding, threshold: 0 } }] });
    await add(`compress-route-override-${encoding}`, kind, { body: 'hello', middleware: [{ type: 'compress', options: { encoding } }], routeMiddleware: [{ type: 'compress', options: { encoding: encoding === 'gzip' ? 'deflate' : 'gzip', threshold: 0 } }] });
  }
  for (const reverse of [false, true]) {
    const chain = [{ type: 'cors', options: { origin: ['https://a.test'] } }, { type: 'compress', options: { threshold: 0 } }];
    if (reverse) chain.reverse();
    await add(`combined-${reverse}`, kind, { headers: { origin: 'https://a.test' }, middleware: chain });
    await add(`combined-preflight-${reverse}`, kind, { method: 'OPTIONS', headers: { origin: 'https://a.test', 'access-control-request-method': 'GET' }, middleware: chain });
  }
}
writeFileSync('../../eventhandler/http/testdata/middleware-v2.35.0.json', JSON.stringify({ version: '2.35.0', cases }, null, 2) + '\n');
console.log(`${cases.length} HTTP middleware reference cases`);
