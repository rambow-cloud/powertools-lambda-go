import { Logger } from '@aws-lambda-powertools/logger';
import { writeFileSync } from 'node:fs';

// Only development uses Node.js; fixture generation does not ship in Lambda.
for (const key of ['POWERTOOLS_LOG_LEVEL','LOG_LEVEL','AWS_LAMBDA_LOG_LEVEL','POWERTOOLS_LOGGER_SAMPLE_RATE','POWERTOOLS_DEV','POWERTOOLS_LOGGER_LOG_EVENT','TZ','_X_AMZN_TRACE_ID']) delete process.env[key];
const stdout = process.stdout.write;
const stderr = process.stderr.write;
let lines = '';
const collect = (chunk, encoding, callback) => {
  lines += chunk.toString();
  if (typeof encoding === 'function') encoding();
  if (typeof callback === 'function') callback();
  return true;
};
process.stdout.write = collect;
process.stderr.write = collect;
try {
  const logger = new Logger({ serviceName: 'orders', persistentLogAttributes: { source: 'base' } });
  logger.info('plain');
  logger.appendKeys({ order: 7 });
  logger.info('attributes', { source: 'call' });
  logger.removeKeys(['order']);
  logger.debug('filtered');
  logger.warn('warning');
} finally {
  process.stdout.write = stdout;
  process.stderr.write = stderr;
}
const records = lines.trim().split('\n').map(line => {
  const record = JSON.parse(line);
  delete record.timestamp;
  return record;
});
writeFileSync(new URL('../../logger/testdata/typescript-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', records }, null, 2) + '\n');
