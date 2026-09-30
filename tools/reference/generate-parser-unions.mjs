// Execute pinned union branch selection, nested diagnostics and refinement behavior.
import { writeFileSync } from 'node:fs';
import { z } from 'zod';
import { parse } from '@aws-lambda-powertools/parser';
import { JSONStringified } from '@aws-lambda-powertools/parser/helpers';
import { issues } from './parser-issues.mjs';

const positive=z.number().refine(value=>value>0,{message:'positive required'});
const long=z.string().refine(value=>value.length>3,{message:'long text required'});
const uppercase=z.string().transform(value=>value.toUpperCase());
const schemas={
 unionTypes:z.union([z.string(),z.number()]),
 unionSole:z.union([long,z.number()]),
 unionTwo:z.union([long,z.string().refine(value=>value.startsWith('a'),{message:'prefix required'})]),
 unionSingle:z.union([positive]),
 unionObjects:z.union([z.object({kind:z.literal('a'),value:positive}),z.object({kind:z.literal('b'),value:z.string()})]),
 unionArrays:z.union([z.array(z.number()).min(2),z.string()]),
 unionNested:z.object({payload:z.union([z.object({value:z.union([z.string(),z.number()])}),z.array(z.boolean())])}),
 unionStrict:z.union([z.object({id:z.string()}).strict(),z.number()]),
 unionJSON:z.union([JSONStringified(z.object({id:z.string()})),z.number()]),
 unionRefinements:positive.refine(value=>value%2===0,{message:'even required'}),
 unionTransform:z.union([uppercase,z.number()]),
 unionArrayRefine:z.union([z.array(z.number()).min(2),z.boolean()]).refine(value=>!Array.isArray(value)||value.length>0,{message:'nonempty required'}),
 unionObjectRefine:z.object({value:positive}).refine(value=>value.value!==-3,{message:'not minus three'}),
 unionNullable:z.union([long,z.number()]).nullable(),
 unionDictionary:z.record(z.string(),z.union([z.string(),z.number()])),
 unionInArray:z.array(z.union([z.string(),z.number()])),
};
const inputs=[null,false,true,0,-3,-2,2,'a','bad','valid','{','{"id":2}','{"id":"a"}',[],[false],[1],[1,2],['bad'],{}, {id:'a',extra:true},{kind:'a',value:-3},{kind:'b',value:1},{value:-3},{payload:{value:false}},{payload:[0,false]},{a:false,b:[]}];
const cases=[];
inputs.push('', '😀', {length:0}, {length:2}, {length:'0'}, {length:null}, {length:[]}, {length:[2]}, {length:'bad'}, {length:[[0]]});
for(const [schema,validator] of Object.entries(schemas))for(const input of inputs)for(const safe of [true,false]){
 const item={name:schema+'-'+cases.length,schema,input,safe};
 try{const result=parse(input,undefined,validator,safe);item.expected=safe&&!result.success?{success:false,original:input,issues:issues(result.error)}:{success:true,data:safe?result.data:result};}
 catch(error){item.expected={success:false,thrown:true,error:error.name,issues:issues(error)};}
 cases.push(item);
}
writeFileSync('../../parser/testdata/unions-v2.35.0.json',JSON.stringify({version:'2.35.0',zodVersion:'4.1.12',cases},null,2)+'\n');
console.log(`${cases.length} union and refinement cases`);
