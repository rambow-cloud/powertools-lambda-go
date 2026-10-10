---
description: "Parse API Gateway, ALB and other supported HTTP Lambda event shapes into typed Go models using Powertools Parser."
---

# HTTP Parser contracts

Reference: installed TypeScript Parser v2.35.0 with Zod v4.1.12. This implementation covers all eighteen runtime schema exports from eight HTTP-related source families and five body envelopes. It validates event shapes and payloads; routing, authorization decisions, response validation and OpenAPI belong to the separate Event Handler/Validation work.

## Schema export mapping

These names are exported from the Go `parser/schemas` package with their reference spelling.

| Source family | Go exports | Main behavior |
| --- | --- | --- |
| apigw-proxy | APIGatewayCert, APIGatewayRecord, APIGatewayStringArray, APIGatewayHttpMethod | Certificate structure, string dictionaries/lists and seven-method enum |
| api-gateway | APIGatewayEventRequestContextSchema, APIGatewayProxyEventSchema, APIGatewayRequestAuthorizerEventSchema, APIGatewayTokenAuthorizerEventSchema | REST request/context, REQUEST/TOKEN authorizer events, identity and authorizer variants |
| api-gatewayv2 | APIGatewayProxyEventV2Schema, APIGatewayRequestAuthorizerEventV2Schema, APIGatewayRequestAuthorizerV2Schema, APIGatewayRequestContextV2Schema | HTTP API v2 request/context, JWT/IAM/Lambda identity fields and REQUEST authorizer event |
| api-gateway-websocket | APIGatewayProxyWebsocketEventSchema | WebSocket connection/message event metadata and direction enum |
| alb | AlbSchema, AlbMultiValueHeadersSchema | Ordinary and required multi-value headers/query variants |
| lambda | LambdaFunctionUrlSchema | The reference HTTP API v2 schema with its own extensible object instance |
| vpc-lattice | VpcLatticeSchema | Snake-case v1 event metadata and required body |
| vpc-latticev2 | VpcLatticeV2Schema | Camel-case v2 metadata, identity fields and optional body/Base64 flag |

The implementation reuses shared fields, dictionaries, method enums and certificates where the reference contracts agree. Differences are preserved:

- REST query dictionaries and body permit explicit null but are required; several other REST dictionaries are optional and nullable.
- REST Lambda authorizers preserve custom context values alongside integrationLatency/principalId, as specified by the [AWS authorizer contract](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-lambda-authorizer-output.html). Contexts containing claims use the typed Cognito branch, including scopes validation, even when Lambda metadata is present. This corrects custom-field stripping in the pinned model; schemas validate shape rather than make authorization decisions.
- HTTP API v2 body is optional but not nullable. Its version is a string; the REQUEST authorizer event specifically requires `2.0`.
- REST identity permits `test-invoke-source-ip` and an absent sourceIp. HTTP API v2 requires a valid IPv4/IPv6 address. WebSocket identity only requires a string.
- REST request context permits a nonempty messageId only when eventType is MESSAGE.
- ALB permits an arbitrary method string, while API Gateway and VPC Lattice use the reference seven-method enum.
- VPC Lattice v2 headers and query values are string arrays, including single values; v1 keeps scalar strings. V2 preserves an optional requestId, and timeEpoch remains a string. Identity recognizes the AWS spellings principalOrgID, x509SubjectCn, x509IssuerOu and x509SanNameCn, with legacy aliases retained. These intentionally correct the pinned TypeScript model against the [AWS service contract](https://docs.aws.amazon.com/vpc-lattice/latest/ug/lambda-functions.html#receive-event-from-service); requestId is optional, as reported in the [upstream event example](https://github.com/aws-powertools/powertools-lambda-typescript/issues/5764).

## Body envelopes

| TypeScript | Go constructor | Base schema |
| --- | --- | --- |
| ApiGatewayEnvelope | envelopes.APIGateway | REST proxy event |
| ApiGatewayV2Envelope | envelopes.APIGatewayV2 | HTTP API v2 proxy event |
| LambdaFunctionUrlEnvelope | envelopes.LambdaFunctionURL | Lambda URL event |
| VpcLatticeEnvelope | envelopes.VpcLattice | VPC Lattice v1 |
| VpcLatticeV2Envelope | envelopes.VpcLatticeV2 | VPC Lattice v2 |

Each envelope replaces the base body's schema with the application schema and validates the whole event. Metadata and payload issues can therefore appear together, in schema field order. Payload issue paths start with `body`. The application schema controls whether an absent/null body is accepted, independently of the base event body's optional/nullable policy.

No automatic JSON or Base64 decoding occurs. Use `JSONStringified` or `Base64Encoded` explicitly as appropriate for the input. A true isBase64Encoded/is_base64_encoded flag alone leaves the body unchanged. There is no ALB envelope in the pinned reference; use AlbSchema extension or explicit body parsing.

```go
payload := parser.Typed[Order](parser.Object(
    parser.Field{Name: "id", Schema: parser.String()},
    parser.Field{Name: "amount", Schema: parser.Number()},
))
schema := envelopes.APIGatewayV2(parser.JSONStringified(payload))
result, err := parser.SafeParse(ctx, event, schema)
```

See the complete [HTTP Lambda example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/parser/http/main.go) for safe parsing and HTTP 400 handling. It keeps operational errors/cancellation distinct from validation failures.

EventBridge and the HTTP envelopes share a private object-field validator. It preserves an application schema's safe-mode interface, returns a zero value for accepted absence and does not store per-request results in shared state. Extending the selected payload rule leaves the exported base model unchanged.

## Evidence and remaining boundaries

Local acceptance on 2026-09-15 passed all eighteen packaged modules, fifteen standalone consumers, both CGO-disabled Lambda architecture builds, 226/226 Docker assertions and 14/14 Batch artifact checks. Runtime checks execute all five new body envelopes, ALB multi-value headers and combined metadata/body failures. Disposable containers and their internal network were removed. See MODULE_ACCEPTANCE.json, LOCAL_ACCEPTANCE.json and BATCH_ACCEPTANCE.json; no AWS deployment was performed.

`generate-parser-http.mjs` runs the actual pinned packages and writes 458 cases to `parser/testdata/http-v2.35.0.json`. Cases include valid/missing/null schemas, individual top-level field omission/type mutations, nested certificates/identity variants, IPv4/IPv6 and invalid addresses, REST messageId refinement, ordinary/safe envelopes, combined metadata/payload errors, missing/null bodies, application object bodies and explicit no-decoding behavior. Existing core/stream cases remain part of the same Parser regression suite (604 cases total).

Additional Go tests cover accepted absence, concurrent schema reuse, unchanged base schemas, safe versus ordinary payload modes, cancellation, nil schemas and operational error identity. The shared field extractor also fixes absent EventBridge unknown detail returning an internal absence value.

Full inferred-type/export mapping, nested Zod error trees, exception-class identity, every IP/encoding/type edge and performance budgets remain open. REST request-context refinement is a composed Go Schema interface; it does not expose Zod-specific refinement/extension methods. HTTP models and envelopes do not constitute an implemented Event Handler router or JSON Schema Validation module.
