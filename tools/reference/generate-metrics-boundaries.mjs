import { writeFileSync } from 'node:fs';
import { Metrics } from '@aws-lambda-powertools/metrics';
const now=1789992000000;
Date.now=()=>now;
for(const key of ['POWERTOOLS_METRICS_DISABLED','POWERTOOLS_DEV','POWERTOOLS_SERVICE_NAME','POWERTOOLS_METRICS_NAMESPACE'])delete process.env[key];
const cases=[];
for(const [name,key,input] of [
  ['string','payload','hello'],['boolean','payload',false],['null','payload',null],
  ['number','payload',1.5],['large-number','payload',9007199254740993],
  ['object','payload',{nested:{value:1}}],['array','payload',[1,'a',null]],
  ['prototype','__proto__','ignored'],['envelope','_aws',{custom:true}],['constructor','constructor','metadata'],
  ['mutation-object','payload',{nested:{value:'old'}}],['mutation-array','payload',['old']],
  ['nan','payload',NaN],['infinity','payload',Infinity],['negative-infinity','payload',-Infinity],
]){
  const special=typeof input==='number'&&!Number.isFinite(input)?String(input):undefined;
  const saved=special?null:structuredClone(input);
  const m=new Metrics({namespace:'Example',serviceName:'orders',logger:{warn(){},error(){},debug(){}}});
  m.addMetadata(key,input);m.addMetric('Count','Count',1);
  if(name==='mutation-object')input.nested.value='new';
  if(name==='mutation-array')input[0]='new';
  const expected=JSON.parse(JSON.stringify(m.serializeMetrics()));
  if(expected._aws?.CloudWatchMetrics)for(const d of expected._aws.CloudWatchMetrics)for(const keys of d.Dimensions)keys.sort();
  cases.push({name,key,input:saved,special,expected});
}
writeFileSync(new URL('../../metrics/testdata/boundaries-v2.35.0.json',import.meta.url),JSON.stringify({version:'2.35.0',now,cases},null,2)+'\n');
console.log(`${cases.length} actual Metrics metadata observations`);
