// Capture configuration/timezone/sink behavior from the actual pinned Logger.
import { Logger } from '@aws-lambda-powertools/logger';
import { writeFileSync } from 'node:fs';
const keys = ['TZ', 'POWERTOOLS_LOG_LEVEL', 'LOG_LEVEL', 'AWS_LAMBDA_LOG_LEVEL', 'POWERTOOLS_LOGGER_SAMPLE_RATE', 'POWERTOOLS_DEV', 'POWERTOOLS_SERVICE_NAME', '_X_AMZN_TRACE_ID'];
const OriginalDate = Date;
let instant;
globalThis.Date = class extends OriginalDate { constructor(...args) { super(...(args.length ? args : [instant])); } };
const cases = [];
const specs = [
  ...['UTC', 'Asia/Hong_Kong', 'America/New_York', 'Europe/London', 'Asia/Kathmandu', 'Etc/GMT+5', 'Etc/UTC', 'Not/AZone'].flatMap(zone => ['2026-01-01T00:00:00.123Z', '2026-07-01T00:00:00.123Z'].map(now => ({ name: `${zone}-${now}`, now, env: { TZ: zone } }))),
  ...[
    ['invalid-level', { POWERTOOLS_LOG_LEVEL: 'invalid' }],
    ['lowercase-level', { POWERTOOLS_LOG_LEVEL: 'debug' }],
    ['alias', { LOG_LEVEL: 'DEBUG' }],
    ['primary', { POWERTOOLS_LOG_LEVEL: 'WARN', LOG_LEVEL: 'DEBUG' }],
    ['invalid-primary', { POWERTOOLS_LOG_LEVEL: 'invalid', LOG_LEVEL: 'DEBUG' }],
    ['invalid-rate', { POWERTOOLS_LOGGER_SAMPLE_RATE: 'invalid' }],
    ['negative-rate', { POWERTOOLS_LOGGER_SAMPLE_RATE: '-1' }],
    ['large-rate', { POWERTOOLS_LOGGER_SAMPLE_RATE: '2' }],
    ['alc-fatal', { AWS_LAMBDA_LOG_LEVEL: 'FATAL', POWERTOOLS_LOG_LEVEL: 'DEBUG' }],
    ['alc-invalid', { AWS_LAMBDA_LOG_LEVEL: 'invalid' }],
  ].map(([name, env]) => ({ name, env, now: '2026-01-01T00:00:00.123Z' })),
];
try {
  for (const spec of specs) {
    keys.forEach(key => delete process.env[key]);
    Object.assign(process.env, spec.env);
    instant = spec.now;
    const originalOut = process.stdout.write, originalErr = process.stderr.write;
    const records = [], diagnostics = [];
    const collect = sink => chunk => {
      const text = chunk.toString().trim();
      try { records.push({ sink, record: JSON.parse(text) }); } catch { diagnostics.push(text); }
      return true;
    };
    process.stdout.write = collect('stdout'); process.stderr.write = collect('stderr');
    try {
      const logger = new Logger({ serviceName: 'audit' });
      for (const level of ['trace', 'debug', 'info', 'warn', 'error', 'critical']) logger[level](level);
      cases.push({ ...spec, level: logger.getLevelName(), records, diagnostics });
    } catch (error) {
      cases.push({ ...spec, error: { name: error.name, message: error.message }, records, diagnostics });
    } finally { process.stdout.write = originalOut; process.stderr.write = originalErr; }
  }
  writeFileSync(new URL('../../logger/testdata/boundaries-v2.35.0.json', import.meta.url), JSON.stringify({ version: '2.35.0', cases }, null, 2) + '\n');
  console.log(`${cases.length} Logger configuration/timezone/sink cases`);
} finally { globalThis.Date = OriginalDate; }
