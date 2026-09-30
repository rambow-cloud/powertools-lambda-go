// Execute standard queries, Powertools functions and every upstream envelope.
import { mkdirSync, writeFileSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import { search } from '@aws-lambda-powertools/jmespath';
import { PowertoolsFunctions } from '@aws-lambda-powertools/jmespath/functions';
import * as envelopes from '@aws-lambda-powertools/jmespath/envelopes';

const base64 = value => Buffer.from(value).toString('base64');
const s3 = { Records:[{s3:{bucket:{name:'fixture'}}},{ignored:true}] };
const sns = { Message:JSON.stringify(s3) };
const sqs = value => ({Records:[{body:JSON.stringify(value)}]});
const firehose = value => ({records:[{data:base64(JSON.stringify(value))}]});
const eventFixtures = {
  API_GATEWAY_HTTP:{body:'{"id":"http"}'}, API_GATEWAY_REST:{body:'{"id":"rest"}'},
  SQS:{Records:[{body:'{"id":1}'},{body:'{"id":2}'}]},
  SNS:{Records:[{Sns:{Message:'{"id":1}'}},{Sns:{Message:'{"id":2}'}}]},
  EVENTBRIDGE:{detail:{id:1}}, CLOUDWATCH_EVENTS_SCHEDULED:{detail:{}},
  KINESIS_DATA_STREAM:{Records:[{kinesis:{data:base64('{"id":1}')}}]},
  CLOUDWATCH_LOGS:{awslogs:{data:gzipSync('{"logEvents":[{"id":"log","message":"hello"}]}').toString('base64')}},
  S3_SNS_SQS:sqs(sns), S3_SQS:sqs(s3), S3_SNS_KINESIS_FIREHOSE:firehose(sns),
  S3_KINESIS_FIREHOSE:firehose(s3), S3_EVENTBRIDGE_SQS:sqs({detail:{id:'detail'}}),
};
const input = {items:[{name:'b',n:2},{name:'a',n:1},{name:'c',n:3}], nested:[[1,2],[3]], text:'hello', nil:null};
const expressions = [
  'items[*].name','items[?n >= `2`].name','items[-1].name','items[0:3:2].n','items[::-1].name',
  'nested[]','items[*].missing','items[9]','missing || text','nil && text','!nil',
  'items | [0].name','{names:items[*].name, count:length(items)}','[text,nil,missing]',
  'abs(`-3`)','avg(items[*].n)','ceil(`1.2`)','contains(text, `"ell"`)','ends_with(text, `"lo"`)',
  'floor(`1.8`)','join(`","`,items[*].name)','sort(keys(@))','length(`"你好"`)',
  'map(&n,items)','max(items[*].n)','max_by(items,&n).name','merge(`{"a":1}`,`{"a":2,"b":3}`)',
  'min(items[*].n)','min_by(items,&n).name','not_null(missing,nil,text)','reverse(text)',
  'sort(items[*].name)','sort_by(items,&n)[*].name','starts_with(text, `"he"`)','sum(items[*].n)',
  'to_array(text)','to_number(`"12.5"`)','to_string(`42`)','type(nil)','sort(values(`{"a":2,"b":1}`))',
  'avg(`[]`)','max(`[]`)','min(`[]`)','sort(`[]`)','sum(`[]`)','to_number(`true`)',
  'items[?n == `2`].name','`0` || `1`','`[]` || text','"text"',
];
const cases = expressions.map((expression,i) => ({name:`standard-${i}`,expression,data:input}));
for (const [name,data] of Object.entries(eventFixtures)) cases.push({name:`envelope-${name}`,envelope:name,expression:envelopes[name],data,powertools:true});
for (const [name,expression,data] of [
  ['json','powertools_json(@)','{"name":"value","nested":[1,null]}'],
  ['base64','powertools_base64(@)',base64('hello 世界')],
  ['bom','powertools_base64(@)',base64('\ufeffhello')],
  ['gzip','powertools_base64_gzip(@)',gzipSync('hello 世界').toString('base64')],
  ['invalid-json','powertools_json(@)','bad'],['invalid-base64','powertools_base64(@)','a'],
  ['invalid-gzip','powertools_base64_gzip(@)',base64('not gzip')],
]) cases.push({name,expression,data,powertools:true});
for (const expression of ['', 'items[', 'unknown(text)', 'length()', 'length(text,text)', 'length(`1`)', 'sum(`[1,"bad"]`)', 'powertools_json(text)']) cases.push({name:`error-${cases.length}`,expression,data:input});
for (const item of cases) {
  try {
    item.expected = search(item.expression,item.data,item.powertools ? {customFunctions:new PowertoolsFunctions()} : undefined);
    if (item.expected === undefined) item.expectedUndefined = true;
  }
  catch (error) { item.error = error.constructor.name; }
}
const directory = new URL('../../jmespath/testdata/',import.meta.url);
mkdirSync(directory,{recursive:true});
writeFileSync(new URL('typescript-v2.35.0.json',directory),JSON.stringify({source:'@aws-lambda-powertools/jmespath@2.35.0',cases},null,2)+'\n');
console.log(`Generated ${cases.length} JMESPath reference cases`);
