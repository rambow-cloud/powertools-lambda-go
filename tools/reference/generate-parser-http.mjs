// Execute HTTP schemas and envelopes from the pinned Parser distribution.
import { writeFileSync } from 'node:fs';
import { z } from 'zod';
import { parse } from '@aws-lambda-powertools/parser';
import { JSONStringified } from '@aws-lambda-powertools/parser/helpers';
import * as alb from '@aws-lambda-powertools/parser/schemas/alb';
import * as rest from '@aws-lambda-powertools/parser/schemas/api-gateway';
import * as http from '@aws-lambda-powertools/parser/schemas/api-gatewayv2';
import * as websocket from '@aws-lambda-powertools/parser/schemas/api-gateway-websocket';
import * as proxy from '@aws-lambda-powertools/parser/schemas/apigw-proxy';
import * as lambda from '@aws-lambda-powertools/parser/schemas/lambda';
import * as lattice from '@aws-lambda-powertools/parser/schemas/vpc-lattice';
import * as latticeV2 from '@aws-lambda-powertools/parser/schemas/vpc-latticev2';
import { ApiGatewayEnvelope, ApiGatewayV2Envelope, LambdaFunctionUrlEnvelope, VpcLatticeEnvelope, VpcLatticeV2Envelope } from '@aws-lambda-powertools/parser/envelopes';

