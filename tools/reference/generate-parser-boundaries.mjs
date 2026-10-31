import { readFileSync, writeFileSync } from 'node:fs';
import { parse } from '@aws-lambda-powertools/parser';
import * as envelopes from '@aws-lambda-powertools/parser/envelopes';
import { z } from 'zod';
import { issues } from './parser-issues.mjs';

const names = {
  sqs:'SqsEnvelope', eventbridge:'EventBridgeEnvelope', sns:'SnsEnvelope',
  snssqs:'SnsSqsEnvelope', kinesis:'KinesisEnvelope', firehose:'KinesisFirehoseEnvelope',
  cloudwatch:'CloudWatchEnvelope', dynamodb:'DynamoDBStreamEnvelope', kafka:'KafkaEnvelope',
  apigateway:'ApiGatewayEnvelope', apigatewayv2:'ApiGatewayV2Envelope',
  lambdaurl:'LambdaFunctionUrlEnvelope', lattice:'VpcLatticeEnvelope', latticev2:'VpcLatticeV2Envelope',
};
const cases=[];
for(const [name,exportName]of Object.entries(names)) {
  if(!envelopes[exportName])throw new Error(`Missing pinned envelope ${exportName}`);
  for(const [kind,input]of Object.entries({missing:{},scalar:0,array:[]})) {
    for(const safe of [false,true]) {
      const item={name:`boundary-${name}-${kind}-${safe}`,schema:'unknown',envelope:name,input,safe};
      try {
        const result=parse(input,envelopes[exportName],z.unknown(),safe);
        item.expected=safe&&!result.success
          ?{success:false,issues:issues(result.error),original:result.originalEvent}
          :{success:true,data:safe?result.data:result};
      }catch(error){item.expected={success:false,thrown:true,error:error.name,message:error.message,issues:issues(error)};}
      cases.push(item);
    }
  }
}
writeFileSync(new URL('../../parser/testdata/boundaries-v2.35.0.json',import.meta.url),JSON.stringify({version:'2.35.0',zodVersion:JSON.parse(readFileSync(new URL('./node_modules/zod/package.json',import.meta.url),'utf8')).version,cases},null,2)+'\n');
console.log(`${cases.length} invalid-input observations across ${Object.keys(names).length} envelopes`);
