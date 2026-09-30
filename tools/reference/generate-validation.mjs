// Run the pinned Powertools utility; preserve complete ordered AJV diagnostics.
import { mkdirSync, writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';
import Ajv from 'ajv';

const schemas={
 string:{type:'string'}, integer:{type:'integer'}, nullable:{type:'string',nullable:true},
 nullableEnum:{type:'string',nullable:true,enum:['ok']},
 limits:{type:'number',minimum:1,maximum:5}, exclusive:{type:'number',exclusiveMinimum:1,exclusiveMaximum:5},
 multiple:{type:'number',multipleOf:2}, text:{type:'string',minLength:2,maxLength:4},
 pattern:{type:'string',pattern:'^a+$'}, constant:{const:'ok'}, enum:{enum:[1,'ok',null]},
 object:{type:'object',properties:{id:{type:'string'}},required:['id'],additionalProperties:false},
 array:{type:'array',items:{type:'integer'},minItems:1,maxItems:2}, unique:{type:'array',uniqueItems:true},
 objectSize:{type:'object',minProperties:1,maxProperties:2},
 anyOf:{anyOf:[{type:'string'},{type:'number'}]}, oneOf:{oneOf:[{type:'number'},{type:'integer'}]},
 allOf:{allOf:[{type:'number'},{minimum:1}]}, not:{not:{type:'string'}},
 localRef:{$defs:{id:{type:'string'}},$ref:'#/$defs/id',minLength:2},
 defaults:{type:'object',properties:{id:{type:'string',default:'a'}}},
 combined:{type:'string',const:'ok',minLength:3,pattern:'^x'},
 true:true, false:false,
};
const inputs=[null,false,0,1,2,6,1.5,'','a','ok','aaaaa','😀',[],[1],[1,2,3],[1,1],{}, {id:'a'},{id:1},{extra:true}];
const cases=[];
function add(name,schema,payload,options={}){
 const item={name,schema,payload,options};
 const formats={startsA:value=>value.startsWith('a'),even:{type:'number',validate:value=>value%2===0}};
 try{
  const result=validate({schema,payload:structuredClone(payload),...options,formats:options.formats?formats:undefined,ajv:new Ajv({allErrors:true,logger:false})});
  item.expected={success:true,value:result};
 }catch(error){item.expected={success:false,error:error.name,message:error.message};if(Array.isArray(error.cause))item.expected.issues=error.cause;}
 cases.push(item);
}
for(const [name,schema] of Object.entries(schemas))for(const payload of inputs)add(name+'-'+cases.length,schema,payload);
for(const schema of [{format:'email'},{typo:true},{type:'bogus'},{nullable:true},{type:'null',nullable:false},{$ref:'https://example.invalid/missing'},{$schema:'https://json-schema.org/draft/2020-12/schema'}, {type:'string',minLength:-1}])add('compile-'+cases.length,schema,'ok');
for(const payload of ['abc','bad',2])add('custom-string-'+cases.length,{format:'startsA'},payload,{formats:true});
for(const payload of [2,3,'bad'])add('custom-number-'+cases.length,{format:'even'},payload,{formats:true});
for(const payload of [{body:{id:'a'}},{body:{id:1}}])add('envelope-'+cases.length,schemas.object,payload,{envelope:' body '});
for(const payload of ['ok',0])add('external-'+cases.length,{$ref:'https://example.test/id'},payload,{externalRefs:[{$id:'https://example.test/id',type:'string'}]});
for(const schema of [{type:['string','null'],nullable:false},{type:[],nullable:true},{type:'string',allOf:[]},{type:'string',allOf:false}])add('compile-'+cases.length,schema,'ok');
mkdirSync('../../validation/testdata',{recursive:true});
writeFileSync('../../validation/testdata/validation-v2.35.0.json',JSON.stringify({version:'2.35.0',ajvVersion:'8.20.0',cases},null,2)+'\n');
console.log(`${cases.length} validation cases`);
