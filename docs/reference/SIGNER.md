# Signer: API and behavior notes

Start with the [Signer guide](../SIGNER.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

## Configuration and errors

`Region` defaults to the raw `AWS_REGION` value. Service and resolved region must be nonempty. The default provider reads `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` for each signing operation, allowing environment credential refresh. It makes no credential-discovery network requests. Supply any `aws.CredentialsProvider` for static credentials, role assumption, profiles, or caching; callers own that provider and its I/O. `aws.CredentialsProviderFunc` and `aws.NewCredentialsCache` can be used directly.

`Clock` supports deterministic signing tests. The constructor returns `*ConfigError` for missing configuration, and the default provider returns it for absent credential variables. Injected provider errors propagate unchanged. Body/request/signing failures use `*SigningError` with `Unwrap`; context cancellation remains discoverable with `errors.Is`.


## Ownership, redirects, and composition

Native header map names are canonicalized before signing. Case-colliding entries
join in sorted raw-name order; stale authentication/date/token headers are removed
regardless of spelling. Explicit payload hashes use the same folded lookup.
Transport gives the signer an owned request clone, preserving the caller's URL,
headers and Body field while downstream upload and closure retain body ownership.

`Sign(*http.Request)` signs without sending. It clones the URL and headers and returns a replayable body with `GetBody`. If the input supplies `GetBody`, its body stream is untouched. Otherwise the finite stream is buffered and restored on the original request, preserving its `Close`. After a read failure the successfully read prefix is restored in front of the unread stream. Standalone callers close both bodies and must not use the input concurrently with signing. Buffering is proportional to payload size; context checks cannot forcibly interrupt an arbitrary blocking application Reader.

`Transport` accepts any `Signer` interface and any `http.RoundTripper`. It closes the incoming body if signing fails. After handoff, the downstream transport owns the signed body's lifetime; closing it also releases the incoming body. Custom signers may return the input request or a clone sharing its body. The base transport can finish reading and closing that body after `RoundTrip` returns, including error paths, as permitted by [Go's transport contract](https://pkg.go.dev/net/http#RoundTripper). Empty signed bodies retain Go's empty-body framing and release the incoming body immediately. It performs no retry loop. `HTTPClient` copies its supplied client and defaults to stopping at redirects. Supply a deliberate `CheckRedirect` policy to allow trusted destinations; each permitted redirect is signed again, and normal Go replay rules apply to 307/308 responses.

Compose with tracing as `signer.HTTPClient(s, tr.HTTPClient(baseClient))`. Neither utility imports the other. AWS SDK service clients already sign their own requests and must not receive another signing layer.


## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| `s` | Reusable service, region, clock and credentials-provider configuration. |
| `request` / `signed` | `Sign` returns a signed copy; the caller owns body closure and request replay rules. |
| `HTTPClient` | Copied client with a signing transport; credentials, retries and redirects stay explicit. |

