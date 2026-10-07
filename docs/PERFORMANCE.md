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
