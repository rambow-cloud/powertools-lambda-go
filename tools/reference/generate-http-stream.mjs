// Use the pinned router; replace only the Lambda-owned destination framing hook.
import { writeFileSync } from 'node:fs';
import { Readable, Writable } from 'node:stream';

globalThis.awslambda = { HttpResponseStream: { from(destination, metadata) {
  destination.metadata = structuredClone(metadata);
  return destination;
} } };
process.env.POWERTOOLS_DEV = 'false';
const { Router, BadRequestError } = await import('@aws-lambda-powertools/event-handler/http');
const { cors, compress } = await import('@aws-lambda-powertools/event-handler/http/middleware');
const cases = [];

function event(kind, method = 'GET') {
  const headers = { origin: 'https://app.test', 'accept-encoding': 'gzip' };
  if (kind === 'v2' || kind === 'url') return {
    version: '2.0', routeKey: '$default', rawPath: '/items/a', rawQueryString: '', headers,
    requestContext: { http: { method }, domainName: kind === 'url' ? 'id.lambda-url.ap-east-1.on.aws' : 'api.example.test' }, isBase64Encoded: false,
  };
  const base = { httpMethod: method, path: '/items/a', headers, requestContext: { elb: {} }, body: null, isBase64Encoded: false };
  if (kind === 'alb') return base;
  return { ...base, resource: '/items/:id', requestContext: { domainName: 'api.example.test' }, pathParameters: null, queryStringParameters: null, multiValueQueryStringParameters: null, stageVariables: null };
}
async function add(name, kind, spec) {
  const app = new Router({ logger: { debug() {}, warn() {}, error() {} } });
  if (spec.cors) app.use(cors({ origin: ['https://app.test'], exposeHeaders: ['X-A', 'X-B'] }));
  if (spec.compress) app.use(compress({ threshold: 0 }));
  app.use(async ({ reqCtx, next }) => {
    reqCtx.res.headers.set('x-before', 'yes');
    await next();
    reqCtx.res.headers.set('x-after', 'yes');
  });
  if (!spec.missing) app.route(async (ctx) => {
    switch (spec.action) {
      case 'error': throw new BadRequestError('bad stream');
      case 'inspect': return { streaming: ctx.isHttpStreaming, route: ctx.route, params: ctx.params };
      case 'json': return spec.value;
      case 'bytes': return Uint8Array.from(spec.bytes).buffer;
      case 'proxy': return { statusCode: spec.status ?? 200, body: spec.body, headers: spec.headers, multiValueHeaders: spec.multiValueHeaders };
      case 'reader': return Readable.from(spec.chunks.map(value => Buffer.from(value, 'base64')));
      default: return new Response(spec.body ?? null, { status: spec.status ?? 200, headers: spec.headers });
    }
  }, { method: spec.method ?? 'GET', path: '/items/:id' });
  const chunks = [];
  const destination = new Writable({ write(chunk, _encoding, next) { chunks.push(Buffer.from(chunk)); next(); } });
  let failure;
  try { await app.resolveStream(event(kind, spec.method), {}, { responseStream: destination }); }
  catch (error) { failure = { name: error.name, message: error.message }; }
  cases.push({ name: `${kind}-${name}`, ...spec, event: event(kind, spec.method), expected: { metadata: destination.metadata, body: Buffer.concat(chunks).toString('base64'), ended: destination.writableEnded, error: failure } });
}
for (const kind of ['v1', 'v2', 'alb', 'url']) {
  for (const cors of [false, true]) for (const compress of [false, true]) {
    for (const [name, spec] of Object.entries({
      null: { body: null }, empty: { body: '' }, text: { body: 'hello λ 世界' },
      large: { body: 'a'.repeat(4096) }, noContent: { status: 204 }, notModified: { status: 304 },
      inspect: { action: 'inspect' }, json: { action: 'json', value: { id: 1, label: '<λ>&' } },
      bytes: { action: 'bytes', bytes: [0, 1, 128, 255] },
      reader: { action: 'reader', chunks: ['YQ==', 'AP8=', '5LiW55WM'] },
      proxy: { action: 'proxy', status: 201, body: 'created', headers: { 'x-test': 'ok' } },
      error: { action: 'error' }, missing: { missing: true },
      head: { method: 'HEAD', body: 'head body' }, unsupported: { method: 'TRACE' },
      cookies: { body: 'cookie', headers: { 'set-cookie': 'a=1, b=2', vary: 'Origin, Accept', 'cache-control': 'public, max-age=10' } },
      proxyHeaders: { action: 'proxy', body: 'headers', multiValueHeaders: { 'set-cookie': ['a=1', 'b=2'], vary: ['Origin', 'Accept'], 'x-many': ['one', 'two'] } },
    })) await add(`${name}-${cors}-${compress}`, kind, { ...spec, cors, compress });
  }
}
writeFileSync('../../eventhandler/http/testdata/stream-v2.35.0.json', JSON.stringify({ version: '2.35.0', cases }, null, 2) + '\n');
console.log(`${cases.length} HTTP stream reference cases`);
