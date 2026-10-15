# Local performance baseline

The first baseline covers eleven representative operations across six core
modules. It is a local Windows amd64 workspace measurement on Go 1.27.1 with
`CGO_ENABLED=0`, `GOMAXPROCS=2`, one 200ms measurement per benchmark, and a fixed
small payload. Exact source identity, environment and results are recorded in
[PERFORMANCE_BASELINE.json](PERFORMANCE_BASELINE.json). Track acceptance in
[#113](https://github.com/rambow-cloud/powertools-lambda-go/issues/113).

| Operation | ns/op | B/op | allocs/op | Output B/op |
| --- | ---: | ---: | ---: | ---: |
| Logger emitted | 8,357 | 5,256 | 116 | 208 |
| Logger filtered | 454.7 | 528 | 4 | 0 |
| Metrics add and publish | 2,809 | 2,112 | 40 | 191 |
| Parser valid object | 532.6 | 384 | 8 | — |
| Parser invalid object | 583.5 | 864 | 13 | — |
| Compiled Validation valid | 3,838 | 2,321 | 55 | — |
| Compiled Validation invalid | 9,067 | 6,774 | 158 | — |
| Idempotency store replay | 3,286 | 2,073 | 61 | — |
| Idempotency local-cache replay | 3,202 | 2,089 | 60 | — |
| Parameters JSON cache hit | 209.6 | 336 | 2 | — |
| Parameters forced fetch/transform | 798.1 | 744 | 9 | — |

## Reproduce and compare

Run from each of `logger`, `metrics`, `parser`, `validation`, `idempotency` and
`parameters` in the workspace:

```powershell
$env:CGO_ENABLED = '0'
$env:GOMAXPROCS = '2'
go test -p 1 -run '^$' -bench Benchmark -benchmem -benchtime=200ms -count=1 .
```

For a suspected regression, compare the same benchmark, Go version, operating
system, architecture, payload, CPU limit and output backend. Repeat measurements
only to investigate a meaningful change or unresolved variability. Review
allocations and output sizes alongside time; this single-run baseline does not
define statistically justified pass/fail percentages or universal latency budgets.

## Logger hot-path comparison (#248)

The Logger optimization compares the same benchmark source against commit
`35f9d088e9ca96d5718f4ca1a793332c756c2f2c` and the updated implementation.
Both runs use Go 1.27.1, Windows amd64, `CGO_ENABLED=0`, `GOMAXPROCS=2`, a
counting writer and one 300ms measurement per case. The original baseline above
remains a separate historical measurement.

| Operation | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Logger emitted, 208 output bytes | 9,771 | 5,249 | 5,592 | 2,752 | 118 | 34 |
| Invocation-bound INFO, 404 output bytes | 14,750 | 6,939 | 9,417 | 4,448 | 193 | 53 |
| Bind context and INFO, 404 output bytes | 14,812 | 7,764 | 9,704 | 4,737 | 194 | 54 |
| Complete wrapper and INFO, 404 output bytes | 15,696 | 7,715 | 11,377 | 6,409 | 208 | 68 |
| Logger filtered | 544.2 | 15.68 | 528 | 0 | 4 | 0 |
| Invocation-bound filtered | 285.3 | 16.45 | 432 | 0 | 5 | 0 |

All emitted cases retain their output size. The invocation benchmark also includes
a direct JSON control with identical enriched fields: 47 allocations and 2,773
bytes per operation in both runs. That control omits Logger normalization, root
ordering and synchronization. The Logger cases preserve those behaviors, error
atomicity, formatter/replacer hooks and the main-only stack Preview.

The improvements come from sharing one JSON v2 encoder, processing ordinary maps
and scalars without reflective iteration, reducing ordering/output copies, and
avoiding unused trace lookup on filtered/unbuffered paths. Source hashes and all
seven raw benchmark summaries are recorded in
[PERFORMANCE_LOGGER_248.json](PERFORMANCE_LOGGER_248.json).

Run from `logger` with CGO disabled:

```sh
CGO_ENABLED=0 GOMAXPROCS=2 go test -p 1 -run '^$' -bench '^BenchmarkLogger($|Invocation)' -benchmem -benchtime=300ms -count=1 .
```

These single-run local observations locate overhead; they do not establish a
statistical regression budget or predict Linux Lambda latency. CloudWatch/stdout
I/O, CPU quotas, GC under large-event workloads and cold starts require separate
measurements. No AWS benchmark was run for this change.

## JSON v2 snapshot and normalization comparison (#250–#252)

These changes keep JSON v2 and reduce conversions around it. An internal private
copy handles ordinary JSON-shaped inputs for correlation extraction and JMESPath.
Unsupported types, custom marshalers, raw JSON, invalid values, and deep/cyclic
inputs retain the JSON v2 round trip. The copy preserves float64 numbers and
default empty-container behavior; it does not bypass errors in unrelated fields.
Validation decodes an existing serialized snapshot directly with exact-number
options. Logger combines root preparation/normalization without callbacks and
avoids unused array-index strings while preserving the general callback path.

The same new benchmark sources run against `d991b5af2d0e` and the candidate.
Measurements use Go 1.27.1, Windows amd64, `CGO_ENABLED=0`, `GOMAXPROCS=2`, and
one 500ms run per case. Large Logger/correlation/JMESPath cases use 2,048 records
with 512-byte messages. The large Validation case contains a 2 MiB string body;
it is a different workload from the earlier array-event experiments.

| Operation | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Invocation-bound INFO, 404 output bytes | 7,623 | 6,937 | 4,448 | 3,496 | 53 | 48 |
| Large event log, 1,141,832 output bytes | 5,078,787 | 4,314,234 | 3,963,179 | 3,590,002 | 26,406 | 22,569 |
| Large event correlation snapshot | 4,405,535 | 748,599 | 5,933,979 | 868,712 | 30,765 | 8,196 |
| Large event JMESPath lookup | 4,207,010 | 790,587 | 5,963,999 | 868,767 | 30,764 | 8,197 |
| Compiled Validation, small valid input | 5,044 | 4,184 | 2,321 | 2,241 | 55 | 52 |
| Compiled Validation, large string input | 4,763,843 | 2,955,717 | 6,512,209 | 4,484,167 | 39 | 33 |

The identical direct-JSON invocation control retains 47 allocations, 2,773 bytes
and 404 output bytes. Filtered Logger calls retain zero allocations. Full raw
benchmark output and candidate production-source hashes are recorded in
[PERFORMANCE_JSON_V2_250.json](PERFORMANCE_JSON_V2_250.json).

Run the maintained benchmarks from their owning modules with CGO disabled:

```sh
CGO_ENABLED=0 GOMAXPROCS=2 go test -run '^$' -bench 'Benchmark(Logger|CorrelationSnapshot)' -benchmem -benchtime=500ms -count=1 .
CGO_ENABLED=0 GOMAXPROCS=2 go test -run '^$' -bench '^BenchmarkSearchSnapshot$' -benchmem -benchtime=500ms -count=1 .
CGO_ENABLED=0 GOMAXPROCS=2 go test -run '^$' -bench 'BenchmarkValidation(Compiled|LargeSnapshot)' -benchmem -benchtime=500ms -count=1 .
```

For a baseline checkout without the new benchmark files, copy only the matching
`snapshot_benchmark_test.go` from the candidate before measuring. The root
`internal/jsonvalue` benchmark also compares the private copy with a JSON v2
round-trip control in the same binary. These are descriptive local measurements,
not a Lambda performance guarantee or permission to emit oversized cloud logs.
No AWS tests, new codec/parser, external dependencies or release versions are
included. Logger and JMESPath pin the internal root module to commit
`e300c9596f35` because the released root v1.2.0 lacks the new snapshot helper.
The corresponding Go pseudo-version and workspace mapping keep independent
branch consumers buildable without filesystem replacements in module files.

## Measurement boundaries

Construction and schema compilation happen outside the timed operations. Clocks
are fixed and diagnostics are disabled in Idempotency. Logger/Metrics writers
count bytes without retaining documents. Metrics flushes each iteration and
verifies that no metrics remain. Parser/Validation measure success and failure
with reused schemas. Parameters verifies whether the fetch callback ran; forced
fetch includes transformation and replacement of one cache entry, without service
latency. Idempotency seeds one completed record and verifies replay never invokes
the handler again. Its existing in-memory test double is not a production store;
bounded operation-trace bookkeeping remains inside the measurement.

These benchmarks retain bounded output/cache/store state. They do not measure
Lambda cold starts, Linux/arm64 performance, concurrent throughput, long-lived
heap behavior, binary size, AWS/KMS network latency or Redis topology. They are
the first comparison baseline, not completed performance acceptance for every
module. Broader resource budgets remain tracked in CHECKLIST.md and #112.
