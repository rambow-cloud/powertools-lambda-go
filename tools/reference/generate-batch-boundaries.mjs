// Run the pinned processors; barriers observe their collectors after completion.
import { writeFileSync } from 'node:fs';
import { BatchProcessor, SqsFifoPartialProcessorAsync, EventType } from '@aws-lambda-powertools/batch';

const types = { sqs: EventType.SQS, kinesis: EventType.KinesisDataStreams, dynamodb: EventType.DynamoDBStreams };
const makeRecord = (source, id, fail = false, group = 'a') => ({
  fixtureId: id, fail,
  ...(source === 'sqs' ? { messageId: id, body: '{}', attributes: { MessageGroupId: group } }
    : source === 'kinesis' ? { kinesis: { sequenceNumber: id, data: 'e30=' } }
      : { dynamodb: { SequenceNumber: id } }),
});
const snapshot = (p, results, error) => ({
  response: p.response(), successes: p.successMessages.map(r => r?.fixtureId ?? null),
  failures: p.failureMessages.map(r => r?.fixtureId ?? null),
  results: results?.map(r => ({ status: r[0], id: r[2]?.fixtureId ?? null })) ?? null,
  error: error?.name ?? null,
});
const parallel = [];
for (const source of Object.keys(types)) {
  const records = Array.from({ length: 6 }, (_, n) => makeRecord(source, String(n), [0, 1, 3, 5].includes(n)));
  const gates = records.map(() => Promise.withResolvers());
  const completed = records.map(() => Promise.withResolvers());
  const p = new BatchProcessor(types[source]);
  const original = p.processRecord.bind(p);
  p.processRecord = async record => {
    const result = await original(record);
    completed[Number(record.fixtureId)].resolve();
    return result;
  };
  p.register(records, async r => {
    await gates[Number(r.fixtureId)].promise;
    if (r.fail) throw new Error(`failed:${r.fixtureId}`);
    return `ok:${r.fixtureId}`;
  });
  const processing = p.process();
  for (const n of [1, 2, 3, 0, 5, 4]) {
    gates[n].resolve();
    await completed[n].promise;
  }
  parallel.push({ source, records, expected: snapshot(p, await processing) });
}

const warm = [];
for (const source of Object.keys(types)) {
  for (const fifo of source === 'sqs' ? [false, true] : [false]) {
    const p = fifo ? new SqsFifoPartialProcessorAsync() : new BatchProcessor(types[source]);
    const steps = [];
    for (const records of [
      [makeRecord(source, 'failed', true), makeRecord(source, 'later', false, 'b')],
      [makeRecord(source, 'new-1'), makeRecord(source, 'new-2')], [],
    ]) {
      p.register(records, async r => { if (r.fail) throw new Error('failed'); return r.fixtureId; }, { processInParallel: false, throwOnFullBatchFailure: false });
      steps.push({ records, expected: snapshot(p, await p.process()) });
    }
    warm.push({ source, fifo, steps });
  }
}

const malformed = [];
for (const source of Object.keys(types)) {
  const field = source === 'sqs' ? 'messageId' : source === 'kinesis' ? 'sequenceNumber' : 'SequenceNumber';
  for (const name of ['absent-id', 'empty-id', 'absent-container', 'numeric-id', 'null-record']) {
    let record = makeRecord(source, name, true);
    const container = source === 'sqs' ? record : record[source];
    if (name === 'absent-id') delete container[field];
    if (name === 'empty-id') container[field] = '';
    if (name === 'absent-container') { if (source === 'sqs') delete record.attributes; else delete record[source]; }
    if (name === 'numeric-id') container[field] = 7;
    if (name === 'null-record') record = null;
    const p = new BatchProcessor(types[source]);
    p.register([record], async () => { throw new Error('malformed'); }, { throwOnFullBatchFailure: false });
    let result, error;
    try { result = await p.process(); } catch (e) { error = e; }
    malformed.push({ source, name, records: [record], expected: snapshot(p, result, error) });
  }
}
writeFileSync(new URL('../../batch/testdata/boundaries-v2.35.0.json', import.meta.url), JSON.stringify({ version: '2.35.0', parallel, warm, malformed }, null, 2) + '\n');
console.log(`${parallel.length} parallel, ${warm.length * 3} warm invocation and ${malformed.length} malformed cases`);
