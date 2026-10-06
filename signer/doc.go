// Package signer signs outbound HTTP requests with AWS Signature Version 4.
//
// [New] constructs a reusable [SigV4Signer] from a service, region and credentials
// provider in [Config]. [SigV4Signer.Sign] clones request headers and URL and
// produces a replayable body. It signs locally without sending the request.
//
// # Transport and body ownership
//
// [Transport] adapts a signer to net/http.RoundTripper, while [HTTPClient] copies
// an existing HTTP client and disables redirects by default. The downstream
// transport owns signed bodies after handoff. When signing manually, callers
// retain the input body and must close both input and signed bodies.
//
// Credentials are retrieved per request; use an AWS credentials cache explicitly
// when needed. Applications own retries, trusted redirect policy and endpoint
// selection. [ConfigError] and [SigningError] identify configuration and signing
// failures and preserve underlying errors where applicable.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/SIGNER.md
package signer
