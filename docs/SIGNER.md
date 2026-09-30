# Signer

The independent `github.com/rambow-cloud/powertools-lambda-go/signer` module signs HTTP requests with AWS SDK for Go v2 SigV4. It does not depend on Commons, Logger, Tracer, AWS config loading, or the X-Ray SDK. All builds keep `CGO_ENABLED=0`.

```go
s, err := signer.New(signer.Config{Service: "execute-api"})
if err != nil {
    return err
}
client := signer.HTTPClient(s, &http.Client{Timeout: 5 * time.Second})
request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
if err != nil {
    return err
}
response, err := client.Do(request)
if err != nil {
    return err
}
defer response.Body.Close()
```

Use `Service: "execute-api"` for IAM API Gateway, `"lambda"` for IAM Lambda function URLs, and `"appsync"` for IAM AppSync requests. For S3 set `DisableURIPathEscaping: true`. The offline [signing example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/signing/main.go) demonstrates all three IAM endpoint families using synthetic credentials without sending requests.

## Configuration and errors

`Region` defaults to the raw `AWS_REGION` value. Service and resolved region must be nonempty. The default provider reads `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` for each signing operation, allowing environment credential refresh. It makes no credential-discovery network requests. Supply any `aws.CredentialsProvider` for static credentials, role assumption, profiles, or caching; callers own that provider and its I/O. `aws.CredentialsProviderFunc` and `aws.NewCredentialsCache` can be used directly.

`Clock` supports deterministic signing tests. The constructor returns `*ConfigError` for missing configuration, and the default provider returns it for absent credential variables. Injected provider errors propagate unchanged. Body/request/signing failures use `*SigningError` with `Unwrap`; context cancellation remains discoverable with `errors.Is`.

## Ownership, redirects, and composition

`Sign(*http.Request)` signs without sending. It clones the URL and headers and returns a replayable body with `GetBody`. If the input supplies `GetBody`, its body stream is untouched. Otherwise the finite stream is buffered and restored on the original request, preserving its `Close`. After a read failure the successfully read prefix is restored in front of the unread stream. Standalone callers close both bodies and must not use the input concurrently with signing. Buffering is proportional to payload size; context checks cannot forcibly interrupt an arbitrary blocking application Reader.

`Transport` accepts any `Signer` interface and any `http.RoundTripper`. It owns and closes the incoming body even if signing fails, while the downstream transport owns the signed body. It performs no retry loop. `HTTPClient` copies its supplied client and defaults to stopping at redirects. Supply a deliberate `CheckRedirect` policy to allow trusted destinations; each permitted redirect is signed again, and normal Go replay rules apply to 307/308 responses.

Compose with tracing as `signer.HTTPClient(s, tr.HTTPClient(baseClient))`. Neither utility imports the other. AWS SDK service clients already sign their own requests and must not receive another signing layer.

## Reference scope and intentional differences

Reference: TypeScript v2.35.0. Thirteen fixed-time cases cover GET/JSON/binary/empty bodies, escaped paths, ports, repeated/whitespace headers, session tokens, explicit unsigned payloads, encoded queries, and service names. Twelve signatures match exactly. The duplicate-query case records an intentional difference: the upstream request converter signs only the last occurrence while returning the original URL. Go signs every occurrence, preserving the original URL and a valid canonical query.

Other Go contracts are explicit: empty `Region` means environment fallback; missing service is rejected; malformed query encoding and inconsistent Content-Length are rejected. Redirects require an explicit policy. Signer uses Go request-body ownership, and Go transport-generated headers are not signed unless supplied explicitly. Custom Host values and service-specific URL/path normalization follow the Go SDK; these do not establish every web Request conversion edge case. General presigning and SigV4a are not part of the pinned Powertools Signer API and are not implemented here.

Tests cover body replay/errors, configuration refresh, cancellation, custom transports, redirects, and concurrent use. Local Docker verifies synthetic signatures and OTel composition. These checks do not establish live IAM authorization, service-side acceptance for every path, or performance budgets.

## Sources

- [Pinned Signer implementation](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/packages/signer/src/SigV4Signer.ts)
- [AWS SDK for Go v2 SigV4](https://github.com/aws/aws-sdk-go-v2/tree/v1.47.0/aws/signer/v4)
