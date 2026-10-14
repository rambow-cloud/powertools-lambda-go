// Additional finite scalar boundaries; keep the original corpus unchanged.
import { awsDate, awsTime, awsDateTime, awsTimestamp } from '@aws-lambda-powertools/event-handler/appsync-graphql';
import { writeFileSync } from 'node:fs';

const RealDate = Date;
const cases = [];
try {
  for (const millis of [-8640000000000001, -8640000000000000, -8639999999999999, -1, 0, 1, 8639999999999999, 8640000000000000, 8640000000000001]) {
    globalThis.Date = class extends RealDate {
      constructor(...args) { super(...(args.length ? args : [millis])); }
      static now() { return millis; }
    };
    for (const offset of [0, -12, 14, 0.0000001, 'NaN']) {
      const values = {};
      for (const [name, fn] of Object.entries({ date: awsDate, time: awsTime, datetime: awsDateTime })) {
        values[name] = fn(offset === 'NaN' ? NaN : offset);
      }
      cases.push({ millis, offset, timestamp: awsTimestamp(), values });
    }
  }
} finally {
  globalThis.Date = RealDate;
}
writeFileSync(new URL('../../eventhandler/appsyncgraphql/testdata/boundaries-v2.35.0.json', import.meta.url), JSON.stringify({ upstream: '2.35.0', cases }, null, 2) + '\n');
console.log(`${cases.length} exact scalar boundary scenarios`);