const order=z.object({id:z.string(),amount:z.number().refine(value=>value>=0,{message:'amount must be non-negative'}),retry:z.number().default(3),note:z.string().nullable().optional()});
const schemas={...alb,...rest,...http,...websocket,...proxy,...lambda,...lattice,...latticeV2,order,json:JSONStringified(order),text:z.string(),unknown:z.unknown()};
const envelopes={apigateway:ApiGatewayEnvelope,apigatewayv2:ApiGatewayV2Envelope,lambdaurl:LambdaFunctionUrlEnvelope,lattice:VpcLatticeEnvelope,latticev2:VpcLatticeV2Envelope};
const cases=[];
import { issues } from './parser-issues.mjs';
function add(name,schema,input,envelope,safe=true){
 const item={name,schema,input,envelope,safe};
 try{const result=parse(input,envelopes[envelope],schemas[schema],safe);item.expected=safe&&!result.success?{success:false,issues:issues(result.error),original:result.originalEvent}:{success:true,data:safe?result.data:result};}
 catch(error){item.expected={success:false,thrown:true,error:error.name,issues:issues(error)};}
 cases.push(item);
}
const cert={clientCertPem:'pem',subjectDN:'subject',issuerDN:'issuer',serialNumber:'serial',validity:{notBefore:'before',notAfter:'after'}};
const body=JSON.stringify({id:'http',amount:1});
const identity={accessKey:null,accountId:null,apiKey:null,apiKeyId:null,caller:null,cognitoAuthenticationProvider:null,cognitoAuthenticationType:null,cognitoIdentityId:null,cognitoIdentityPoolId:null,principalOrgId:null,sourceIp:'127.0.0.1',user:null,userAgent:'agent',userArn:null,clientCert:cert};
const context={accountId:'account',apiId:'api',deploymentId:null,authorizer:{integrationLatency:1,principalId:'p',ignored:true},stage:'test',protocol:'HTTP/1.1',identity,requestId:'request',requestTime:'time',requestTimeEpoch:1,resourceId:null,resourcePath:'/',domainName:'example.test',domainPrefix:'example',extendedRequestId:null,httpMethod:'POST',path:'/',connectedAt:null,connectionId:null,eventType:null,messageDirection:null,messageId:null,routeKey:null,operationName:null};
const restEvent={resource:'/',path:'/',httpMethod:'POST',headers:{accept:'json'},multiValueHeaders:{accept:['json']},queryStringParameters:null,multiValueQueryStringParameters:null,pathParameters:null,stageVariables:null,requestContext:context,body,isBase64Encoded:false};
const restAuth={type:'REQUEST',methodArn:'arn:method',resource:'/',path:'/',httpMethod:'GET',headers:{},multiValueHeaders:{},queryStringParameters:{},multiValueQueryStringParameters:{},pathParameters:{},stageVariables:{},requestContext:context,domainName:'example.test',deploymentId:'deploy',apiId:'api'};
const authorizer={jwt:{claims:{sub:'user'},scopes:null},iam:{accessKey:'key',accountId:'account',callerId:'caller',principalOrgId:null,userArn:'arn:user',userId:'user',cognitoIdentity:{amr:['authenticated'],identityId:'id',identityPoolId:'pool'}},lambda:{role:'admin'}};
const contextV2={accountId:'account',apiId:'api',authorizer,authentication:{clientCert:cert},domainName:'example.test',domainPrefix:'example',http:{method:'POST',path:'/',protocol:'HTTP/1.1',sourceIp:'2001:db8::1',userAgent:'agent'},requestId:'id',routeKey:'POST /',stage:'test',time:'time',timeEpoch:1};
const httpEvent={version:'2.0',routeKey:'POST /',rawPath:'/',rawQueryString:'',cookies:['a=b'],headers:{accept:'json'},queryStringParameters:{},requestContext:contextV2,body,pathParameters:null,isBase64Encoded:false,stageVariables:null};
const httpAuth={version:'2.0',type:'REQUEST',routeArn:'arn:route',identitySource:null,routeKey:'POST /',rawPath:'/',rawQueryString:'',cookies:['a=b'],headers:{},queryStringParameters:{},requestContext:contextV2,pathParameters:null,stageVariables:null};
const ws={type:'REQUEST',methodArn:'arn:method',headers:null,multiValueHeaders:{},queryStringParameters:null,multiValueQueryStringParameters:null,stageVariables:null,requestContext:{routeKey:'$connect',eventType:'CONNECT',extendedRequestId:'extended',requestTime:'time',messageDirection:'IN',stage:'test',connectedAt:1,requestTimeEpoch:1,identity:{sourceIp:'not required to be IP',userAgent:'agent'},requestId:'id',domainName:'example.test',connectionId:'connection',apiId:'api'},isBase64Encoded:false,body:null};
const albEvent={httpMethod:'CUSTOM',path:'/',body,isBase64Encoded:false,headers:{accept:'json'},multiValueHeaders:{accept:['json']},queryStringParameters:{},multiValueQueryStringParameters:{},requestContext:{elb:{targetGroupArn:'arn:target'}}};
const vpc={method:'POST',raw_path:'/',body,is_base64_encoded:false,headers:{},query_string_parameters:{}};
const vpcV2={version:'2.0',path:'/',method:'POST',headers:{},queryStringParameters:{},body,isBase64Encoded:false,requestContext:{serviceNetworkArn:'arn:network',serviceArn:'arn:service',targetGroupArn:'arn:target',region:'ap-east-1',timeEpoch:'1',identity:{sourceVpcArn:'arn:vpc',type:'AWS_IAM',principal:'p',principalOrgId:'o',sessionName:'s',X509SubjectCn:'subject',X509IssuerOu:'issuer',x509SanDns:'dns',x509SanUri:'uri',X509SanNameCn:'cn'}}};
const samples={APIGatewayCert:cert,APIGatewayRecord:{a:'b'},APIGatewayStringArray:['a'],APIGatewayHttpMethod:'GET',APIGatewayEventRequestContextSchema:context,APIGatewayProxyEventSchema:restEvent,APIGatewayRequestAuthorizerEventSchema:restAuth,APIGatewayTokenAuthorizerEventSchema:{type:'TOKEN',authorizationToken:'token',methodArn:'arn:method'},APIGatewayRequestAuthorizerV2Schema:authorizer,APIGatewayRequestContextV2Schema:contextV2,APIGatewayProxyEventV2Schema:httpEvent,APIGatewayRequestAuthorizerEventV2Schema:httpAuth,APIGatewayProxyWebsocketEventSchema:ws,AlbSchema:albEvent,AlbMultiValueHeadersSchema:albEvent,LambdaFunctionUrlSchema:httpEvent,VpcLatticeSchema:vpc,VpcLatticeV2Schema:vpcV2};
for(const [schema,input] of Object.entries(samples)){
 add(schema+'-valid',schema,input);add(schema+'-missing',schema,{});add(schema+'-null',schema,null);
 if(input&&typeof input==='object'&&!Array.isArray(input))for(const key of Object.keys(input)){
  const removed={...input};delete removed[key];add(schema+'-omit-'+key,schema,removed);
  add(schema+'-invalid-'+key,schema,{...input,[key]:typeof input[key]==='number'?'bad':0});
 }
}
for(const [envelope,input] of [['apigateway',restEvent],['apigatewayv2',httpEvent],['lambdaurl',httpEvent],['lattice',vpc],['latticev2',vpcV2]]){
 for(const safe of [true,false]){
  add(envelope+'-json-'+safe,'json',input,envelope,safe);
  add(envelope+'-invalid-body-'+safe,'json',{...input,body:'{"id":1,"amount":-1}'},envelope,safe);
  add(envelope+'-invalid-both-'+safe,'json',{...input,body:'broken',requestContext:null,method:'OTHER',httpMethod:'OTHER'},envelope,safe);
 }
 add(envelope+'-text','text',input,envelope);
 add(envelope+'-no-implicit-json','order',input,envelope);
 add(envelope+'-no-implicit-base64','text',{...input,body:Buffer.from(body).toString('base64'),isBase64Encoded:true,is_base64_encoded:true},envelope);
 add(envelope+'-object-body','order',{...input,body:{id:'http',amount:1}},envelope);
 const missing={...input};delete missing.body;
 add(envelope+'-missing-body','json',missing,envelope);add(envelope+'-unknown-missing-body','unknown',missing,envelope);
 add(envelope+'-unknown-null-body','unknown',{...input,body:null},envelope);
}
for(const ip of ['127.0.0.1','::1','2001:db8::1','::ffff:192.0.2.1','test-invoke-source-ip','999.1.1.1','1.2.3','01.2.3.4','[::1]','fe80::1%eth0',null]){
 add('rest-ip-'+ip,'APIGatewayEventRequestContextSchema',{...context,identity:{sourceIp:ip}});
 add('http-ip-'+ip,'APIGatewayRequestContextV2Schema',{...contextV2,http:{...contextV2.http,sourceIp:ip}});
}
for(const [messageId,eventType] of [['id','MESSAGE'],['id','CONNECT'],['','CONNECT'],[null,'DISCONNECT']])add('rest-message-'+String(messageId)+'-'+eventType,'APIGatewayEventRequestContextSchema',{...context,messageId,eventType});
add('rest-jwt-authorizer','APIGatewayEventRequestContextSchema',{...context,authorizer:{claims:{sub:'user'},scopes:['read']}});
add('rest-null-authorizer','APIGatewayEventRequestContextSchema',{...context,authorizer:null});
add('rest-minimal-identity','APIGatewayEventRequestContextSchema',{...context,identity:{}});
add('vpc-time-is-string','VpcLatticeV2Schema',{...vpcV2,requestContext:{...vpcV2.requestContext,timeEpoch:1}});
add('websocket-direction','APIGatewayProxyWebsocketEventSchema',{...ws,requestContext:{...ws.requestContext,messageDirection:'OTHER'}});
writeFileSync('../../parser/testdata/http-v2.35.0.json',JSON.stringify({version:'2.35.0',zodVersion:'4.1.12',cases},null,2)+'\n');
console.log(`${cases.length} HTTP Parser cases; ${Object.keys(samples).length} schema exports`);
