// Execute all pinned AppSync and Cognito schema exports and nested mutations.
import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { z } from 'zod';
import { parse } from '@aws-lambda-powertools/parser';
import { issues } from './parser-issues.mjs';
import * as shared from '@aws-lambda-powertools/parser/schemas/appsync-shared';
import * as appsync from '@aws-lambda-powertools/parser/schemas/appsync';
import * as events from '@aws-lambda-powertools/parser/schemas/appsync-events';
import * as cognito from '@aws-lambda-powertools/parser/schemas/cognito';

const schemas={...shared,...appsync,...events,...cognito,null:z.null()};
const cases=[];
function add(name,schema,input){
 const result=parse(input,undefined,schemas[schema],true);
 const expected=result.success?{success:true,data:result.data}:{success:false,original:input,issues:issues(result.error)};
 cases.push({name,schema,input,safe:true,expected});
}
const iam={accountId:'account',cognitoIdentityPoolId:null,cognitoIdentityId:null,sourceIp:['unrestricted'],username:'user',userArn:'arn:user',cognitoIdentityAuthType:null,cognitoIdentityAuthProvider:null};
const user={sub:'sub',issuer:'issuer',username:'user',claims:{role:'reader'},sourceIp:['127.0.0.1'],defaultAuthStrategy:null,groups:null};
const oidc={claims:{role:'reader'},issuer:'issuer',sub:'sub'};
const lambda={resolverContext:{role:'reader'}};
const lambdaAuth={handlerContext:{role:'reader'}};
const resolver={arguments:{id:'order'},identity:iam,source:null,request:{domainName:null,headers:{host:'example.test'}},info:{selectionSetList:['id'],selectionSetGraphQL:'{id}',parentTypeName:'Query',fieldName:'order',variables:{}},prev:{result:{}},stash:{retained:true}};
const info={channel:{path:'/orders/new',segments:['orders','new']},channelNamespace:{name:'orders'},operation:'PUBLISH'};
const request={headers:{host:'example.test'},domainName:null};
const baseEvent={identity:null,result:null,request,info,error:null,prev:null,stash:{stripped:true},outErrors:[{retained:true}],events:null};
const publish={...baseEvent,events:[{payload:{id:'order'},id:'1'}]};
const subscribe={...baseEvent,info:{...info,operation:'SUBSCRIBE'}};
const base={version:'1',triggerSource:'unspecified',region:'ap-east-1',userPoolId:'pool',userName:'user',callerContext:{awsSdkVersion:'version',clientId:'client'},request:{stripped:true},response:{stripped:true}};
const attributes={email:'synthetic@example.test'};
const metadata={application:'test'};
const group={groupsToOverride:['readers'],iamRolesToOverride:[],preferredRole:null};
const tokenRequest={userAttributes:attributes,groupConfiguration:group,clientMetadata:metadata};
const challenge={challengeName:'CUSTOM_CHALLENGE',challengeResult:false,challengeMetadata:'metadata'};
const challengeRequest={userAttributes:attributes,session:[challenge],clientMetadata:metadata,userNotFound:false};
const samples={
 AppSyncIamIdentity:iam,AppSyncCognitoIdentity:user,AppSyncOidcIdentity:oidc,AppSyncLambdaIdentity:lambda,
 AppSyncResolverSchema:resolver,AppSyncBatchResolverSchema:[resolver],AppSyncLambdaAuthIdentity:lambdaAuth,
 AppSyncEventsRequestSchema:request,AppSyncEventsInfoSchema:info,AppSyncEventsBaseSchema:baseEvent,AppSyncEventsPublishSchema:publish,AppSyncEventsSubscribeSchema:subscribe,
 CognitoTriggerBaseSchema:base,
 PreSignupTriggerSchema:{...base,triggerSource:'PreSignUp_SignUp',request:{userAttributes:attributes,validationData:null,clientMetadata:metadata,userNotFound:false},response:{autoConfirmUser:false,autoVerifyEmail:false,autoVerifyPhone:false}},
 PostConfirmationTriggerSchema:{...base,triggerSource:'PostConfirmation_ConfirmSignUp',request:{userAttributes:attributes,clientMetadata:metadata}},
 PreAuthenticationTriggerSchema:{...base,triggerSource:'PreAuthentication_Authentication',request:{userAttributes:attributes,validationData:null,userNotFound:false}},
 PostAuthenticationTriggerSchema:{...base,triggerSource:'PostAuthentication_Authentication',request:{userAttributes:attributes,newDeviceUsed:false,clientMetadata:metadata}},
 PreTokenGenerationTriggerGroupConfigurationSchema:group,PreTokenGenerationTriggerRequestSchema:tokenRequest,
 PreTokenGenerationTriggerSchemaV1:{...base,request:tokenRequest},PreTokenGenerationTriggerSchemaV2AndV3:{...base,version:'3',request:{...tokenRequest,scopes:['read']}},
 MigrateUserTriggerSchema:{...base,request:{password:'synthetic',validationData:{key:'value'},clientMetadata:metadata},response:{userAttributes:null,finalUserStatus:null,messageAction:null,desiredDeliveryMediums:null,forceAliasCreation:null,enableSMSMFA:null}},
 CustomMessageTriggerSchema:{...base,request:{userAttributes:attributes,codeParameter:'code',linkParameter:null,usernameParameter:null,clientMetadata:metadata},response:{smsMessage:null,emailMessage:null,emailSubject:null}},
 CustomEmailSenderTriggerSchema:{...base,triggerSource:'CustomEmailSender_SignUp',request:{type:'customEmailSenderRequestV1',code:'code',clientMetadata:metadata,userAttributes:attributes}},
 CustomSMSSenderTriggerSchema:{...base,triggerSource:'CustomSMSSender_SignUp',request:{type:'customSMSSenderRequestV1',code:'code',clientMetadata:metadata,userAttributes:attributes}},
 ChallengeResultSchema:challenge,
 DefineAuthChallengeTriggerSchema:{...base,triggerSource:'DefineAuthChallenge_Authentication',request:challengeRequest,response:{challengeName:null,issueTokens:null,failAuthentication:null}},
 CreateAuthChallengeTriggerSchema:{...base,triggerSource:'CreateAuthChallenge_Authentication',request:{userAttributes:attributes,challengeName:'CUSTOM_CHALLENGE',session:[challenge],clientMetadata:metadata,userNotFound:false},response:{publicChallengeParameters:null,privateChallengeParameters:null,challengeMetadata:null}},
 VerifyAuthChallengeTriggerSchema:{...base,triggerSource:'VerifyAuthChallengeResponse_Authentication',request:{userAttributes:attributes,privateChallengeParameters:{answer:'synthetic'},challengeAnswer:'synthetic',clientMetadata:metadata,userNotFound:false},response:{answerCorrect:false}},
};
const exports=Object.keys(schemas).filter(name=>name!=='null').sort();
if(JSON.stringify(exports)!==JSON.stringify(Object.keys(samples).sort()))throw new Error('Missing schema sample');
function paths(value,path=[]){
 if(!value||typeof value!=='object')return [];
 return Object.entries(value).flatMap(([key,item])=>{const next=[...path,Array.isArray(value)?Number(key):key];return [next,...paths(item,next)];});
}
for(const [schema,input] of Object.entries(samples)){
 add(schema+'-valid',schema,input);add(schema+'-empty',schema,{});add(schema+'-null',schema,null);
 for(const path of paths(input))for(const mode of ['omit','null','wrong']){
  const changed=structuredClone(input);const parent=path.slice(0,-1).reduce((value,key)=>value[key],changed);const key=path.at(-1);
  if(mode==='omit'){if(Array.isArray(parent))parent.splice(key,1);else delete parent[key];}
  else parent[key]=mode==='null'?null:0;
  add(schema+'-'+path.join('.')+'-'+mode,schema,changed);
 }
}
for(const identity of [user,iam,oidc,lambda,lambdaAuth,null,{},'invalid']){
 const name=cases.length;
 add('resolver-identity-'+name,'AppSyncResolverSchema',{...resolver,identity});
 add('events-identity-'+name,'AppSyncEventsPublishSchema',{...publish,identity});
}
for(const schema of ['AppSyncCognitoIdentity'])for(const sourceIp of [['::1'],['bad'],[],['127.0.0.1','::1']])add('identity-ip-'+cases.length,schema,{...user,sourceIp});
add('resolver-empty-lambda-identity','AppSyncResolverSchema',{...resolver,identity:{unrecognized:true}});
add('resolver-oidc-precedes-lambda','AppSyncResolverSchema',{...resolver,identity:{...oidc,...lambda}});
add('empty-resolver-batch','AppSyncBatchResolverSchema',[]);
add('empty-publish-events','AppSyncEventsPublishSchema',{...publish,events:[]});
add('publish-operation-mismatch','AppSyncEventsPublishSchema',{...publish,info:{...info,operation:'SUBSCRIBE'}});
add('subscribe-operation-mismatch','AppSyncEventsSubscribeSchema',{...subscribe,info});
for(const schema of ['DefineAuthChallengeTriggerSchema','CreateAuthChallengeTriggerSchema'])add(schema+'-empty-session',schema,{...samples[schema],request:{...samples[schema].request,session:[]}});
for(const challengeName of ['CUSTOM_CHALLENGE','SRP_A','PASSWORD_VERIFIER','SMS_MFA','EMAIL_OTP','SOFTWARE_TOKEN_MFA','DEVICE_SRP_AUTH','DEVICE_PASSWORD_VERIFIER','ADMIN_NO_SRP_AUTH','INVALID'])add('challenge-'+challengeName,'ChallengeResultSchema',{...challenge,challengeName});
for(const field of ['autoConfirmUser','autoVerifyEmail','autoVerifyPhone'])add('signup-true-'+field,'PreSignupTriggerSchema',{...samples.PreSignupTriggerSchema,response:{...samples.PreSignupTriggerSchema.response,[field]:true}});
for(const [schema,triggerSource] of [['PreSignupTriggerSchema','PreSignUp_AdminCreateUser'],['PostConfirmationTriggerSchema','PostConfirmation_ConfirmForgotPassword'],['CustomEmailSenderTriggerSchema','CustomEmailSender_Authentication'],['CustomSMSSenderTriggerSchema','CustomSMSSender_Authentication']])add(schema+'-other-trigger',schema,{...samples[schema],triggerSource});
for(const input of [null,{},0,'null',false,[]])add('null-'+cases.length,'null',input);
writeFileSync('../../parser/testdata/identity-v2.35.0.json',JSON.stringify({version:'2.35.0',zodVersion:'4.1.12',exports,cases},null,2)+'\n');
// Map every public runtime schema export, including wildcard paths and re-exports.
// This inventories named Go definitions; behavior and inferred types have separate gates.
const definitions=new Map();
for(const file of readdirSync('../../parser/schemas').filter(file=>file.endsWith('.go')&&!file.endsWith('_test.go'))){
 const source=readFileSync('../../parser/schemas/'+file,'utf8');
 for(const match of source.matchAll(/^var ([A-Z]\w*)\s*=/gm))definitions.set(match[1],'parser/schemas/'+file);
}
const inventory=JSON.parse(readFileSync('../../docs/PARSER_EXPORTS.json','utf8'));
const mapped=new Map();
for(const module of inventory.modules.filter(module=>module.subpath.startsWith('schemas/'))){
 for(const name of module.runtimeExports){
  if(!definitions.has(name))throw new Error('Missing Go schema export: '+name);
  if(!mapped.has(name))mapped.set(name,{referenceName:name,goSymbol:'schemas.'+name,goSource:definitions.get(name),referenceSubpaths:[]});
  mapped.get(name).referenceSubpaths.push(module.subpath);
 }
}
if(mapped.size!==90)throw new Error('Unexpected public schema export count: '+mapped.size);
writeFileSync('../../docs/PARSER_SCHEMA_MAP.json',JSON.stringify({reference:'2.35.0',scope:'Runtime schema names only; inferred types and complete behavior are separate acceptance gates.',exports:[...mapped.values()].sort((a,b)=>a.referenceName.localeCompare(b.referenceName))},null,2)+'\n');
console.log(`${cases.length} identity Parser cases; ${exports.length} schema exports`);
console.log(`${mapped.size} public runtime schema names mapped to Go definitions`);
