# JMESPath

JMESPath queries JSON documents, decodes Powertools envelopes and supports custom functions. Import `github.com/rambow-cloud/powertools-lambda-go/jmespath`. It is optional for Logger and does not contact AWS.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Complete example

Run this complete offline example with `go run ./examples/query` and `CGO_ENABLED=0`. It decodes the JSON string inside an SQS body, then uses a compiled expression to set Logger's correlation ID.

~~~go
--8<-- "examples/query/main.go"
~~~

## Input and output

The first stdout line is Go's printed result, `[map[orderId:order-1]]`; it is not JSON. The next line is an `INFO` JSON record with `message: "order received"`, `correlation_id: "order-1"` and default `service: "service_undefined"`. Its timestamp varies. The wrapped callback returns `"ok"` separately. Compile expressions once when reused; inspect the returned `any` or marshal it to JSON for an application response.

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `query` | `Compile` returns an expression or syntax error; `Search` evaluates it against one input. |
| `payloads` | Decoded query result, not the original event wrapper. |
| `CorrelationExtractor` | Logger consumes the small `Search(any)` interface; this module is installed only when your app imports it. |

## TypeScript feature coverage

Compared with the [official v2.35.0 jmespath guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/jmespath.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| Extraction / reusable queries | `Search`, `Compile`, `MustCompile` | Snapshots inputs/results; syntax cache uses deterministic LRU. |
| Built-in envelopes | `ExtractDataFromEnvelope`, thirteen constants | Reference expressions preserved, including first-record selections. |
| Decode functions | `WithPowertoolsFunctions` | JSON/Base64/gzip; Go surfaces decoder errors instead of swallowing them. |
| Custom functions | `WithFunctions`, `Function` | Typed signatures and concurrency-safe callbacks replace subclassing. |
| Logger correlation | Compiled `CorrelationExtractor` | Dependency-free integration interface. |

Executable evidence: [jmespath/jmespath_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/jmespath/jmespath_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.

## Powertools functions and envelopes

Use `WithPowertoolsFunctions()` to enable `powertools_json`, `powertools_base64`, and `powertools_base64_gzip`. All require exactly one string argument. Base64 uses the shared reference validation; gzip verifies decompression errors. Decoding buffers the complete value, so applications should bound untrusted payload sizes.

`ExtractDataFromEnvelope(data, envelope)` enables these functions by default. Explicit options replace that default; include `WithPowertoolsFunctions()` alongside custom definitions when both are needed.

| TypeScript constant | Go constant |
| --- | --- |
| API_GATEWAY_REST | APIGatewayREST |
| API_GATEWAY_HTTP | APIGatewayHTTP |
| SQS | SQS |
| SNS | SNS |
| EVENTBRIDGE | EventBridge |
| CLOUDWATCH_EVENTS_SCHEDULED | CloudWatchEventsScheduled |
| KINESIS_DATA_STREAM | KinesisDataStream |
| CLOUDWATCH_LOGS | CloudWatchLogs |
| S3_SNS_SQS | S3SNSSQS |
| S3_SQS | S3SQS |
| S3_SNS_KINESIS_FIREHOSE | S3SNSKinesisFirehose |
| S3_KINESIS_FIREHOSE | S3KinesisFirehose |
| S3_EVENTBRIDGE_SQS | S3EventBridgeSQS |

Expressions match the pinned constants literally. In particular SNS selects the first record, and the S3 nested envelopes select the first S3 record within each outer record; callers needing other behavior can provide their own expressions.

## Custom functions and Logger integration

```go
option := jmespath.WithFunctions(jmespath.Function{
    Name: "upper",
    Arguments: []jmespath.Argument{{Types: []jmespath.Type{jmespath.String}}},
    Handler: func(args []any) (any, error) {
        return strings.ToUpper(args[0].(string)), nil
    },
})
query, err := jmespath.Compile("upper(name)", option)
```

Signatures support unions, string/number/object/array/boolean/null, homogeneous string/number arrays, expression references, and a final variadic argument. Zero-argument functions are checked as zero-argument functions. Custom definitions may override standard functions. `ExpressionReference` supports user functions accepting an `&expression` argument. Go callbacks replace TypeScript subclassing and decorators.

Pass a compiled expression to `logger.HandlerOptions.CorrelationExtractor`. Logger only depends on the small `Search(any) (any, error)` interface, so installing Logger alone does not install this module. A configured `CorrelationID` callback takes precedence, followed by the extractor, then a built-in source. Extraction failures reach Logger's instrumentation error callback without changing the business result. See the offline [query example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/query/main.go).

## Compatibility and evidence

Seventy-eight cases execute the actual TypeScript v2.35.0 package: standard functions, projections/filters/slices/pipes, numeric/null behavior, all thirteen envelopes, Unicode/BOM handling, and errors. Additional Go tests cover custom functions, signature errors, result/definition isolation, typed events, and 100 concurrent searches. Docker covers decoded projections and Logger correlation alongside Signer and OTel.

`*Error` carries `Kind`, `Expression`, optional `Function`, and an unwrap cause. Syntax errors are grouped instead of reproducing every TypeScript lexer/parser exception class or message; empty expressions have a distinct kind. Function type/arity and unknown-function failures remain distinguishable.

The pinned interpreter silently swallows ordinary decoder and custom-function failures, returning JavaScript `undefined`. Go intentionally returns an error and preserves the cause. Fixtures explicitly record the three decoder `undefined` results rather than treating them as successful null values. JSON object ordering, `to_string` serialization details, malformed UTF-8 replacement boundaries, and extreme numeric behavior still require exhaustive cross-language coverage. This implementation does not claim a complete specification compliance audit or performance budgets.

## Sources

- [Pinned JMESPath implementation](https://github.com/aws-powertools/powertools-lambda-typescript/tree/7bcc27b1574493f9452688673658f52b80c53847/packages/jmespath/src)
- [Go JMESPath engine](https://github.com/jmespath-community/go-jmespath/tree/v1.1.1)
