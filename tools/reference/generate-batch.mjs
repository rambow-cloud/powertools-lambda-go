// Execute pinned Batch processors without AWS calls.
import { mkdirSync, writeFileSync } from 'node:fs';
import { BatchProcessor, BatchProcessorSync, SqsFifoPartialProcessorAsync, SqsFifoPartialProcessor, EventType, processPartialResponse } from '@aws-lambda-powertools/batch';

const records = (source, entries) => entries.map(([id,fail=false,group]) => {
  const common = {fixtureId:id, fail};
  if (source === 'sqs') return {...common,messageId:id,body:JSON.stringify({id}),attributes:group === undefined ? {} : {MessageGroupId:group}};
  if (source === 'kinesis') return {...common,kinesis:{sequenceNumber:id,data:Buffer.from(JSON.stringify({id})).toString('base64')}};
  return {...common,dynamodb:{SequenceNumber:id}};
});
const scenarios = [];
for (const source of ['sqs','kinesis','dynamodb']) {
  for (const sync of [false,true]) {
    for (const [name, entries, suppress=false] of [
      ['empty',[]],['success',[['1'],['2']]],['partial',[['1'],['2',true],['3'],['4',true]]],
      ['all-failed',[['1',true],['2',true]]],['suppressed',[['1',true],['2',true]],true],
    ]) scenarios.push({name:`${source}-${sync?'sync':'async'}-${name}`,source,sync,records:records(source,entries),suppress});
  }
}
for (const sync of [false,true]) {
  for (const [name, entries, skip=false, suppress=false] of [
    ['stop',[['1',false,'a'],['2',true,'a'],['3',false,'b'],['4',false,'a']]],
    ['groups',[['1',true,'a'],['2',false,'b'],['3',false,'a'],['4',true,'b'],['5',false,'b'],['6',false,'c']],true],
    ['all',[['1',true,'a'],['2',false,'a'],['3',false,'b']]],
    ['suppressed',[['1',true,'a'],['2',false,'a']],false,true],
    ['missing-group',[['1',true],['2'],['3',false,'a']],true],
    ['empty-group',[['1',true,''],['2',false,''],['3',false,'a']],true],
  ]) scenarios.push({name:`fifo-${sync?'sync':'async'}-${name}`,source:'sqs',fifo:true,sync,records:records('sqs',entries),skip,suppress});
}
scenarios.push({name:'dynamodb-empty-sequence',source:'dynamodb',records:records('dynamodb',[['1'],['',true]]),suppress:true});
const result = [];
for (const scenario of scenarios) {
  const source = {sqs:EventType.SQS,kinesis:EventType.KinesisDataStreams,dynamodb:EventType.DynamoDBStreams}[scenario.source];
  const processor = scenario.fifo ? (scenario.sync ? new SqsFifoPartialProcessor() : new SqsFifoPartialProcessorAsync()) : (scenario.sync ? new BatchProcessorSync(source) : new BatchProcessor(source));
  const visited = [];
  const handler = record => { visited.push(record.fixtureId); if (record.fail) throw new Error(`failed:${record.fixtureId}`); return `ok:${record.fixtureId}`; };
  processor.register(scenario.records, scenario.sync ? handler : async record => handler(record), {processInParallel:false,skipGroupOnError:scenario.skip,throwOnFullBatchFailure:!scenario.suppress});
  let failure;
  try { if (scenario.sync) processor.processSync(); else await processor.process(); } catch(error) { failure = error.name; }
  result.push({...scenario,expected:{response:processor.response(),visited,successes:processor.successMessages.map(r=>r.fixtureId),failures:processor.failureMessages.map(r=>r.fixtureId),errorTypes:processor.errors.map(e=>e.name),error:failure??null}});
}
const invalidEvents = [];
for (const event of [{},{Records:null},{Records:{}},{Records:'invalid'}]) {
  try { await processPartialResponse(event,async()=>{},new BatchProcessor(EventType.SQS)); }
  catch(error) { invalidEvents.push({event,error:error.name}); }
}
const directory = new URL('../../batch/testdata/',import.meta.url);
mkdirSync(directory,{recursive:true});
writeFileSync(new URL('typescript-v2.35.0.json',directory),JSON.stringify({source:'@aws-lambda-powertools/batch@2.35.0',cases:result,invalidEvents},null,2)+'\n');
console.log(`Generated ${result.length} Batch cases and ${invalidEvents.length} invalid envelopes`);
