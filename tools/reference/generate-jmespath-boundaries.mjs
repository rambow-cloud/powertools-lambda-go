import { writeFileSync } from 'node:fs';
import { search } from '@aws-lambda-powertools/jmespath';
const cases=[];
function normalize(value){if(typeof value==='number'&&!Number.isFinite(value))return {number:String(value)};if(typeof value==='number'&&Object.is(value,-0))return {number:'-0'};return value;}
function add(name,expression,data,specialData){
  let expected;
  try{expected={value:normalize(search(expression,data))};}catch(e){expected={error:e.constructor.name,message:e.message};}
  cases.push({name,expression,data:specialData?null:data,specialData,expected});
}
for(const [i,input]of['',' ','0x10','0b10','0o10','NaN','Infinity','-Infinity','1e309','-1e309','123bad','1e-324','1e2','-0','\ufeff1'].entries()){
  add(`number-${i}`,'to_number(@)',input);add(`string-number-${i}`,'to_string(to_number(@))',input);
}
for(const [name,input]of[
  ['html',['<&>']],['line-separators',{value:'a\u2028b\u2029c'}],['unicode','你好'],
  ['ascii-order',{b:1,a:2}],['numeric-order',{'10':'ten','2':'two'}],
  ['number-small',1e-7],['number-large',1e21],['negative-zero',-0],
])add(`string-${name}`,'to_string(@)',input,name==='negative-zero'?'-0':undefined);
add('transient-array',"to_string([to_number('NaN'),to_number('Infinity'),to_number('-0')])",null);
add('transient-array-object',"to_string([to_number('NaN'),to_number('Infinity'),to_number('-0')])",{});
for(const [i,data]of[null,false,0,[],{},[1],''].entries())for(const [kind,expression]of[['list','[@]'],['object','{value:@}']])add(`null-guard-${kind}-${i}`,expression,data);
for(const [name,expression,data]of[
 ['nested-list','missing.[@]',{}],['nested-object','missing.{value:@}',{}],
 ['pipe-list','missing | [@]',{}],['pipe-object','missing | {value:@}',{}],
 ['projection-list','items[*].[@]',{items:[null,false,{},[]]}],
 ['projection-object','items[*].{value:@}',{items:[null,false,{},[]]}],
 ['map','map(&[@], @)',[null,false,{}]],['nested-string','to_string([@])',null],
])add(`null-guard-${name}`,expression,data);
for(const [i,expression]of['',' ','items[','foo..bar',"'unterminated",'@ +','let $x = `1` in $x'].entries())add(`syntax-${i}`,expression,{});
for(const [i,expression,data]of[
  [0,'unknown(@)',1],[1,'length()',1],[2,'length(@,@)','x'],[3,'sum(@)',[1,'x']],
])add(`function-error-${i}`,expression,data);
writeFileSync(new URL('../../jmespath/testdata/boundaries-v2.35.0.json',import.meta.url),JSON.stringify({version:'2.35.0',cases},null,2)+'\n');
console.log(`${cases.length} JMESPath grammar/number/serialization observations`);
