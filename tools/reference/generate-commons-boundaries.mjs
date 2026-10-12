import { writeFileSync } from 'node:fs';
import { fromBase64 } from '@aws-lambda-powertools/commons/utils/base64';
import { unmarshallDynamoDB } from '@aws-lambda-powertools/commons/utils/unmarshallDynamoDB';
import { isTruthy, isIntegerNumber, getType } from '@aws-lambda-powertools/commons/typeutils';

const normalize = value => {
  if (typeof value === 'bigint') return { bigint: String(value) };
  if (typeof value === 'number' && !Number.isFinite(value)) return { number: String(value) };
  if (value instanceof Set || value instanceof Uint8Array || Array.isArray(value)) return [...value].map(normalize);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([k,v]) => [k,normalize(v)]));
  return value;
};
const outcome = fn => { try { return { value: normalize(fn()) }; } catch (e) { return { error: e.name, message: e.message }; } };
const numbers = ['',' ','-0','.5','1.','+2','1e-324','5e-324','1e309','-1e309','NaN','nan','Infinity','-Infinity','0x20000000000001','0b10','0o10','0b2','0x_1','1_000','9007199254740993','-9007199254740993','9007199254740993.0','1e20'];
const cases = numbers.map(input => ({ mode:'number', input, ...outcome(() => unmarshallDynamoDB({n:{N:input}}).n) }));
for (const input of [
  {NS:['invalid','NaN','invalid','0','-0','Infinity','Infinity']},
  {NS:['9007199254740993','9007199254740993','1']},
  {SS:['a','a','b']}, {BOOL:0}, {BOOL:1}, {BOOL:''}, {BOOL:'0'}, {BOOL:null},
  {L:[{S:'a'},{M:{n:{N:'2'}}}]}, {NULL:false}, {UNKNOWN:'x'}, {M:[]}, {NS:'text'},
]) cases.push({mode:'attribute',input,...outcome(() => unmarshallDynamoDB({value:input}).value)});
for (const input of ['A===','====','ab=c','aA==','AA==','AAAA','YWJj','aGVs bG8','aGVsbG8','ab-_']) {
  for (const encoding of ['base64','hex','utf16le']) cases.push({mode:'base64',input,encoding,...outcome(() => fromBase64(input,encoding))});
}
for (const input of ['NaN','Infinity','-Infinity','-0']) {
  const value=Number(input);
  cases.push({mode:'type',input,value:{truthy:isTruthy(value),integer:isIntegerNumber(value),type:getType(value)}});
}
writeFileSync(new URL('../../commons/testdata/boundaries-v2.35.0.json',import.meta.url),JSON.stringify({version:'2.35.0',cases},null,2)+'\n');
writeFileSync(new URL('../../commons/dynamodb/testdata/boundaries-v2.35.0.json',import.meta.url),JSON.stringify({version:'2.35.0',cases:cases.filter(x=>x.mode==='number')},null,2)+'\n');
console.log(`${cases.length} Commons observations; ${numbers.length} also owned by the DynamoDB module`);
