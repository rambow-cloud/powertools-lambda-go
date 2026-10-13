import { writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';

const cases=[],registration=[];
function add(target,name,schema,payload,options={}) {
 const item={name,schema:structuredClone(schema),payload,options:structuredClone(options)};
 const warnings=[];
 const warn=console.warn;
 console.warn=(...args)=>warnings.push(args.map(String).join(' '));
 // Execute the public default factory without injecting or changing AJV options.
 try {item.expected={success:true,value:validate({schema:structuredClone(schema),payload:structuredClone(payload),...structuredClone(options)})};}
 catch(error){item.expected={success:false,error:error.name,message:error.message,...(Array.isArray(error.cause)?{issues:error.cause}:{}),...(error.cause instanceof Error?{cause:error.cause.message}:{})};}
 finally {console.warn=warn;}
 item.warnings=warnings;
 target.push(item);
}
for(const [name,body]of Object.entries({unknown:{typo:true},format:{format:'email'},pattern:{pattern:'['},length:{minLength:-1},nullable:{nullable:true}})) {
 for(const reachable of [false,true])for(const [i,payload]of ['value',1].entries()) {
  const schema={definitions:{target:body},...(reachable?{$ref:'#/definitions/target'}:{})};
  add(cases,`default-${name}-${reachable}-${i}`,schema,payload);
 }
}
for(const [i,schema]of [
 {definitions:{a:{$id:'https://example.test/id'},b:{$id:'https://example.test/id'}}},
 {$id:'https://example.test/root',definitions:{a:{$id:'#same'},b:{$id:'#same'}}},
 {$ref:'https://example.test/missing'},
].entries())add(cases,`identity-${i}`,schema,'value');
const conditional={type:'object',properties:{kind:{type:'string'},value:{type:'number'}},required:['kind','value'],if:{properties:{kind:{const:'positive'}},required:['kind']},then:{properties:{value:{minimum:1}}},else:{properties:{value:{maximum:0}}}};
for(const [i,payload]of [{kind:'positive',value:0},{kind:'positive',value:1},{kind:'other',value:1},{kind:'other',value:0}].entries())add(cases,`conditional-${i}`,conditional,payload);
for(const [i,payload]of [0.3,0.30000000000000004,1.0000000000000002,0.7].entries())add(cases,`numeric-${i}`,{type:'number',multipleOf:0.1,maximum:1},payload);
for(const [i,pattern]of ['^.$','^\\p{Letter}+$','(?i)a'].entries())for(const [j,payload]of ['😀','A'].entries())add(cases,`regex-${i}-${j}`,{type:'string',pattern},payload);
for(const [name,schema,payload]of [
 ['true',true,null],['false',false,0],['nullable',{type:'string',nullable:true},null],
 ['no-coercion',{type:'number'},'1'],['no-default',{type:'object',properties:{value:{type:'number',default:1}},required:['value']},{}],
 ['no-removal',{type:'object',additionalProperties:false},{extra:true}],
])add(cases,`factory-${name}`,schema,payload);
add(registration,'duplicate-external',true,'value',{externalRefs:[{$id:'https://example.test/duplicate',type:'string'},{$id:'https://example.test/duplicate',type:'number'}]});
add(registration,'invalid-unused-external',true,'value',{externalRefs:[{$id:'https://example.test/invalid',type:'bogus'}]});
for(const [file,observations]of [['boundaries',cases],['registration-boundaries',registration]])writeFileSync(new URL(`../../validation/testdata/${file}-v2.35.0.json`,import.meta.url),JSON.stringify({version:'2.35.0',ajvVersion:'8.20.0',cases:observations},null,2)+'\n');
console.log(`${cases.length} public default validation observations; ${registration.length} original registration errors`);
