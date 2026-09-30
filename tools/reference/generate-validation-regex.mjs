// Preserve actual JavaScript Unicode regex behavior through the pinned utility.
import { readFileSync, writeFileSync } from 'node:fs';
import { validate } from '@aws-lambda-powertools/validation';
const patterns=[
 'a(?=b)','a(?!b)','(?<=a)b','(?<!a)b','^(a|b)\\1$','^(a)?b\\1$', '\\1(a)',
 '^(?<word>a)\\k<word>$','^(?<named>a)(b)\\1\\2$',
 '^.$','^..$','^\\u{1F600}$','^\\uD83D\\uDE00$','^[😀-🙏]$',
 '^\\p{Letter}+$','^\\p{L}+$','^\\p{Script=Greek}+$','^\\p{Script_Extensions=Hiragana}+$','^\\p{Emoji}+$',
 '^\\P{ASCII}+$','^\\p{White_Space}$','^\\s$','^\\S$','^\\w+$','^\\d+$',
 '^a$','a$','[^]','[]','^[^a]+$','^a{0,2}$','(?:a|ab)+?b','\\bword\\b',
 '(?=a)+','[','(','a{2,1}','a++','(?>a)',"(?'name'a)",'(?P<name>a)','(?(1)a|b)',
 '\\a','\\_','\\8','[\\1]','\\u{110000}','\\u{}','\\p{Not_A_Property}','(?i)a',
 '^(?<x>a)\\k<missing>$','^\\cA$','^\\0$','^a\\/b$','^[a-z-]+$',
 '^\\p{Any}$','^\\P{Any}$','^[\\p{ASCII}]+$','^[^\\P{ASCII}]+$',
 '^\\p{gc=Lu}$','^\\p{sc=Grek}$','^\\p{scx=Hira}$','\\p{letter}','\\p{Script=greek}',
 '[a-\\d]','[\\d-a]','^+','$?','\\b*','(?=a){1}','(?i:a)',
 '^(?<x>a)(?<x>b)$','\\u{+1}','\\x+1','\\c_','\\08','\\u{00000000000000000041}',
];
const inputs=['','a','b','ab','aa','aba','bab','abab','word',' word ','1','١','é','Ελληνικά','ひ','ー','😀','🙏','💩','ab\n','a\n','a\r','a\u2028','a\u2029','\n','\r','\u2028','\u2029','\t','\u00a0','\ufeff','\u0085','\u0001','\u0000','a/b'];
const cases=[];
function record(name,schema,payload){
 const item={name,schema,payload};
 try{item.expected={success:true,value:validate({schema,payload})};}
 catch(error){item.expected={success:false,error:error.name,message:error.message};if(Array.isArray(error.cause))item.expected.issues=error.cause;}
 cases.push(item);
}
for(const pattern of patterns)for(const payload of inputs)record('pattern-'+cases.length,{type:'string',pattern},payload);
for(const pattern of ['(?<=pre-)id','^\\p{Letter}+$','^a/b~c$'])for(const key of ['pre-id','bad','Ελληνικά','1','a/b~c'])for(const value of ['ok',1]){
 record('property-'+cases.length,{type:'object',patternProperties:{[pattern]:{type:'string'}},additionalProperties:false},{[key]:value});
}
const tables=JSON.parse(readFileSync('../../validation/unicode_properties.json','utf8'));
for(const [alias,canonical] of Object.entries(tables.aliases)){
 const pattern=`^\\p{${alias}}$`;
 record('alias-empty-'+cases.length,{type:'string',pattern},'');
 const interval=tables.ranges[canonical].find(([start,end])=>start<0xd800||end>0xdfff);
 if(interval){const cp=interval[0]>=0xd800&&interval[0]<=0xdfff?0xe000:interval[0];record('alias-member-'+cases.length,{type:'string',pattern},String.fromCodePoint(cp));}
}
writeFileSync('../../validation/testdata/regex-v2.35.0.json',JSON.stringify({version:'2.35.0',ajvVersion:'8.20.0',unicodeRegExp:true,cases},null,2)+'\n');
console.log(`${cases.length} Unicode regular expression cases`);
