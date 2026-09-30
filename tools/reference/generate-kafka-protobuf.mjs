import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
const require = createRequire(import.meta.url);
const { kafkaConsumer } = require('@aws-lambda-powertools/kafka');
const protobuf = require('protobufjs');
const descriptor = require('protobufjs/ext/descriptor');
const cases = [];
const normalize = value => value === undefined ? { $:'undefined' } : value;
async function run(name, data, metadata, mode = 'inspect') {
  const calls = [];
  let value = null, error = null;
  const message = { decode(input, length) {
    const buffer = input instanceof Uint8Array ? input : input.buf;
    const position = input instanceof Uint8Array ? 0 : input.pos;
    calls.push({ position, explicitLength: length !== undefined, buffer:[...buffer] });
    if (mode === 'reject') throw new RangeError('decoder rejected at ' + position);
    if (mode === 'marker' && buffer[position] !== 42) throw new Error('expected marker at ' + position);
    return { position, length: normalize(length), bytes:[...buffer.subarray(position)] };
  } };
  try {
    value = await kafkaConsumer(async event => event.records[0].value,{value:{type:'protobuf',schema:message}})({records:{topic:[{value:data,headers:[],valueSchemaMetadata:metadata}]}},{});
  } catch(e) { error={name:e.name,message:e.message}; }
  cases.push({name,data,metadata:normalize(metadata),mode,calls,value,error});
}
const b64 = b => Buffer.from(b).toString('base64');
for (const meta of [undefined,null,{}, {schemaId:undefined}, {schemaId:null}, {schemaId:'123'}, {schemaId:'1234567890'}, {schemaId:'12345678901'}, {schemaId:'abcdefab-cdef-1234-5678-abcdefabcdef'}, {schemaId:42}, {schemaId:false}, {schemaId:'😀😀😀😀😀'}, {schemaId:'😀😀😀😀😀😀'}]) {
  for (const bytes of [[],[0],[0,42],[1,0,42],[2,0,42],[128],[255,255,255,255,15,42],[255,255,255,255,255,255,255,255,255,1,42]]) {
    await run('prefix-' + cases.length,b64(bytes),meta);
  }
}
for (const bytes of [[2,0,42],[2,0,42],[1,0,42],[1,0,42],[128,0,42],[0,42],[3,0,0,42],[4,0,0,42]]) await run('adaptive-' + cases.length,b64(bytes),{schemaId:'17'},'marker');
for (const bytes of [[0,42],[2,0,42],[],[128],[5,0],[255,255,255,255,15]]) await run('first-error-' + cases.length,b64(bytes),{schemaId:'17'},'reject');
for (const data of ['!!!ACo','ACo','ACo=\n','____','-w==']) await run('base64-' + cases.length,data,{});
// JavaScript reads schemaId.length rather than requiring a string schema ID.
// Object lengths use relational numeric coercion; arrays use their own length.
for (const length of [null, false, true, 0, 10, 10.5, 11, -11, '', '11', ' 11 ', '0xb', '1.1e1', 'Infinity', 'invalid', [], [11], ['11'], [[11]], [1,1], {}, { value: 11 }]) {
  for (const bytes of [[1,42], [0,42], [128], []]) {
    await run('schema-length-' + cases.length, b64(bytes), { schemaId: { length } });
  }
}
for (const id of [[], Array(10).fill(0), Array(11).fill(0), {}]) {
  await run('schema-id-array-' + cases.length, b64([1,42]), { schemaId: id });
}
for (const metadata of [0, false, 'metadata', [], [1]]) {
  await run('metadata-native-' + cases.length, b64([1,42]), metadata);
}
const nativeRoot = protobuf.parse('syntax="proto2"; package fixture; message Payload { optional string name=1; optional int64 id=2; optional uint64 count=3; optional bytes raw=4; optional bool enabled=5; repeated sint32 scores=6; enum State { NEW=0; DONE=1; } optional State state=7; optional Payload child=8; optional double ratio=9; }').root;
const type = nativeRoot.lookupType('fixture.Payload');
const native = [];
for (const input of [{},{name:'',id:'0',enabled:false,ratio:0},{name:'é😀',id:'9223372036854775807',count:'18446744073709551615',raw:Buffer.from([0,255,128]).toString('base64'),enabled:true,scores:[-1,0,2147483647],state:'DONE',child:{name:'child',id:'-9223372036854775808'},ratio:1.5}]) {
  const bytes = Buffer.from(type.encode(type.fromObject(input)).finish());
  for (const mode of ['plain','glue','confluent']) {
    const wire = mode === 'plain' ? bytes : Buffer.concat([Buffer.from([0]),bytes]);
    const metadata = mode === 'plain' ? {} : {schemaId:mode === 'glue' ? '00000000-0000-0000-0000-000000000000' : '42'};
    const value = await kafkaConsumer(async event => event.records[0].value.toJSON(),{value:{type:'protobuf',schema:type}})({records:{topic:[{value:wire.toString('base64'),headers:[],valueSchemaMetadata:metadata}]}},{});
    native.push({mode,data:wire.toString('base64'),metadata,value});
  }
}
const schema = Buffer.from(descriptor.FileDescriptorSet.encode(nativeRoot.toDescriptor('proto2')).finish()).toString('base64');
mkdirSync('../../kafka/protobuf/testdata',{recursive:true});
writeFileSync('../../kafka/protobuf/testdata/typescript-v2.35.0.json',JSON.stringify({version:'2.35.0',protobufVersion:require('protobufjs/package.json').version,cases,schema,native},null,2)+'\n');
console.log('Wrote ' + cases.length + ' Protobuf prefix scenarios and ' + native.length + ' native messages');
