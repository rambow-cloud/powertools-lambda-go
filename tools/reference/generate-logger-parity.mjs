import { Logger, LogFormatter, LogItem } from '@aws-lambda-powertools/logger';
import { writeFileSync } from 'node:fs';

// This development-only corpus executes the pinned package, not a reimplementation.
const environmentKeys = ['POWERTOOLS_LOG_LEVEL', 'LOG_LEVEL', 'AWS_LAMBDA_LOG_LEVEL',
  'POWERTOOLS_LOGGER_SAMPLE_RATE', 'POWERTOOLS_DEV', 'POWERTOOLS_LOGGER_LOG_EVENT',
  'POWERTOOLS_SERVICE_NAME', 'AWS_LAMBDA_MAX_CONCURRENCY', 'TZ', '_X_AMZN_TRACE_ID'];
const traceA = 'Root=1-12345678-123456789012345678901234;Parent=1234567890123456;Sampled=1';
const traceB = 'Root=1-87654321-123456789012345678901234;Parent=1234567890123456;Sampled=0';
const timestamp = '2026-10-04T00:00:00.000Z';
const OriginalDate = globalThis.Date;
globalThis.Date = class extends OriginalDate {
  constructor(...args) { super(...(args.length ? args : [timestamp])); }
};

class EnvelopeFormatter extends LogFormatter {
  formatAttributes(base, additional) {
    return new LogItem({ attributes: {
      message: base.message, empty: '', null_value: null, nested: additional,
    } });
  }
}

function replacer(mode) {
  if (!mode) return undefined;
  return (key, value) => {
    if (mode === 'root' && key === '') {
      return { ...value, replaced_empty: '', replaced_null: null };
    }
    if (mode === 'values') {
      if (key === 'null_after') return null;
      if (key === 'empty_after') return '';
      if (key === 'empty' || key === 'null_value') return 'RESCUED';
    }
    return value;
  };
}

function execute(spec) {
  for (const key of environmentKeys) delete process.env[key];
  const stdout = process.stdout.write;
  const stderr = process.stderr.write;
  const records = [], byteSizes = [], snapshots = [];
  const collect = chunk => {
    const record = JSON.parse(chunk.toString());
    byteSizes.push(Buffer.byteLength(JSON.stringify(record)));
    delete record.timestamp;
    // JS stack/type/location are intentionally Go-native; compare the error message.
    if (record.error) record.error = { message: record.error.message };
    records.push(record);
    return true;
  };
  process.stdout.write = collect;
  process.stderr.write = collect;
  try {
    const loggers = { parent: new Logger({
      serviceName: 'orders', persistentKeys: spec.persistent || {},
      ...(spec.buffer && { logBufferOptions: {
        maxBytes: spec.maxBytes ?? 20480,
        bufferAtVerbosity: spec.bufferAt || 'DEBUG',
        flushOnErrorLog: !spec.disableFlush,
      } }),
      ...(spec.formatter && { logFormatter: new EnvelopeFormatter() }),
      jsonReplacerFn: replacer(spec.replacer),
    }) };
    for (const step of spec.steps) {
      const logger = loggers[step.target || 'parent'];
      switch (step.op) {
        case 'child': loggers[step.name] = logger.createChild({ persistentKeys: step.fields || {} }); break;
        case 'append': logger.appendKeys(step.fields); break;
        case 'persistent': logger.appendPersistentKeys(step.fields); break;
        case 'remove': logger.removeKeys(step.keys); break;
        case 'removePersistent': logger.removePersistentKeys(step.keys); break;
        case 'reset': logger.resetKeys(); break;
        case 'snapshot': snapshots.push(JSON.parse(JSON.stringify({ target: step.target || 'parent', persistent: logger.getPersistentLogAttributes(), correlation: logger.getCorrelationId() ?? null }))); break;
        case 'trace': if (step.value) process.env._X_AMZN_TRACE_ID = step.value; else delete process.env._X_AMZN_TRACE_ID; break;
        case 'flush': logger.flushBuffer(); break;
        case 'clear': logger.clearBuffer(); break;
        default: logger[step.op](step.message ?? '', ...(step.fields ? [step.fields] : []));
      }
    }
  } finally {
    process.stdout.write = stdout;
    process.stderr.write = stderr;
  }
  return { ...spec, records, snapshots, byteSizes };
}

const specs = [];
const attributeSets = [
  ['scalar', { shared: 'persistent' }, { shared: 'temporary' }, { shared: 'child' }],
  ['nested', { shared: { base: 1, keep: true } }, { shared: { temporary: 2 } }, { shared: { child: 3 } }],
  ['arrays', { shared: [{ base: 1 }, 2, 3] }, { shared: [{ temporary: 2 }, 4] }, { shared: [{ child: 3 }, 5] }],
  ['false-zero', { shared: false, zero: 0 }, { shared: 0, zero: false }, { shared: true }],
  ['null', { shared: 'persistent' }, { shared: null }, { shared: null }],
  ['empty', { shared: 'persistent' }, { shared: '' }, { shared: '' }],
];
for (const [name, persistent, temporary, overrides] of attributeSets) {
  for (const override of [false, true]) {
    specs.push({ name: `child-${name}-${override ? 'override' : 'inherit'}`, category: 'child',
      persistent: { ...persistent, nested: { parent: true } }, steps: [
        { op: 'append', fields: { ...temporary, request: 'original', correlation_id: 'correlation' } },
        { op: 'child', name: 'child', fields: override ? overrides : {} },
        { op: 'snapshot', target: 'child' },
        { op: 'info', target: 'child', message: 'before' },
        { op: 'child', target: 'child', name: 'grandchild', fields: { component: 'grandchild' } },
        { op: 'append', fields: { request: 'parent-later' } },
        { op: 'info', target: 'child', message: 'snapshot' },
        { op: 'remove', target: 'child', keys: ['shared', 'zero', 'request', 'correlation_id'] },
        { op: 'info', target: 'child', message: 'removed' },
        { op: 'snapshot', target: 'child' },
        { op: 'append', target: 'child', fields: { request: 'child-later' } },
        { op: 'reset', target: 'child' },
        { op: 'info', target: 'child', message: 'reset' },
        { op: 'persistent', target: 'child', fields: { nested: { child: true } } },
        { op: 'info', target: 'child', message: 'merged' },
        { op: 'removePersistent', target: 'child', keys: ['shared'] },
        { op: 'info', target: 'child', message: 'persistent removed' },
        { op: 'info', target: 'grandchild', message: 'grandchild snapshot' },
        { op: 'info', message: 'parent unchanged' },
        { op: 'snapshot' },
      ] });
  }
}

