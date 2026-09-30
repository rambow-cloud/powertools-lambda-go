// Generate Unicode property ranges using the same Node runtime as the fixtures.
// Property names come from the Unicode Character Database; Node decides support.
import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';

const unicodeVersion='16.0.0';
if(process.versions.unicode!=='16.0')throw new Error(`Expected Node Unicode 16.0, got ${process.versions.unicode}`);
const directory=new URL('./unicode/',import.meta.url);
mkdirSync(directory,{recursive:true});
const sources={};
async function rows(name){
 const file=new URL(name,directory);
 let raw;
 try{raw=readFileSync(file);}catch(error){
  if(error.code!=='ENOENT')throw error;
  const response=await fetch(`https://www.unicode.org/Public/${unicodeVersion}/ucd/${name}`);
  if(!response.ok)throw new Error(`${name}: HTTP ${response.status}`);
  raw=Buffer.from(await response.arrayBuffer());writeFileSync(file,raw);
 }
 sources[name]={url:`https://www.unicode.org/Public/${unicodeVersion}/ucd/${name}`,sha256:createHash('sha256').update(raw).digest('hex')};
 return raw.toString('utf8').split('\n').map(line=>line.split('#')[0].trim()).filter(Boolean).map(line=>line.split(';').map(value=>value.trim()));
}
const propertyRows=await rows('PropertyAliases.txt');
const valueRows=await rows('PropertyValueAliases.txt');
const aliases={};
function supported(name){try{new RegExp(`\\p{${name}}`,'u');return true;}catch{return false;}}
function add(name,canonical){if(supported(name))aliases[name]=canonical;}
for(const row of propertyRows){const canonical=row[1];if(supported(canonical))for(const name of row)add(name,canonical);}
for(const name of ['ASCII','Any','Assigned'])add(name,name);
for(const [property,short,long,...extra] of valueRows){
 if(!['gc','sc'].includes(property))continue;
 const values=[short,long,...extra];
 const families=property==='gc'?[['gc','General_Category']]:[['sc','Script'],['scx','Script_Extensions']];
 for(const names of families){const canonical=`${names[1]}=${long}`;if(!supported(canonical))continue;for(const name of names)for(const value of values)add(`${name}=${value}`,canonical);if(property==='gc')for(const value of values)add(value,canonical);}
}
function sequence(first,last){let result='';for(let base=first;base<=last;base+=4096){const part=[];for(let cp=base;cp<=Math.min(last,base+4095);cp++)part.push(cp);result+=String.fromCodePoint(...part);}return result;}
const chunks=[sequence(0,0xd7ff),sequence(0xe000,0x10ffff)];
const ranges={};
for(const name of [...new Set(Object.values(aliases))].sort()){
 const pattern=new RegExp(`\\p{${name}}+`,'gu');const found=[];
 for(const chunk of chunks)for(const match of chunk.matchAll(pattern)){
  const value=match[0];const low=value.charCodeAt(value.length-1);
  const last=value.codePointAt(low>=0xdc00&&low<=0xdfff?value.length-2:value.length-1);
  found.push([value.codePointAt(0),last]);
 }
 const scalar=new RegExp(`^\\p{${name}}$`,'u');
 for(let cp=0xd800;cp<=0xdfff;cp++)if(scalar.test(String.fromCharCode(cp)))found.push([cp,cp]);
 found.sort((a,b)=>a[0]-b[0]);const merged=[];
 for(const entry of found){const previous=merged.at(-1);if(previous&&entry[0]===previous[1]+1)previous[1]=entry[1];else merged.push(entry);}
 ranges[name]=merged;
}
const result={unicodeVersion,nodeVersion:process.versions.node,sources,aliases:Object.fromEntries(Object.entries(aliases).sort()),ranges};
writeFileSync('../../commons/regex/unicode_properties.json',JSON.stringify(result)+'\n');
console.log(`${Object.keys(aliases).length} exact aliases and ${Object.keys(ranges).length} Unicode property tables`);
