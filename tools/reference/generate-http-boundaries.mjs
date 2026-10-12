// Run the actual pinned converters and router; preserve native diagnostics.
import { writeFileSync } from 'node:fs';
import * as http from '@aws-lambda-powertools/event-handler/http';

process.env.POWERTOOLS_DEV = 'false';
const cases = [];
function event(kind, path = '/items/a', headers = {}) {
  if (kind === 'v2' || kind === 'url') return {
    version: '2.0', routeKey: '$default', rawPath: path, rawQueryString: '', headers,
    requestContext: { http: { method: 'GET' }, domainName: 'api.example.test' }, isBase64Encoded: false,
  };
  if (kind === 'alb') return { httpMethod: 'GET', path, headers, requestContext: { elb: {} }, body: null, isBase64Encoded: false };
  return { httpMethod: 'GET', path, resource: '/{proxy+}', headers, requestContext: { domainName: 'api.example.test' },
    body: null, isBase64Encoded: false, pathParameters: null, queryStringParameters: null,
    multiValueQueryStringParameters: null, stageVariables: null };
}
async function add(name, mode, input, run) {
  let expected;
  try { expected = { value: await run() }; }
  catch (error) { expected = { error: error.name, message: error.message }; }
  cases.push({ name, mode, input, expected });
}
for (const kind of ['v1', 'v2', 'alb', 'url']) {
  for (const [name, headers] of [
    ['token', { "!#$%&'*+-.^_`|~09AZaz": 'ok' }],
    ['control-name', { 'x-\u0001': 'ok' }], ['del-name', { 'x-\u007f': 'ok' }],
    ['unicode-name', { 'x-é': 'ok' }], ['space-name', { 'bad name': 'ok' }],
    ['colon-name', { 'bad:name': 'ok' }], ['empty-name', { '': 'ok' }],
    ['crlf-value', { 'x-test': 'a\r\nb' }], ['null-value', { 'x-test': 'a\u0000b' }],
    ['padding', { 'x-test': ' \tvalue\t ' }], ['tab', { 'x-test': 'a\tb' }],
    ['latin1', { 'x-test': 'é' }],
  ]) {
    const input = event(kind, '/items/a', headers);
    await add(`${kind}-${name}`, 'request', input, () => {
      const req = http.proxyEventToWebRequest(input);
      return { url: req.url, headers: Object.fromEntries(req.headers) };
    });
  }
  for (const [name, path, headers] of [
    ['encoded-slash', '/items/a%2Fb', {}], ['plus', '/items/a+b', {}],
    ['ipv6', '/items/a', { host: '[::1]:8443' }],
    ['bad-host', '/items/a', { host: '[::1' }],
    ['credentials', '/items/a', { host: 'user:pass@example.test' }],
    ['malformed-escape', '/items/%', {}],
  ]) {
    const input = event(kind, path, headers);
    await add(`${kind}-${name}`, 'url', input, () => {
      const req = http.proxyEventToWebRequest(input);
      return { url: req.url, headers: Object.fromEntries(req.headers) };
    });
  }
}
for (const [name, value] of [
  ['control-name', { headers: { 'x-\u0001': 'ok' } }],
  ['del-name', { headers: { 'x-\u007f': 'ok' } }],
  ['unicode-name', { headers: { 'x-é': 'ok' } }],
  ['crlf-value', { headers: { 'x-test': 'a\r\nb' } }],
  ['null-value', { headers: { 'x-test': 'a\u0000b' } }],
  ['multi-invalid', { multiValueHeaders: { 'x-test': ['ok', 'a\r\nb'] } }],
  ['cookie-invalid', { cookies: ['a=1\r\nb=2'] }],
  ['cookies', { cookies: ['a=1; Expires=Mon, 21 Oct 2030 07:28:00 GMT', 'b=2'] }],
]) {
  const input = { statusCode: 200, body: 'ok', ...value };
  await add(`response-${name}`, 'response', input, async () => {
    const res = http.handlerResultToWebResponse(input);
    return { status: res.status, headers: Object.fromEntries(res.headers), cookies: res.headers.getSetCookie(), body: await res.text() };
  });
}
for (const [name, pattern, path] of [
  ['named', '/items/(?<id>[0-9]+)', '/items/42'],
  ['named-miss', '/items/(?<id>[0-9]+)', '/items/no'],
  ['slash', '/items/(?<id>.+)', '/items/a%2Fb'],
  ['invalid-utf8', '/items/(?<id>.+)', '/items/%FF'],
  ['blank', '/items/(?<id>.+)', '/items/%20'],
  ['alternation-first', '/items/a|/other', '/items/abc'],
  ['alternation-last', '/items/a|/other', '/prefix/other'],
  ['anchors', '^/items/(?<id>[0-9]+)$', '/items/42'],
]) {
  const input = { pattern, event: event('v2', path) };
  await add(`regex-${name}`, 'regex', input, async () => {
    const app = new http.Router({ logger: { debug() {}, error() {}, warn() {} } });
    app.route(ctx => ({ params: ctx.params }), { method: 'GET', path: new RegExp(pattern) });
    return app.resolve(input.event, {});
  });
}
writeFileSync(new URL('../../eventhandler/http/testdata/boundaries-v2.35.0.json', import.meta.url), JSON.stringify({ version: '2.35.0', cases }, null, 2) + '\n');
console.log(`${cases.length} HTTP boundary observations`);
