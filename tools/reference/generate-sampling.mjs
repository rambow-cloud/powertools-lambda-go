import crypto from 'node:crypto';
import { syncBuiltinESMExports } from 'node:module';
import { writeFileSync } from 'node:fs';
import { Logger } from '@aws-lambda-powertools/logger';

for (const key of ['POWERTOOLS_LOG_LEVEL', 'LOG_LEVEL', 'AWS_LAMBDA_LOG_LEVEL', 'POWERTOOLS_LOGGER_SAMPLE_RATE', 'POWERTOOLS_DEV', '_X_AMZN_TRACE_ID']) delete process.env[key];
const originalRandom = crypto.randomInt;
const stdout = process.stdout.write;
const draws = [10, 11, 0];
let count = 0;
const diagnostics = [];
crypto.randomInt = () => draws[count++];
syncBuiltinESMExports();
process.stdout.write = chunk => {
  const { level, message, sampling_rate } = JSON.parse(chunk.toString());
  diagnostics.push({ level, message, sampling_rate });
  return true;
};
let levels;
try {
  const logger = new Logger({ serviceName: 'orders', sampleRateValue: 0.1 });
  levels = [logger.getLevelName()];
  for (let i = 0; i < 3; i++) {
    logger.refreshSampleRateCalculation();
    levels.push(logger.getLevelName());
  }
} finally {
  process.stdout.write = stdout;
  crypto.randomInt = originalRandom;
  syncBuiltinESMExports();
}
writeFileSync(new URL('../../logger/testdata/sampling-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', draws, drawCount: count, levels, diagnostics }, null, 2) + '\n');
