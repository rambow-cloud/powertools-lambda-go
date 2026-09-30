// Execute the pinned HTTP router without network requests or Lambda dependencies.
import { writeFileSync } from 'node:fs';
import * as http from '@aws-lambda-powertools/event-handler/http';

process.env.POWERTOOLS_DEV = 'false';
const cases = [];
function event(kind, path = '/items/a', method = 'GET', extra = {}) {
  if (kind === 'v2' || kind === 'url') return {
    version: '2.0', routeKey: '$default', rawPath: path, rawQueryString: '', headers: {},
    requestContext: { http: { method }, domainName: kind === 'url' ? 'id.lambda-url.ap-east-1.on.aws' : 'api.example.test' },
    isBase64Encoded: false, ...extra,
  };
  if (kind === 'alb') return { httpMethod: method, path, headers: {}, requestContext: { elb: {} }, body: '', isBase64Encoded: false, ...extra };
  return { httpMethod: method, path, resource: '/{proxy+}', headers: {}, requestContext: { domainName: 'api.example.test' },
    body: null, isBase64Encoded: false, pathParameters: null, queryStringParameters: null,
    multiValueQueryStringParameters: null, stageVariables: null, ...extra };
}
function middleware(name) {
  return async ({ reqCtx, next }) => {
    const order = reqCtx.get('order') ?? [];
    reqCtx.set('order', order);
    order.push(`${name}:before`);
    if (name === 'early') return { statusCode: 202, body: { early: true } };
    if (name === 'fail') throw new http.BadRequestError('middleware failed');
    reqCtx.res.headers.set(`x-${name}`, 'before');
    await next();
    if (name === 'twice') await next();
    order.push(`${name}:after`);
    reqCtx.res.headers.set(`x-${name}`, 'after');
    reqCtx.res.headers.set('x-order', order.join('|'));
    if (name === 'replace') return { statusCode: 203, body: 'replaced' };
  };
}
function handler(route) {
  return async (ctx) => {
    const order = ctx.get('order');
    order?.push('handler');
    switch (route.action) {
      case 'validated': return { valid: ctx.valid, body: await ctx.req.text() };
      case 'value': return route.value;
      case 'proxy': return route.value;
      case 'native': return new Response(route.value.body ?? null, { status: route.value.statusCode, headers: route.value.headers, statusText: route.value.statusText });
      case 'bytes': return Uint8Array.from(route.value).buffer;
      case 'error': throw new (http[route.error] ?? Error)(route.message ?? 'handler failed', undefined, route.details);
      default: return { method: ctx.req.method, url: ctx.req.url, headers: Object.fromEntries(ctx.req.headers), body: await ctx.req.text(), params: ctx.params, route: ctx.route, responseType: ctx.responseType, order: order ?? [] };
    }
  };
}
function schema(rule) {
  return { '~standard': { version: 1, vendor: 'fixture', validate(value) {
    if (rule === 'reject') return { issues: [{ message: 'rejected value', path: ['value'] }] };
    if (rule === 'order') {
      if (typeof value?.id !== 'string') return { issues: [{ message: 'id required', path: ['id'] }] };
      return { value: { ...value, id: value.id.toUpperCase() } };
    }
    return { value };
  } } };
}
function validation(config) {
  if (!config) return undefined;
  return Object.fromEntries(Object.entries(config).map(([side, fields]) => [side, Object.fromEntries(Object.entries(fields).map(([name, rule]) => [name, schema(rule)]))]));
}
async function add(name, specification) {
  const warnings = [];
  const options = { prefix: specification.prefix, logger: { debug() {}, error() {}, warn(message) { warnings.push(message); } } };
  const app = new http.Router(options);
  for (const name of specification.middleware ?? []) app.use(middleware(name));
  for (const route of specification.routes ?? []) app.route(handler(route), { method: route.method ?? 'GET', path: route.regex ? new RegExp(route.path) : route.path, middleware: (route.middleware ?? []).map(middleware), validation: validation(route.validation) });
  if (specification.child) {
    const child = new http.Router({ ...options, prefix: specification.child.prefix });
    for (const name of specification.child.middleware ?? []) child.use(middleware(name));
    for (const route of specification.child.routes) child.route(handler(route), { method: route.method ?? 'GET', path: route.path });
    app.includeRouter(child, { prefix: specification.child.mount });
  }
  if (specification.errorHandler) {
    app.errorHandler(http[specification.errorHandler.name] ?? Error, () => specification.errorHandler.value);
  }
  let expected;
  try { expected = { response: await app.resolve(specification.event, {}), warnings }; }
  catch (error) { expected = { error: error.name, message: error.message, warnings }; }
  cases.push({ name, ...specification, expected });
}
for (const kind of ['v1', 'v2', 'alb', 'url']) {
  for (const method of ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS', 'get', 'patch', 'TRACE']) {
    await add(`${kind}-method-${method}`, { event: event(kind, '/items/a', method), routes: [{ path: '/items/:id', method: method.toUpperCase() }] });
  }
  for (const path of ['/items/static', '/items/a', '/items/a/', '/items/a///', '/items/a%2Fb', '/items/a+b', '/items/%20', '/items/%E4%BD%A0', '/items/%F0%9F%98%80', '/items/%FF', '/items/a:b', '/items/a.b', '/items/a/b', '/missing']) {
    await add(`${kind}-route-${cases.length}`, { event: event(kind, path), routes: [{ path: '/items/.*', regex: true, action: 'value', value: 'regex' }, { path: '/items/:id' }, { path: '/items/static', action: 'value', value: 'static' }] });
  }
  for (const response of [null, 'text', 42, false, { message: '<hello>&' }, [1, 'a'],
    { statusCode: 201, body: { created: true } }, { statusCode: 202, headers: {} },
    { statusCode: 200, extra: true }, { statusCode: 200, body: 'text', extra: true },
    { statusCode: 200, body: null }, { statusCode: '200', body: '' },
    { statusCode: 200, headers: null, body: '' }, { statusCode: 200, isBase64Encoded: 'true' },
    { statusCode: 200, body: [], headers: { 'x-number': 1, 'x-bool': true, 'x-null': null } },
    { statusCode: 200, body: false },
  ]) {
    await add(`${kind}-value-${cases.length}`, { event: event(kind), routes: [{ path: '/items/a', action: 'value', value: response }] });
  }
  for (const value of [
    { statusCode: 201, body: { created: true }, headers: { 'x-test': 'value' } },
    { statusCode: 202, body: 'text', headers: { 'Content-Type': 'text/custom' } },
    { statusCode: 204 }, { statusCode: 205 }, { statusCode: 304 },
    { statusCode: 204, body: '' },
    { statusCode: 200, body: 'a', cookies: ['a=1', 'b=2'], multiValueHeaders: { vary: ['Accept', 'Origin'], 'x-values': ['a', 'b'] } },
    { statusCode: 200, body: 'a', headers: { 'set-cookie': 'a=1; Expires=Wed, 21 Oct 2030 07:28:00 GMT', 'cache-control': 'public, max-age=10' } },
    { statusCode: 200, body: 'encoded?', isBase64Encoded: true },
    { statusCode: 200, body: 'image', headers: { 'content-type': 'image/png' } },
  ]) await add(`${kind}-proxy-${cases.length}`, { event: event(kind), routes: [{ path: '/items/a', action: 'proxy', value }] });
  for (const value of [
    { statusCode: 201, body: 'plain' }, { statusCode: 204 },
    { statusCode: 299, body: 'custom', statusText: 'Custom' },
    { statusCode: 418, body: 'teapot' },
  ]) await add(`${kind}-native-${cases.length}`, { event: event(kind), routes: [{ path: '/items/a', action: 'native', value }] });
  await add(`${kind}-binary`, { event: event(kind), routes: [{ path: '/items/a', action: 'bytes', value: [0, 1, 255, 128] }] });
  for (const body of ['', 'plain', '{"value":1}', '5L2g5aW9', 'Y Q!!', '__8=']) {
    for (const encoded of [false, true]) await add(`${kind}-body-${cases.length}`, { event: event(kind, '/items/a', 'POST', { body, isBase64Encoded: encoded }), routes: [{ path: '/items/:id', method: 'POST' }] });
  }
  const headers = { Host: 'host.example.test', 'X-Forwarded-Proto': 'http', 'x-list': 'alphabet', 'x-case': 'a', Cookie: 'old=1' };
  await add(`${kind}-request-fields`, { event: event(kind, '/items/a', 'POST', kind === 'v2' || kind === 'url'
    ? { headers, rawQueryString: 'a=1&a=2&space=hello+world&encoded=%2F', cookies: ['a=1', 'b=2'], body: '{}' }
    : { headers, multiValueHeaders: { 'X-List': ['alpha', 'new'], 'X-Case': ['a', 'b'] }, queryStringParameters: { a: 'old', first: '1', space: 'hello world' }, multiValueQueryStringParameters: { a: ['1', '2'], empty: [] }, body: '{}' }), routes: [{ path: '/items/:id', method: 'POST' }] });
  for (const error of ['BadRequestError', 'UnauthorizedError', 'ForbiddenError', 'NotFoundError', 'MethodNotAllowedError', 'RequestTimeoutError', 'RequestEntityTooLargeError', 'InternalServerError', 'ServiceUnavailableError', 'Error']) {
    await add(`${kind}-${error}`, { event: event(kind), routes: [{ path: '/items/a', action: 'error', error, details: { reason: 'fixture' } }] });
  }
  for (const middleware of [['global'], ['early'], ['fail'], ['twice'], ['replace'], ['outer', 'inner']]) {
    await add(`${kind}-middleware-${cases.length}`, { event: event(kind), middleware, routes: [{ path: '/items/:id', middleware: ['route'] }] });
  }
  await add(`${kind}-not-found-middleware`, { event: event(kind), middleware: ['global'], routes: [] });
  for (const name of ['NotFoundError', 'Error']) await add(`${kind}-custom-${name}`, { event: event(kind), routes: [], errorHandler: { name, value: { message: 'custom' } } });
}
for (const prefix of ['/api', '/api/', '/api///']) {
  for (const path of ['/api', '/api/', '/api///', '/api/a']) await add(`prefix-${cases.length}`, { prefix, event: event('v2', path), routes: [{ path: '/' }, { path: '/:id' }] });
}
for (const path of ['/items/a', '/items/a/view', '/items/fixed/view']) await add(`specificity-${cases.length}`, { event: event('v2', path), routes: [{ path: '/:group/:id' }, { path: '/items/:id' }, { path: '/items/:id/view' }, { path: '/:group/fixed/view' }] });
for (const path of ['/items/a', '/items/static']) await add(`duplicate-${cases.length}`, { event: event('v2', path), routes: [{ path: '/items/:id', action: 'value', value: 'old' }, { path: '/items/:id', action: 'value', value: 'new' }, { path: '/items/static', action: 'value', value: 'old' }, { path: '/items/static', action: 'value', value: 'new' }] });
for (const path of ['/items/:id/:id', '/items/:', '/items/:bad-name']) await add(`invalid-pattern-${cases.length}`, { event: event('v2'), routes: [{ path }] });
for (const path of ['/api/v1', '/api/v1/', '/parent']) await add(`included-${cases.length}`, { event: event('v2', path), routes: [{ path: '/parent' }], child: { prefix: '/v1', mount: '/api/', middleware: ['child'], routes: [{ path: '/' }] } });
for (const input of [{}, null, { version: '2.0' }]) await add(`invalid-event-${cases.length}`, { event: input, routes: [] });
for (const path of ['/numbers/42', '/numbers/42/', '/numbers/no', '/numbers/1/extra']) {
  await add(`named-regex-${cases.length}`, { event: event('v2', path), routes: [{ path: '/numbers/(?<id>[0-9]+)', regex: true }] });
}
for (const kind of ['v1', 'v2', 'alb', 'url']) {
  for (const body of ['{"id":"abc"}', '{"id":1}', '{}', '{', 'null']) {
    await add(`${kind}-validated-${cases.length}`, { event: event(kind, '/items/a', 'POST', { headers: { 'content-type': 'application/json' }, body }), routes: [{ path: '/items/:id', method: 'POST', action: 'validated', validation: { req: { body: 'order', headers: 'accept', path: 'accept', query: 'accept' }, res: { headers: 'accept' } } }] });
  }
  await add(`${kind}-validation-aggregate`, { event: event(kind, '/items/a', 'POST', { body: 'plain' }), routes: [{ path: '/items/:id', method: 'POST', action: 'validated', validation: { req: { body: 'reject', headers: 'reject', path: 'reject', query: 'reject' } } }] });
  for (const value of [{ statusCode: 200, body: 'broken-json' }, { statusCode: 200, body: { id: 'abc' } }, { statusCode: 204 }]) {
    await add(`${kind}-validation-output-${cases.length}`, { event: event(kind), routes: [{ path: '/items/a', action: 'proxy', value, validation: { res: { body: 'reject' } } }] });
  }
}
writeFileSync('../../eventhandler/http/testdata/http-v2.35.0.json', JSON.stringify({ version: '2.35.0', cases }, null, 2) + '\n');
console.log(`${cases.length} HTTP reference cases`);
