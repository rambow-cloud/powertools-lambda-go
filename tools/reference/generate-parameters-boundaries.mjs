import { writeFileSync } from 'node:fs';
import { BaseProvider } from '@aws-lambda-powertools/parameters/base';
const RealDate=Date;let now=10000;
globalThis.Date=class extends RealDate{constructor(...args){super(...(args.length?args:[now]));}static now(){return now;}};
class Provider extends BaseProvider{calls=0;constructor(){super({});}async _get(){return `value-${++this.calls}`;}}
const cases=[];
for(const input of [null,'','0','-1','0.1','1e-9','0x10','1e309','-Infinity','Infinity','invalid','NaN','9.223372036854776e9','5','1e-324']){
  if(input===null)delete process.env.POWERTOOLS_PARAMETERS_MAX_AGE;else process.env.POWERTOOLS_PARAMETERS_MAX_AGE=input;
  const p=new Provider();let expected;
  try{const value=await p.get('key');const entry=p.store.get('key,');const ttl=entry?.ttl;expected={value,calls:p.calls,cached:!!entry,ttl:entry?(Number.isNaN(ttl)?{number:'NaN'}:ttl):null};}
  catch(e){expected={error:e.name,message:e.message,calls:p.calls};}
  cases.push({mode:'environment',input,expected});
}
delete process.env.POWERTOOLS_PARAMETERS_MAX_AGE;
for(const [name,age,steps]of[
  ['boundary-force-clear',1,[{at:10000},{at:11000},{at:11001},{at:11001,force:true},{at:11001,clear:true}]],
  ['fractional',0.1,[{at:10000},{at:10001}]],
  ['zero',0,[{at:10000},{at:10000}]],
  ['warm-nonpositive',1,[{at:10000},{at:10000,age:0},{at:10000,age:-1},{at:10000}]],
]){
  const p=new Provider();const expected=[];
  for(const step of steps){now=step.at;if(step.clear)p.clearCache();const value=await p.get('key',{maxAge:step.age??age,forceFetch:!!step.force});expected.push({value,calls:p.calls});}
  cases.push({mode:'sequence',name,age,steps,expected});
}
globalThis.Date=RealDate;
writeFileSync(new URL('../../parameters/testdata/boundaries-v2.35.0.json',import.meta.url),JSON.stringify({version:'2.35.0',cases},null,2)+'\n');
console.log(`${cases.length} Parameters duration/cache observations`);
