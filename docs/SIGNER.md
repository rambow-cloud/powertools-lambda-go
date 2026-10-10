---
description: "Sign Go HTTP requests using AWS Signature Version 4 with Powertools Signer, credentials and reusable signing transports."
---

# Signer

Signer adds AWS Signature Version 4 authentication to HTTP requests. Import `github.com/rambow-cloud/powertools-lambda-go/signer`. Standalone signing does not send a request; `HTTPClient` provides a signed transport.

See [installation](MODULES.md) and the [compatibility baseline](COMPATIBILITY.md).

## Install

Use Go 1.27 or newer and install the module in your own application:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/signer@v1.1.0
```

## Complete example

Save this program as `main.go` in your application and run `CGO_ENABLED=0 go run .`. It uses synthetic credentials and signs API Gateway, Lambda Function URL and AppSync requests without sending them.

~~~go
--8<-- "examples/signing/main.go"
~~~

## Input and output

Stdout is exactly the following. `true` means the signed copy contains an Authorization header. These lines are ordinary program output, not structured Logger records. The example prints neither credentials nor signatures and does not establish service-side authorization.

~~~text
execute-api true
lambda true
appsync true
~~~

## Common tasks

- [Choose a service, region and credentials](#configuration-and-errors).
- [Use a signed HTTP client](#ownership-redirects-and-composition).

## Configuration and errors

`Region` defaults to the raw `AWS_REGION` value. Service and resolved region must be nonempty. The default provider reads `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` for each signing operation, allowing environment credential refresh. It makes no credential-discovery network requests. Supply any `aws.CredentialsProvider` for static credentials, role assumption, profiles, or caching; callers own that provider and its I/O. `aws.CredentialsProviderFunc` and `aws.NewCredentialsCache` can be used directly.

`Clock` supports deterministic signing tests. The constructor returns `*ConfigError` for missing configuration, and the default provider returns it for absent credential variables. Injected provider errors propagate unchanged. Body/request/signing failures use `*SigningError` with `Unwrap`; context cancellation remains discoverable with `errors.Is`.

## Ownership, redirects, and composition

`Sign(*http.Request)` signs without sending. It clones the URL and headers and returns a replayable body with `GetBody`. If the input supplies `GetBody`, its body stream is untouched. Otherwise the finite stream is buffered and restored on the original request, preserving its `Close`. After a read failure the successfully read prefix is restored in front of the unread stream. Standalone callers close both bodies and must not use the input concurrently with signing. Buffering is proportional to payload size; context checks cannot forcibly interrupt an arbitrary blocking application Reader.

`Transport` accepts any `Signer` interface and any `http.RoundTripper`. It closes the incoming body if signing fails. After handoff, the downstream transport owns the signed body's lifetime; closing it also releases the incoming body. Custom signers may return the input request or a clone sharing its body. The base transport can finish reading and closing that body after `RoundTrip` returns, including error paths, as permitted by [Go's transport contract](https://pkg.go.dev/net/http#RoundTripper). Empty signed bodies retain Go's empty-body framing and release the incoming body immediately. It performs no retry loop. `HTTPClient` copies its supplied client and defaults to stopping at redirects. Supply a deliberate `CheckRedirect` policy to allow trusted destinations; each permitted redirect is signed again, and normal Go replay rules apply to 307/308 responses.

Compose with tracing as `signer.HTTPClient(s, tr.HTTPClient(baseClient))`. Neither utility imports the other. AWS SDK service clients already sign their own requests and must not receive another signing layer.

## Reference scope and intentional differences

??? info "Reference evidence and compatibility details"

    Reference: TypeScript v2.35.0. Thirteen fixed-time cases cover GET/JSON/binary/empty bodies, escaped paths, ports, repeated/whitespace headers, session tokens, explicit unsigned payloads, encoded queries, and service names. Twelve signatures match exactly. The duplicate-query case records an intentional difference: the upstream request converter signs only the last occurrence while returning the original URL. Go signs every occurrence, preserving the original URL and a valid canonical query.

    Other Go contracts are explicit: empty `Region` means environment fallback; missing service is rejected; malformed query encoding and inconsistent Content-Length are rejected. Redirects require an explicit policy. Signer uses Go request-body ownership, and Go transport-generated headers are not signed unless supplied explicitly. Custom Host values and service-specific URL/path normalization follow the Go SDK; these do not establish every web Request conversion edge case. General presigning and SigV4a are not part of the pinned Powertools Signer API and are not implemented here.

    Tests cover body replay/errors, configuration refresh, cancellation, custom transports, redirects, and concurrent use. Local Docker verifies synthetic signatures and OTel composition. These checks do not establish live IAM authorization, service-side acceptance for every path, or performance budgets.


## Sources

??? info "Reference evidence and compatibility details"

    - [Pinned Signer implementation](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/signer/src/SigV4Signer.ts)
    - [AWS SDK for Go v2 SigV4](https://github.com/aws/aws-sdk-go-v2/tree/v1.47.0/aws/signer/v4)

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `s` | Reusable service, region, clock and credentials-provider configuration. |
| `request` / `signed` | `Sign` returns a signed copy; the caller owns body closure and request replay rules. |
| `HTTPClient` | Copied client with a signing transport; credentials, retries and redirects stay explicit. |

## TypeScript feature coverage

??? info "Compare with TypeScript v2.35.0"

    Compared with the [official v2.35.0 signer guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/signer.md) and the pinned npm implementation. The table maps capabilities; it does not certify every native type or service behavior.

    | TypeScript feature | Go API or approach | Compatibility scope |
    | --- | --- | --- |
    | Signed fetch / other clients | `Sign`, `Transport`, `HTTPClient` | Go request/transport interfaces replace fetch. |
    | Region / credentials | `Config.Region`, `Credentials` | Environment or injected SDK provider; no automatic config-loader network calls. |
    | Errors | `ConfigError`, `SigningError` | Unwrap causes/cancellation; unsigned request is not sent on failure. |
    | Bodies / redirects | Replayable signed copies and client policy | Ownership and trusted redirect policy differ from JavaScript Request. |

    Executable evidence: [signer/signer_test.go](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/signer/signer_test.go). See [the verification scope](FEATURE_PARITY.md) and [project progress](CHECKLIST.md) for open gates.
