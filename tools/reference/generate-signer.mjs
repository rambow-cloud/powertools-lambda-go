// Generate deterministic fixtures by executing the pinned TypeScript Signer.
import { mkdirSync, writeFileSync } from 'node:fs';
import { SigV4Signer } from '@aws-lambda-powertools/signer/sigv4';

const timestamp = '2026-09-14T12:34:56.000Z';
const NativeDate = Date;
globalThis.Date = class extends NativeDate {
  constructor(...args) { super(...(args.length ? args : [timestamp])); }
  static now() { return new NativeDate(timestamp).getTime(); }
};
const credentials = { accessKeyId: 'AKIDEXAMPLE', secretAccessKey: 'wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY', sessionToken: 'fixture-session' };
const cases = [
  { name: 'get', url: 'https://example.execute-api.ap-east-1.amazonaws.com/items?b=two&a=one' },
  { name: 'json', method: 'POST', body: '{"name":"世界","count":2}', headers: { 'content-type': 'application/json' } },
  { name: 'binary', method: 'PUT', bodyBase64: 'AAECA//+' },
  { name: 'empty', method: 'POST', body: '' },
  { name: 'escaped-path', url: 'https://example.com/a%20b/%E4%B8%96%E7%95%8C' },
  { name: 'port', url: 'https://example.com:8443/a' },
  { name: 'headers', headers: [['x-custom','first'],['x-custom','second'],['x-space','  hello  world  ']] },
  { name: 'tokenless', tokenless: true },
  { name: 'unsigned-payload', headers: { 'x-amz-content-sha256': 'UNSIGNED-PAYLOAD' } },
  { name: 'query-encoding', url: 'https://example.com/path?space=hello+world&symbol=%21%2A%27%28%29&empty=' },
  { name: 'appsync', service: 'appsync', method:'POST', body:'{"query":"{ field }"}', headers:{'content-type':'application/json'} },
  { name: 'function-url', service: 'lambda', url: 'https://example.lambda-url.ap-east-1.on.aws/' },
  { name: 'duplicate-query', url: 'https://example.com/path?a=first&a=second', intentionalDifference: true },
];
const results = [];
for (const item of cases) {
  const body = item.bodyBase64 ? Buffer.from(item.bodyBase64,'base64') : item.body;
  const request = new Request(item.url ?? 'https://example.com/items', { method:item.method ?? 'GET', headers:item.headers, body });
  const selected = { ...credentials, ...(item.tokenless ? { sessionToken:undefined } : {}) };
  const signer = new SigV4Signer({ service:item.service ?? 'execute-api', region:'ap-east-1', credentials:selected });
  const originalBody = Buffer.from(await request.clone().arrayBuffer()).toString('base64');
  const signed = await signer.sign(request);
  results.push({ name:item.name, url:request.url, method:request.method, headers:Object.fromEntries(request.headers), bodyBase64:originalBody, service:item.service ?? 'execute-api', tokenless:item.tokenless ?? false, intentionalDifference:item.intentionalDifference ?? false, signedHeaders:Object.fromEntries(signed.headers), signedBodyBase64:Buffer.from(await signed.arrayBuffer()).toString('base64') });
}
const directory = new URL('../../signer/testdata/', import.meta.url);
mkdirSync(directory,{ recursive:true });
writeFileSync(new URL('typescript-v2.35.0.json',directory), JSON.stringify({ source:'@aws-lambda-powertools/signer@2.35.0', timestamp, credentials, cases:results },null,2)+'\n');
console.log(`Generated ${results.length} Signer reference cases`);
