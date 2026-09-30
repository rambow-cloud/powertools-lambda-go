// Capture pinned HTTP middleware calls through a recording Tracer contract.
// This does not run or validate the retired X-Ray SDK.
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { Router, BadRequestError } from '@aws-lambda-powertools/event-handler/http';
import { tracer as middleware } from '@aws-lambda-powertools/event-handler/http/middleware/tracer';
import { compress } from '@aws-lambda-powertools/event-handler/http/middleware';

process.env.POWERTOOLS_DEV = 'false';
const logger = { debug() {}, warn() {}, error() {} };
const inputs = JSON.parse(readFileSync(new URL('../../eventhandler/http/metrics/testdata/metrics-v2.35.0.json', import.meta.url))).cases;
const cases = [];
for (const kind of ['v1', 'v2', 'alb', 'url']) {
  for (const action of ['json', 'text', 'charset', 'invalid-json', 'http-error', 'error', 'missing', 'server', 'disabled', 'capture-off', 'config-off', 'compressed', 'length', 'length-prefix', 'length-negative', 'length-invalid']) {
    for (const metadata of ['full', 'missing']) {
      const event = inputs.find(item => item.name === `${kind}-${action === 'missing' ? 'missing' : 'ok'}-${metadata}`).event;
      const recorded = { spans: [], responses: [], errors: [], annotations: [] };
      const root = { addNewSubsegment(name) {
        const child = { name, close() { child.closed = true; } };
        recorded.spans.push(child);
        return child;
      } };
      let current = root;
      const tracer = {
        isTracingEnabled: () => action !== 'disabled', getSegment: () => current, setSegment: value => { current = value; },
        annotateColdStart() { recorded.annotations.push('ColdStart'); }, addServiceNameAnnotation() { recorded.annotations.push('Service'); },
        addResponseAsMetadata(value, name) { if (action !== 'config-off') recorded.responses.push({ name, value }); },
        addErrorAsMetadata(error) { recorded.errors.push(error.name); },
      };
      const app = new Router({ logger });
      app.use(middleware(tracer, { captureResponse: action !== 'capture-off', logger }));
      if (action === 'compressed') app.use(compress({ threshold: 0 }));
      app.get('/items/:id', () => {
        if (action === 'http-error') throw new BadRequestError('bad input');
        if (action === 'error') throw new Error('business failure');
        const headers = { 'Content-Type': action === 'text' ? 'text/plain' : action === 'charset' ? 'application/json; charset=utf-8' : 'application/json' };
        const lengths = { length: '12', 'length-prefix': '+12trailing', 'length-negative': '-2', 'length-invalid': 'invalid' };
        if (action in lengths) headers['Content-Length'] = lengths[action];
        return new Response(action === 'invalid-json' ? '{' : action === 'text' ? 'hello' : '{"ok":true}', { status: action === 'server' ? 503 : 200, headers });
      });
      const response = await app.resolve(event, {});
      cases.push({ name: `${kind}-${action}-${metadata}`, action, event, status: response.statusCode, ...recorded, restored: current === root });
    }
  }
}
const directory = new URL('../../eventhandler/http/tracer/testdata/', import.meta.url);
mkdirSync(directory, { recursive: true });
writeFileSync(new URL('tracer-v2.35.0.json', directory), JSON.stringify({ upstream: '2.35.0', scope: 'Actual Router and HTTP tracer middleware; recording tracer contract, no X-Ray SDK. Go maps these calls to OTel.', cases }, null, 2) + '\n');
console.log(`${cases.length} HTTP Tracer reference cases`);