const emptyFields = { empty: '', null_value: null, zero: 0, flag: false,
  list: [], object: {}, nested: { empty: '', null_value: null },
  null_after: 'value', empty_after: 'value' };
for (const source of ['call', 'persistent', 'temporary']) {
  for (const formatter of [false, true]) {
    for (const mode of ['', 'values', 'root']) {
      specs.push({ name: `empty-${source}-${formatter ? 'custom' : 'default'}-${mode || 'plain'}`,
        category: 'empty', formatter, replacer: mode,
        ...(source === 'persistent' && { persistent: emptyFields }),
        steps: [
          ...(source === 'temporary' ? [{ op: 'append', fields: emptyFields }] : []),
          { op: 'info', message: '', ...(source === 'call' && { fields: emptyFields }) },
          { op: 'info', message: 'nonempty', ...(source === 'call' && { fields: emptyFields }) },
        ] });
    }
  }
}

const trace = value => ({ op: 'trace', value });
const log = (op, message, fields) => ({ op, message, ...(fields && { fields }) });
const flush = { op: 'flush' }, clear = { op: 'clear' };
const bufferCases = [
  ['trace-change-flush', [trace(traceA), log('debug', 'old'), trace(traceB), flush, log('debug', 'new'), flush]],
  ['trace-change-clear', [trace(traceA), log('debug', 'retained'), trace(traceB), clear, trace(traceA), flush]],
  ['missing-trace', [trace(traceA), log('debug', 'retained'), trace(''), clear, flush, trace(traceA), flush]],
  ['missing-trace-filtering', [trace(''), log('debug', 'hidden'), log('info', 'visible'), flush]],
  ['error-flush', [trace(traceA), log('debug', 'detail'), log('error', 'failure'), flush]],
  ['critical-flush', [trace(traceA), log('debug', 'detail'), log('critical', 'failure'), flush]],
  ['wrong-trace-error', [trace(traceA), log('debug', 'old'), trace(traceB), log('error', 'new failure'), trace(traceA), flush]],
  ['clear-eviction', [trace(traceA), log('debug', 'first'), log('debug', 'second'), clear, log('debug', 'fresh'), flush], { maxBytes: 300 }],
  ['eviction', [trace(traceA), ...['first', 'second', 'third'].map(m => log('debug', m)), flush, flush], { maxBytes: 300 }],
  ['oversize', [trace(traceA), log('debug', 'oversize'), flush], { maxBytes: 10 }],
  ['child-independent', [trace(traceA), log('debug', 'parent'), { op: 'child', name: 'child' }, { op: 'debug', target: 'child', message: 'child' }, { op: 'flush', target: 'child' }, flush]],
  ['disabled-error-flush', [trace(traceA), log('debug', 'detail'), log('error', 'failure'), flush], { disableFlush: true }],
  ['empty-buffered', [trace(traceA), log('debug', '', emptyFields), flush]],
  ['info-buffer', [trace(traceA), log('info', 'deferred'), flush], { bufferAt: 'INFO' }],
];
for (const [name, steps, options] of bufferCases) {
  specs.push({ name: `buffer-${name}`, category: 'buffer', buffer: true, ...options, steps });
}
// Derive capacity from an actual UTF-8 reference record, retaining its timestamp.
const sizing = execute({ name: 'sizing', steps: [trace(traceA), log('info', '界α')] }).byteSizes[0];
for (const [name, maxBytes] of [['exact', sizing], ['one-byte-small', sizing - 1], ['two-exact', sizing * 2]]) {
  specs.push({ name: `buffer-utf8-${name}`, category: 'buffer', buffer: true,
    bufferAt: 'INFO', maxBytes,
    steps: [trace(traceA), log('info', '界α'), ...(name === 'two-exact' ? [log('info', '界α')] : []), flush] });
}
specs.push({ name: 'buffer-oversize-new-trace', category: 'buffer', buffer: true, maxBytes: 300,
  steps: [trace(traceA), log('debug', 'old'), trace(traceB), log('debug', 'x'.repeat(400)), trace(traceA), flush, trace(traceB), flush] });

try {
  const cases = specs.map(execute);
  writeFileSync(new URL('../../logger/testdata/parity-v2.35.0.json', import.meta.url),
    JSON.stringify({ upstream: '2.35.0', timestamp, cases }, null, 2) + '\n');
  console.log(`Generated ${cases.length} Logger parity scenarios.`);
} finally {
  globalThis.Date = OriginalDate;
}
