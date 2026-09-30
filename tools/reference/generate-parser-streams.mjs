// Execute stream/notification schemas and envelopes from the pinned distribution.
import { writeFileSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import { z } from 'zod';
import { parse } from '@aws-lambda-powertools/parser';
import { JSONStringified } from '@aws-lambda-powertools/parser/helpers';
import { DynamoDBMarshalled } from '@aws-lambda-powertools/parser/helpers/dynamodb';
import * as sns from '@aws-lambda-powertools/parser/schemas/sns';
import * as ddb from '@aws-lambda-powertools/parser/schemas/dynamodb';
import * as kinesis from '@aws-lambda-powertools/parser/schemas/kinesis';
import * as firehose from '@aws-lambda-powertools/parser/schemas/kinesis-firehose';
import * as cloudwatch from '@aws-lambda-powertools/parser/schemas/cloudwatch';
import { SnsEnvelope, SnsSqsEnvelope, KinesisEnvelope, KinesisFirehoseEnvelope, CloudWatchEnvelope, DynamoDBStreamEnvelope } from '@aws-lambda-powertools/parser/envelopes';

const order=z.object({id:z.string(),amount:z.number().refine(value=>value>=0,{message:'amount must be non-negative'}),retry:z.number().default(3),note:z.string().nullable().optional()});
const schemas={...sns,...ddb,...kinesis,...firehose,...cloudwatch,order,json:JSONStringified(order),text:z.string(),unknown:z.unknown(),marshalled:DynamoDBMarshalled(order),rawMarshalled:DynamoDBMarshalled(z.unknown())};
const envelopes={sns:SnsEnvelope,snssqs:SnsSqsEnvelope,kinesis:KinesisEnvelope,firehose:KinesisFirehoseEnvelope,cloudwatch:CloudWatchEnvelope,dynamodb:DynamoDBStreamEnvelope};
const cases=[];
function normalize(value){
 if(typeof value==='bigint')return {bigint:String(value)};
 if(typeof value==='number'&&!Number.isFinite(value))return {number:String(value)};
 if(value instanceof Set)return [...value].map(normalize);
 if(Array.isArray(value))return value.map(normalize);
 if(value&&typeof value==='object')return Object.fromEntries(Object.entries(value).filter(([,value])=>value!==undefined).map(([key,value])=>[key,normalize(value)]));
 return value;
}
import { issues } from './parser-issues.mjs';
function add(name,schema,input,envelope,safe=true){
 const item={name,schema,input,envelope,safe};
 try{const result=parse(input,envelopes[envelope],schemas[schema],safe);item.expected=safe&&!result.success?{success:false,issues:issues(result.error),original:normalize(result.originalEvent)}:{success:true,data:normalize(safe?result.data:result)};}
 catch(error){item.expected={success:false,thrown:true,error:error.name,issues:issues(error)};}
 cases.push(item);
}
const body=JSON.stringify({id:'a',amount:1});
const notification={Subject:null,TopicArn:'arn:topic',UnsubscribeUrl:'https://example.test/unsubscribe',SigningCertUrl:'https://example.test/cert',Type:'Notification',Message:body,MessageId:'1',Timestamp:'2026-09-15T00:00:00Z',MessageAttributes:{tag:{Type:'String',Value:'value'}},ignored:true};
const snsRecord={EventSource:'aws:sns',EventVersion:'1',EventSubscriptionArn:'arn:subscription',Sns:notification};
const sqsNotification={...notification,UnsubscribeURL:'not a URL',SigningCertURL:'https://example.test/cert'};delete sqsNotification.UnsubscribeUrl;delete sqsNotification.SigningCertUrl;
const sqsRecord={messageId:'1',receiptHandle:'r',body:JSON.stringify(sqsNotification),attributes:{ApproximateReceiveCount:'1',ApproximateFirstReceiveTimestamp:'1',SenderId:'s',SentTimestamp:'1'},messageAttributes:{},md5OfBody:'hash',eventSource:'aws:sqs',eventSourceARN:'arn:queue',awsRegion:'ap-east-1'};
const image={id:{S:'a'},amount:{N:'1'}};
const change={ApproximateCreationDateTime:1,Keys:{id:{S:'a'}},NewImage:image,OldImage:{id:{S:'old'},amount:{N:'0'}},SequenceNumber:'90071992547409930001',SizeBytes:10,StreamViewType:'NEW_AND_OLD_IMAGES'};
const identity={type:'Service',principalId:'dynamodb.amazonaws.com'};
const ddbRecord={eventID:'1',eventName:'MODIFY',eventVersion:'1',eventSource:'aws:dynamodb',awsRegion:'ap-east-1',eventSourceARN:'arn:stream',dynamodb:change,userIdentity:identity};
const changeToKinesis={...change};delete changeToKinesis.SequenceNumber;delete changeToKinesis.StreamViewType;
const ddbToKinesis={...ddbRecord,dynamodb:changeToKinesis,recordFormat:'application/json',tableName:'orders',userIdentity:null};delete ddbToKinesis.eventVersion;delete ddbToKinesis.eventSourceARN;
const encode=value=>Buffer.from(typeof value==='string'?value:JSON.stringify(value)).toString('base64');
const payload={kinesisSchemaVersion:'1',partitionKey:'key',sequenceNumber:'90071992547409930001',approximateArrivalTimestamp:1,data:encode(body)};
const kinesisRecord={eventSource:'aws:kinesis',eventVersion:'1',eventID:'1',eventName:'aws:kinesis:record',awsRegion:'ap-east-1',invokeIdentityArn:'arn:role',eventSourceARN:'arn:stream',kinesis:payload};
const metadata={shardId:'shard',partitionKey:'key',approximateArrivalTimestamp:1,sequenceNumber:'1',subsequenceNumber:0};
const firehoseRecord={recordId:'1',approximateArrivalTimestamp:1,kinesisRecordMetadata:metadata,data:encode(body)};
const firehoseEvent={invocationId:'1',deliveryStreamArn:'arn:delivery',region:'ap-east-1',records:[firehoseRecord]};
const log={id:'1',timestamp:1,message:body};
const logs={messageType:'DATA_MESSAGE',owner:'owner',logGroup:'group',logStream:'stream',subscriptionFilters:[],logEvents:[log]};
const zipped=value=>gzipSync(JSON.stringify(value)).toString('base64');
const logEvent={awslogs:{data:zipped(logs)}};
const samples={SnsNotificationSchema:notification,SnsSqsNotificationSchema:sqsNotification,SnsRecordSchema:snsRecord,SnsSchema:{Records:[snsRecord]},DynamoDBStreamChangeRecordBase:change,DynamoDBStreamToKinesisChangeRecord:changeToKinesis,DynamoDBStreamChangeRecord:change,UserIdentity:identity,DynamoDBStreamRecord:ddbRecord,DynamoDBStreamToKinesisRecord:ddbToKinesis,DynamoDBStreamSchema:{Records:[ddbRecord]},KinesisDataStreamRecordPayload:payload,KinesisDataStreamRecord:kinesisRecord,KinesisDataStreamSchema:{Records:[kinesisRecord]},KinesisDynamoDBStreamSchema:{Records:[{...kinesisRecord,kinesis:{...payload,data:encode(ddbToKinesis)}}]},KinesisFirehoseRecordSchema:firehoseRecord,KinesisFirehoseSqsRecordSchema:{...firehoseRecord,data:encode(sqsRecord)},KinesisFirehoseSchema:firehoseEvent,KinesisFirehoseSqsSchema:{...firehoseEvent,records:[{...firehoseRecord,data:encode(sqsRecord)}]},CloudWatchLogEventSchema:log,CloudWatchLogsDecodeSchema:logs,CloudWatchLogsSchema:logEvent};
for(const [schema,input] of Object.entries(samples)){add(schema+'-valid',schema,input);add(schema+'-missing',schema,{});}
for(const [envelope,input,schema] of [['sns',{Records:[snsRecord]},'json'],['snssqs',{Records:[sqsRecord]},'json'],['kinesis',{Records:[kinesisRecord]},'order'],['firehose',firehoseEvent,'json'],['cloudwatch',logEvent,'json'],['dynamodb',{Records:[ddbRecord]},'order']]){
 add(envelope+'-safe',schema,input,envelope);add(envelope+'-ordinary',schema,input,envelope,false);add(envelope+'-invalid-outer',schema,{},envelope);
}
const badBody=JSON.stringify({id:1,amount:-1});
for(const safe of [true,false]){
 add('sns-multiple-'+safe,'json',{Records:[{...snsRecord,Sns:{...notification,Message:badBody}},{...snsRecord,Sns:{...notification,Message:badBody}}]},'sns',safe);
 add('snssqs-multiple-'+safe,'json',{Records:[{...sqsRecord,body:'{broken'},{...sqsRecord,body:JSON.stringify({...sqsNotification,Message:badBody})}]},'snssqs',safe);
 add('kinesis-multiple-'+safe,'order',{Records:[{...kinesisRecord,kinesis:{...payload,data:encode(badBody)}},{...kinesisRecord,kinesis:{...payload,data:encode(badBody)}}]},'kinesis',safe);
 add('firehose-multiple-'+safe,'json',{...firehoseEvent,records:[{...firehoseRecord,data:encode(badBody)},{...firehoseRecord,data:encode(badBody)}]},'firehose',safe);
 add('cloudwatch-multiple-'+safe,'json',{awslogs:{data:zipped({...logs,logEvents:[{...log,message:badBody},{...log,message:badBody}]})}},'cloudwatch',safe);
 const invalidImage={id:{N:'1'},amount:{N:'-1'}};
 add('dynamodb-multiple-'+safe,'order',{Records:[{...ddbRecord,dynamodb:{...change,NewImage:invalidImage,OldImage:invalidImage}},{...ddbRecord,dynamodb:{...change,NewImage:invalidImage}}]},'dynamodb',safe);
}
add('sns-no-implicit-json','order',{Records:[snsRecord]},'sns');
add('firehose-no-implicit-json','order',firehoseEvent,'firehose');
add('cloudwatch-no-implicit-json','order',logEvent,'cloudwatch');
add('kinesis-text','text',{Records:[{...kinesisRecord,kinesis:{...payload,data:encode('text')}}]},'kinesis');
add('kinesis-gzip','order',{Records:[{...kinesisRecord,kinesis:{...payload,data:zipped({id:'a',amount:1})}}]},'kinesis');
add('kinesis-ddb-empty','KinesisDynamoDBStreamSchema',{Records:[]});
add('kinesis-empty','KinesisDataStreamSchema',{Records:[]});
add('ddb-keys-only','order',{Records:[{...ddbRecord,dynamodb:{Keys:{id:{S:'a'}},SequenceNumber:'1',SizeBytes:1,StreamViewType:'KEYS_ONLY'}}]},'dynamodb');
for(const field of ['Keys','NewImage','OldImage'])add('ddb-bad-'+field,'DynamoDBStreamSchema',{Records:[{...ddbRecord,dynamodb:{...change,[field]:{id:{INVALID:true}}}}]});
add('ddb-invalid-enums','DynamoDBStreamRecord',{...ddbRecord,eventName:'OTHER',userIdentity:{type:'Other',principalId:'x'},dynamodb:{...change,StreamViewType:'OTHER'}});
add('ddb-window','DynamoDBStreamSchema',{Records:[ddbRecord],window:{start:'2026-09-15T00:00Z',end:'2026-09-15T00:01Z'},state:{key:'value'},shardId:'s',eventSourceARN:'arn',isFinalInvokeForWindow:false,isWindowTerminatedEarly:true});
add('kinesis-window','KinesisDataStreamSchema',{Records:[kinesisRecord],state:{key:{nested:1}},window:{start:'2026-09-15T00:00Z',end:'2026-09-15T00:01Z'}});
for(const value of ['!','abcd','YWJj','YQ==']){
 add('firehose-base64-'+value,'KinesisFirehoseRecordSchema',{...firehoseRecord,data:value});
 add('cloudwatch-data-'+value,'CloudWatchLogsSchema',{awslogs:{data:value}});
}
add('firehose-zero','KinesisFirehoseRecordSchema',{...firehoseRecord,approximateArrivalTimestamp:0,kinesisRecordMetadata:{...metadata,approximateArrivalTimestamp:-1}});
add('firehose-sqs-invalid','KinesisFirehoseSqsRecordSchema',{...firehoseRecord,data:encode({})});
add('firehose-sqs-unpadded','KinesisFirehoseSqsRecordSchema',{...firehoseRecord,data:encode(sqsRecord).replaceAll('=','')});
add('cloudwatch-empty-log-events','CloudWatchLogsSchema',{awslogs:{data:zipped({...logs,logEvents:[]})}});
add('sns-invalid-url-time','SnsNotificationSchema',{...notification,UnsubscribeUrl:'relative',Timestamp:'bad'});
add('sns-uppercase-only','SnsNotificationSchema',sqsNotification);
add('ddb-helper','marshalled',image);
add('ddb-helper-invalid','marshalled',{id:{INVALID:true}});
add('ddb-helper-invalid-output','marshalled',{id:{N:'1'},amount:{N:'-1'}});
add('ddb-helper-bigint','rawMarshalled',{large:{N:'9007199254740993'},raw:{B:'YQ=='},set:{SS:['a','a','b']},nested:{M:{x:{NULL:true}}}});
writeFileSync('../../parser/testdata/streams-v2.35.0.json',JSON.stringify({version:'2.35.0',zodVersion:'4.1.12',cases},null,2)+'\n');
console.log(`${cases.length} stream Parser cases; ${Object.keys(samples).length} schema exports`);
