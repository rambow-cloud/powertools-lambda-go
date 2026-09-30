// Execute remaining service schemas and the Kafka envelope from pinned packages.
import { writeFileSync } from 'node:fs';
import { z } from 'zod';
import { parse } from '@aws-lambda-powertools/parser';
import { JSONStringified } from '@aws-lambda-powertools/parser/helpers';
import { KafkaEnvelope } from '@aws-lambda-powertools/parser/envelopes';
import * as kafka from '@aws-lambda-powertools/parser/schemas/kafka';
import * as cloudformation from '@aws-lambda-powertools/parser/schemas/cloudformation-custom-resource';
import * as transfer from '@aws-lambda-powertools/parser/schemas/transfer-family';
import * as connect from '@aws-lambda-powertools/parser/schemas/connect-outbound-campaigns';
import * as ses from '@aws-lambda-powertools/parser/schemas/ses';
import * as s3 from '@aws-lambda-powertools/parser/schemas/s3';

const order=z.object({id:z.string(),amount:z.number().refine(value=>value>=0,{message:'amount must be non-negative'}),retry:z.number().default(3),note:z.string().nullable().optional()});
const schemas={...kafka,...cloudformation,...transfer,...connect,...ses,...s3,json:JSONStringified(order),order,text:z.string()};
const cases=[];
import { issues } from './parser-issues.mjs';
function add(name,schema,input,envelope,safe=true){
 const item={name,schema,input,envelope,safe};if(envelope==='kafka'&&input&&typeof input==='object')item.inputJSON=JSON.stringify(input);
 // Observe rejected validation promises without changing what Powertools receives.
 const source=schemas[schema];
 const observed={'~standard':{...source['~standard'],validate(value){
  const result=source['~standard'].validate(value);
  if(result instanceof Promise)result.catch(error=>{item.asyncRejection={error:error.name,message:error.message};});
  return result;
 }}};
 try{const result=parse(input,envelope==='kafka'?KafkaEnvelope:undefined,envelope?source:observed,safe);item.expected=safe&&!result.success?{success:false,issues:issues(result.error),original:result.originalEvent}:{success:true,data:safe?result.data:result};}
 catch(error){item.expected={success:false,thrown:true,error:error.name,message:error.message,issues:issues(error)};}
 cases.push(item);
}
const encode=value=>Buffer.from(typeof value==='string'?value:JSON.stringify(value)).toString('base64');
const record=(id='a')=>({topic:'orders',partition:0,offset:1,timestamp:1,timestampType:'CREATE_TIME',key:encode('key'),value:encode({id,amount:1}),headers:[{tag:[65,233,128512]}]});
const msk={bootstrapServers:'one:9092, two:9092,',records:{'orders-0':[record()]},eventSource:'aws:kafka',eventSourceArn:'arn:cluster'};
const self={...msk,eventSource:'SelfManagedKafka'};delete self.eventSourceArn;
const cfn={ServiceToken:'arn:service',ResponseURL:'https://example.test/response',StackId:'arn:stack',RequestId:'request',LogicalResourceId:'logical',ResourceType:'Custom::Type',ResourceProperties:{name:'value'},RequestType:'Create',PhysicalResourceId:'stripped'};
const customer={ProfileId:'profile',CustomerData:'data',IdempotencyToken:'token'};
const campaigns={InvocationMetadata:{CampaignContext:{CampaignId:'id',RunId:'run',ActionId:'action',CampaignName:'name'}},Items:{CustomerProfiles:[customer]}};
const timestamp='2026-09-15T00:00:00Z';
const verdict={status:'PASS'};
const receipt={timestamp,processingTimeMillis:1,recipients:['to@example.test'],spamVerdict:verdict,virusVerdict:verdict,spfVerdict:verdict,dmarcVerdict:verdict,dkimVerdict:verdict,dmarcPolicy:'none',action:{type:'Lambda',invocationType:'Event',functionArn:'arn:function'}};
const mail={timestamp,source:'from@example.test',messageId:'id',destination:['to@example.test'],headersTruncated:false,headers:[{name:'subject',value:'test'}],commonHeaders:{from:['from@example.test'],to:['to@example.test'],cc:[],bcc:[],sender:[],'reply-to':[],returnPath:'from@example.test',messageId:'id',date:'date',subject:'subject'}};
const sesRecord={eventSource:'aws:ses',eventVersion:'1',ses:{mail,receipt}};
const identity={principalId:'principal'};
const s3Record={eventVersion:'1',eventSource:'aws:s3',awsRegion:'ap-east-1',eventTime:timestamp,eventName:'ObjectCreated:Put',userIdentity:identity,requestParameters:{sourceIPAddress:'127.0.0.1'},responseElements:{'x-amz-request-id':'id','x-amz-id-2':'id2'},s3:{s3SchemaVersion:'1',configurationId:'config',object:{key:'a+b%2Fc',size:1,urlDecodedKey:'explicit',eTag:'etag',sequencer:'1',versionId:'version'},bucket:{name:'bucket',ownerIdentity:identity,arn:'arn:bucket'}},glacierEventData:{restoreEventData:{lifecycleRestorationExpiryTime:'expiry',lifecycleRestoreStorageClass:'class'}}};
const s3Event={Records:[s3Record]};
const sqsRecord={messageId:'1',receiptHandle:'r',body:JSON.stringify(s3Event),attributes:{ApproximateReceiveCount:'1',ApproximateFirstReceiveTimestamp:'1',SenderId:'s',SentTimestamp:'1'},messageAttributes:{},md5OfBody:'hash',eventSource:'aws:sqs',eventSourceARN:'arn:queue',awsRegion:'ap-east-1'};
const detail={version:'0',bucket:{name:'bucket'},object:{key:'key',size:1,etag:'etag','version-id':'version',sequencer:'1'},'request-id':'request',requester:'requester','source-ip-address':'127.0.0.1',reason:'PutObject','deletion-type':'delete','restore-expiry-time':'expiry','source-storage-class':'old','destination-storage-class':'new','destination-access-tier':'tier'};
const bridge={version:'0',id:'id',source:'aws.s3',account:'account',time:timestamp,region:'ap-east-1',resources:[], 'detail-type':'Object Created',detail};
const objectLambda={xAmzRequestId:'id',getObjectContext:{inputS3Url:'url',outputRoute:'route',outputToken:'token'},configuration:{accessPointArn:'arn:access',supportingAccessPointArn:'arn:support',payload:{stripped:true}},userRequest:{url:'url',headers:{}},userIdentity:{type:'AssumedRole',accountId:'account',accessKeyId:'key',userName:'name',principalId:'principal',arn:'arn:user',sessionContext:{sessionIssuer:{type:'Role',userName:'role',principalId:'principal',arn:'arn:role',accountId:'account'},attributes:{creationDate:'date',mfaAuthenticated:'true'}}},protocolVersion:'1'};
const samples={KafkaRecordSchema:record(),KafkaMskEventSchema:msk,KafkaSelfManagedEventSchema:self,CloudFormationCustomResourceCreateSchema:cfn,CloudFormationCustomResourceDeleteSchema:{...cfn,RequestType:'Delete'},CloudFormationCustomResourceUpdateSchema:{...cfn,RequestType:'Update',OldResourceProperties:{old:true}},TransferFamilySchema:{username:'user',password:'synthetic',protocol:'SFTP',serverId:'server',sourceIp:'127.0.0.1'},ConnectOutboundCampaignsCustomerProfileSchema:customer,ConnectOutboundCampaignsSchema:campaigns,SesRecordSchema:sesRecord,SesSchema:{Records:[sesRecord]},S3Schema:s3Event,S3SqsEventNotificationSchema:{Records:[sqsRecord]},S3EventNotificationEventBridgeSchema:bridge,S3ObjectLambdaEventSchema:objectLambda};
for(const [schema,input] of Object.entries(samples)){
 add(schema+'-valid',schema,input);add(schema+'-missing',schema,{});add(schema+'-null',schema,null);
 for(const key of Object.keys(input)){const removed={...input};delete removed[key];add(schema+'-omit-'+key,schema,removed);add(schema+'-invalid-'+key,schema,{...input,[key]:typeof input[key]==='number'?'bad':0});}
}
for(const source of [msk,self])for(const safe of [true,false]){
 const name=source.eventSource+'-'+safe;
 add(name,'json',source,'kafka',safe);
 add(name+'-ordered','json',{...source,records:{'z-0':[record('z')],'a-0':[record('a')]}},'kafka',safe);
 add(name+'-numeric-keys','json',{...source,records:{'10':[record('ten')],'2':[record('two')],'z':[record('z')]}},'kafka',safe);
 const invalid={...record(),value:encode({id:1,amount:-1})};
 add(name+'-invalid-records','json',{...source,records:{'z-0':[invalid,invalid],'a-0':[invalid]}},'kafka',safe);
 add(name+'-empty-topic','json',{...source,records:{'z-0':[],'a-0':[]}},'kafka',safe);
 add(name+'-empty-map','json',{...source,records:{}},'kafka',safe);
}
for(const input of [{},null,0,{records:{},eventSource:'unknown'}])for(const safe of [true,false])add('kafka-source-'+cases.length,'json',input,'kafka',safe);
add('kafka-no-implicit-json','order',msk,'kafka');
add('kafka-text','text',{...msk,records:{'orders-0':[{...record(),value:encode('plain')}] }},'kafka');
for(const value of ['aGk','!!!','8J-YgA=='])add('kafka-buffer-'+value,'KafkaRecordSchema',{...record(),value,key:value});
for(const value of [[-1],[1.5],[1114112],[0,255,128512],[55357,56832],[65,55357,56832,128512]])add('kafka-header-'+value.join('-'),'KafkaRecordSchema',{...record(),headers:[{value}]});
for(const bytes of [[255,255],[226,130],[226,130,65],[237,160,128],[240,144,128],[224,128,128],[244,144,128,128],[194,65],[128,128],[239,191,189]]){
 const value=Buffer.from(bytes).toString('base64');
 add('kafka-utf8-'+bytes.join('-'),'KafkaRecordSchema',{...record(),key:value,value});
}
add('kafka-null-tombstone','KafkaRecordSchema',{...record(),value:null});
for(const value of [0,-1,1.5,-1.5,9007199254740991,9007199254740992,-9007199254740992])add('ses-processing-'+value,'SesRecordSchema',{...sesRecord,ses:{mail,receipt:{...receipt,processingTimeMillis:value}}});
add('ses-invalid-verdict','SesRecordSchema',{...sesRecord,ses:{mail,receipt:{...receipt,spamVerdict:{status:'UNKNOWN'}}}});
add('connect-empty','ConnectOutboundCampaignsSchema',{...campaigns,Items:{CustomerProfiles:[]}});
add('transfer-ipv6','TransferFamilySchema',{...samples.TransferFamilySchema,sourceIp:'::1'});
add('s3-service-ip','S3Schema',{Records:[{...s3Record,requestParameters:{sourceIPAddress:'s3.amazonaws.com'}}]});
add('s3-ipv6','S3Schema',{Records:[{...s3Record,requestParameters:{sourceIPAddress:'::1'}}]});
for(const value of [0,null,'invalid'])add('s3-source-'+String(value),'S3Schema',{Records:[{...s3Record,requestParameters:{sourceIPAddress:value}}]});
add('s3-negative-size','S3Schema',{Records:[{...s3Record,s3:{...s3Record.s3,object:{key:'key',size:-1}}}]});
add('s3-eventbridge-negative-size','S3EventNotificationEventBridgeSchema',{...bridge,detail:{...detail,object:{key:'key',size:-1}}});
add('s3-sqs-invalid-json','S3SqsEventNotificationSchema',{Records:[{...sqsRecord,body:'{broken'}]});
for(const value of [true,false,'true','false','yes'])add('s3-mfa-'+String(value)+'-'+typeof value,'S3ObjectLambdaEventSchema',{...objectLambda,userIdentity:{...objectLambda.userIdentity,sessionContext:{...objectLambda.userIdentity.sessionContext,attributes:{creationDate:'date',mfaAuthenticated:value}}}});
await new Promise(setImmediate);
const rejected=cases.filter(item=>item.asyncRejection);
if(rejected.length!==3||rejected.some(item=>!['kafka-header--1','kafka-header-1.5','kafka-header-1114112'].includes(item.name)||item.asyncRejection.error!=='RangeError'||item.expected.message!=='Schema parsing supports only synchronous validation'))throw new Error('Unexpected asynchronous reference behavior');
writeFileSync('../../parser/testdata/services-v2.35.0.json',JSON.stringify({version:'2.35.0',zodVersion:'4.1.12',cases},null,2)+'\n');
console.log(`${cases.length} service Parser cases; ${Object.keys(samples).length} schema exports`);
