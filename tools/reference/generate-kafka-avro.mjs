import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
const require = createRequire(import.meta.url);
const avro = require('avro-js');
const { kafkaConsumer } = require('@aws-lambda-powertools/kafka');
function normalize(value) {
  if (Buffer.isBuffer(value)) return { $: 'bytes', data: [...value] };
  if (typeof value === 'number' && (!Number.isFinite(value) || Object.is(value, -0))) return { $: Object.is(value, -0) ? '-0' : String(value) };
  if (Array.isArray(value)) return value.map(normalize);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key,v]) => [key,normalize(v)]));
  return value;
}
const cases = [];
async function record(name, schema, data, metadata = { dataFormat: 'AVRO' }) {
  let value = null, error = null;
  try {
    value = await kafkaConsumer(async event => normalize(event.records[0].value), { value: { type: 'avro', schema } })({ records: { topic: [{ value: data, headers: [], valueSchemaMetadata: metadata }] } }, {});
  } catch (e) { error = { name: e.name, message: e.message }; }
  cases.push({ name, schema, data, metadata, value, error });
}
async function valid(schema, values) {
  const text = JSON.stringify(schema);
  const type = avro.parse(text);
  for (const value of values) {
    const bytes = type.toBuffer(value);
    await record('valid-' + cases.length, text, bytes.toString('base64'));
    if (bytes.length) {
      await record('truncated-' + cases.length, text, bytes.subarray(0, -1).toString('base64'));
      await record('trailing-' + cases.length, text, Buffer.concat([bytes, Buffer.from([0])]).toString('base64'));
    }
  }
}
await valid('null', [null]);
await valid('boolean', [false,true]);
await valid('int', [-2147483648,-1,0,1,2147483647]);
await valid('long', [-9007199254740990,-2147483649,-1,0,1,2147483648,9007199254740990]);
await valid('float', [0,-0,1.5,-1.25,1e30,Infinity,-Infinity,NaN]);
await valid('double', [0,-0,Number.MIN_VALUE,Number.MAX_VALUE,1.25,Infinity,-Infinity,NaN]);
await valid('string', ['', 'hello', 'é😀', '\uFEFFvalue', '\u0000']);
await valid('bytes', [Buffer.alloc(0), Buffer.from([0,255,128,65])]);
await valid({ type:'fixed', name:'Token', namespace:'sample', size:4 }, [Buffer.from([0,1,128,255])]);
await valid({ type:'enum', name:'Color', symbols:['RED','GREEN','BLUE'] }, ['RED','BLUE']);
await valid({ type:'array', items:'long' }, [[],[1,2,3],[-1,9007199254740990]]);
await valid({ type:'map', values:'string' }, [{},{ z:'last', a:'first', unicode:'é' }]);
await valid(['null','string','int'], [null,{string:'hello'},{int:42}]);
await valid({ type:'record', name:'Order', namespace:'sample', fields:[{name:'id',type:'string'},{name:'amount',type:'long'},{name:'optional',type:['null','string']},{name:'items',type:{type:'array',items:'bytes'}},{name:'tags',type:{type:'map',values:'int'}}] }, [{id:'a',amount:42,optional:{string:'yes'},items:[Buffer.from([255])],tags:{x:1}},{id:'b',amount:0,optional:null,items:[],tags:{}}]);
await valid({ type:'record', name:'Node', namespace:'graph', fields:[{name:'value',type:'int'},{name:'next',type:['null','Node']}] }, [{value:1,next:null},{value:1,next:{'graph.Node':{value:2,next:null}}}]);
await valid([{type:'record',name:'Thing',namespace:'ns',fields:[{name:'v',type:'int'}]},'null'], [{'ns.Thing':{v:3}},null]);
for (const schema of [{type:'long',logicalType:'timestamp-millis'},{type:'int',logicalType:'date'},{type:'bytes',logicalType:'decimal',precision:10,scale:2},{type:'string',logicalType:'uuid'},{type:'long',logicalType:'unknown'}]) {
  const value = schema.type === 'bytes' ? Buffer.from([1,2]) : schema.type === 'string' ? 'not-a-uuid' : 123;
  await valid(schema,[value]);
}
function zigzag(n) { let value = (n << 1n) ^ (n >> 63n); const bytes = []; do { let b = Number(value & 127n); value >>= 7n; if (value) b |= 128; bytes.push(b); } while(value); return Buffer.from(bytes); }
for (const n of [-9223372036854775808n,-9007199254740992n,-9007199254740991n,9007199254740991n,9007199254740992n,9223372036854775807n]) await record('precision-' + cases.length,'long',zigzag(n).toString('base64'));
for (const bits of [27n,28n,31n,32n,52n,53n,54n,62n]) for (const delta of [-2n,-1n,0n,1n,2n]) for (const sign of [-1n,1n]) for (const schema of ['int','long']) {
  await record('integer-boundary-' + cases.length,schema,zigzag(sign*((1n<<bits)+delta)).toString('base64'));
}
for (const data of ['AQ==','/w==','!!!AQ','AQ','AQ==\n']) await record('permissive-' + cases.length,'boolean',data);
for (const bytes of [[2,255],[4,0xe2,0x82],[6,0xef,0xbb,0xbf]]) await record('utf8-' + cases.length,'string',Buffer.from(bytes).toString('base64'));
for (const schema of ['["null","string"]','{"type":"enum","name":"Color","symbols":["A","B"]}']) for (const index of [-1n,2n,5n]) await record('index-' + cases.length,schema,zigzag(index).toString('base64'));
for (const schema of ['int','boolean','bytes','string','{"type":"fixed","name":"F","size":4}','{"type":"array","items":"int"}','{"type":"map","values":"int"}']) await record('empty-' + cases.length,schema,'');
await record('negative-array-block','{"type":"array","items":"int"}',Buffer.from([3,4,2,4,0]).toString('base64'));
for (const metadata of [{}, {dataFormat:'AVRO',schemaId:'12'}, {dataFormat:'AVRO',schemaId:'11111111-2222-3333-4444-555555555555'}, null]) await record('metadata-' + cases.length,'int','Ag==',metadata);
mkdirSync('../../kafka/avro/testdata',{recursive:true});
writeFileSync('../../kafka/avro/testdata/typescript-v2.35.0.json',JSON.stringify({version:'2.35.0',avroVersion:require('avro-js/package.json').version,cases},null,2)+'\n');
console.log('Wrote ' + cases.length + ' Avro reference scenarios');
