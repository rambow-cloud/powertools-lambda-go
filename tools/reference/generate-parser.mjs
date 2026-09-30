// Execute the pinned Parser/Zod distributions and inventory public subpaths.
import { readFileSync, readdirSync, mkdirSync, writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';
import { gzipSync } from 'node:zlib';
import { z } from 'zod';
import { parse } from '@aws-lambda-powertools/parser';
import { JSONStringified, Base64Encoded } from '@aws-lambda-powertools/parser/helpers';
import { SqsEnvelope, EventBridgeEnvelope } from '@aws-lambda-powertools/parser/envelopes';
import { SqsSchema, EventBridgeSchema, SqsMsgAttributeDataTypeSchema } from '@aws-lambda-powertools/parser/schemas';

const order = z.object({ id: z.string(), amount: z.number().refine(value => value >= 0, { message: 'amount must be non-negative' }), retry: z.number().default(3), note: z.string().nullable().optional() });
const schemas = { order, json: JSONStringified(order), base64: Base64Encoded(order), text: z.string(), strict: order.strict(), passthrough: order.passthrough(), transform: order.transform(value => ({ ...value, id: value.id.toUpperCase() })), sqs: SqsSchema, eventbridge: EventBridgeSchema, dataType: SqsMsgAttributeDataTypeSchema };
const cases = [];
import { issues } from './parser-issues.mjs';
function add(name, schema, input, envelope, safe = true) {
  const item = { name, schema, input, envelope, safe };
  try {
    const result = parse(input, envelope === 'sqs' ? SqsEnvelope : envelope === 'eventbridge' ? EventBridgeEnvelope : undefined, schemas[schema], safe);
    if (safe && !result.success) item.expected = { success: false, issues: issues(result.error), original: result.originalEvent };
    else item.expected = { success: true, data: safe ? result.data : result };
  } catch (error) { item.expected = { success: false, thrown: true, error: error.name, issues: issues(error) }; }
  cases.push(item);
}
for (const input of [{ id: 'a', amount: 1, extra: true }, { id: 'a', amount: -1 }, { id: 1, amount: 'x' }, {}, null, { id: 'a', amount: 0, note: null }, { id: 'a', amount: 1, retry: null }]) {
  add(`order-${cases.length}`, 'order', input);
}
add('strict-unknown','strict',{ id: 'a', amount: 1, extra: true });
add('passthrough','passthrough',{ id: 'a', amount: 1, extra: { enabled: true } });
add('transform','transform',{ id: 'a', amount: 1 });
add('json-success','json','{"id":"a","amount":1}');
add('json-invalid','json','{broken');
add('json-wrong-type','json',{id:'a'});
add('base64-json','base64',Buffer.from('{"id":"a","amount":1}').toString('base64'));
add('base64-gzip','base64',gzipSync('{"id":"a","amount":1}').toString('base64'));
add('base64-text-schema','base64',Buffer.from('plain text').toString('base64'));
for (const value of ['String','Number','Binary','custom',1,null]) add(`data-type-${String(value)}`,'dataType',value);

const record = (body, id = '1') => ({ messageId: id, receiptHandle: 'receipt', body, attributes: { ApproximateReceiveCount:'1', ApproximateFirstReceiveTimestamp:'1', SenderId:'sender', SentTimestamp:'1', DeadLetterQueueSourceArn:'arn:queue' }, messageAttributes: {}, md5OfBody:'hash', eventSource:'aws:sqs', eventSourceARN:'arn:queue', awsRegion:'ap-east-1' });
const valid = { Records:[record('{"id":"a","amount":1}')], ignored:true };
add('sqs-schema','sqs',valid);
add('sqs-envelope','json',valid,'sqs');
add('sqs-no-implicit-json','order',valid,'sqs');
add('sqs-text','text',{ Records:[record('text')] },'sqs');
add('sqs-empty','sqs',{Records:[]});
add('sqs-missing-records','sqs',{});
const attributes = structuredClone(valid); attributes.Records[0].messageAttributes = { custom:{dataType:'custom',stringValue:null,binaryValue:null,stringListValues:['a']} }; add('sqs-attributes','sqs',attributes);
const invalidMetadata=structuredClone(valid); delete invalidMetadata.Records[0].attributes.SenderId; invalidMetadata.Records[0].eventSource='other'; add('sqs-invalid-metadata','json',invalidMetadata,'sqs');
const multiple = { Records:[record('{"id":1,"amount":-1}','1'),record('{"id":"b","amount":-1}','2')] };
add('sqs-safe-collect','json',multiple,'sqs',true);
add('sqs-stop-first','json',multiple,'sqs',false);

const event = {version:'0',id:'event',source:'source',account:'account',time:'2026-09-15T00:00:00Z',region:'ap-east-1',resources:[], 'detail-type':'order',detail:{id:'a',amount:1}};
add('eventbridge-schema','eventbridge',event);
add('eventbridge-envelope','order',event,'eventbridge');
add('eventbridge-invalid-both','order',{...event,time:'invalid',detail:{id:1,amount:-1}},'eventbridge');
const missingDetail=structuredClone(event);delete missingDetail.detail;add('eventbridge-missing-detail','eventbridge',missingDetail);
for (const time of ['2026-09-15T00:00Z','2026-09-15T00:00:00.123456789Z','2026-02-30T00:00:00Z','2026-09-15T00:00:00+08:00']) add('eventbridge-time-'+time,'eventbridge',{...event,time});

const root=resolve('node_modules/@aws-lambda-powertools/parser/lib/esm');
const inventory=[];
for (const directory of ['', 'schemas','envelopes','helpers','middleware','types']) {
  for (const file of readdirSync(resolve(root,directory)).filter(name=>name.endsWith('.js'))) {
    if (!directory && !['index.js','errors.js'].includes(file)) continue;
    const module=await import(pathToFileURL(resolve(root,directory,file)).href);
    const declaration=readFileSync(resolve(root,directory,file.replace(/\.js$/,'.d.ts')),'utf8');
    const declared=new Set([...declaration.matchAll(/(?:export\s+)?(?:declare\s+)?(?:type|interface|class|function|const)\s+([A-Za-z_$][\w$]*)/g)].map(match=>match[1]));
    for(const match of declaration.matchAll(/export\s+(?:type\s+)?\{([^}]+)\}/g))for(const entry of match[1].split(',')){const name=entry.trim().split(/\s+as\s+/).at(-1);if(/^[A-Za-z_$][\w$]*$/.test(name))declared.add(name);}
    inventory.push({subpath:directory ? `${directory}/${file.replace(/\.js$/,'')}` : file.replace(/\.js$/,''),runtimeExports:Object.keys(module).sort(),declarationNames:[...declared].sort()});
  }
}
mkdirSync('../../parser/testdata',{recursive:true});
writeFileSync('../../parser/testdata/typescript-v2.35.0.json',JSON.stringify({version:'2.35.0',zodVersion:JSON.parse(readFileSync('node_modules/zod/package.json','utf8')).version,cases},null,2)+'\n');
writeFileSync('../../docs/PARSER_EXPORTS.json',JSON.stringify({reference:'2.35.0',note:'Runtime exports are observed by import. Declaration names include supporting local types; re-export aliases and complete behavioral mapping require review.',modules:inventory},null,2)+'\n');
console.log(`${cases.length} parser cases; ${inventory.length} public module files inventoried`);
