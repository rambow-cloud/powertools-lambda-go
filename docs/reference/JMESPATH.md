# JMESPath: API and behavior notes

Start with the [JMESPath guide](../JMESPATH.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

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

### Combine envelope decoding and Logger

The maintained example decodes SQS bodies and extracts a Logger correlation ID:

~~~go
--8<-- "examples/query/main.go"
~~~

It prints decoded payloads followed by a log with `correlation_id: "order-1"`.


## Objects and lifecycle

See the [finite boundary audit](../JMESPATH_PLAN.md#finite-boundary-audit-187) for
numeric conversion, JSON v2 serialization and null multi-select evidence, along
with explicit native differences and remaining broad gates.

| Object | How to use it |
| --- | --- |
| Input value | Supply the decoded Go object to query. |
| Compiled expression | Optional reusable query; compile once for repeated searches. |
| Result | Inspect the returned Go value or serialize it as JSON. |

