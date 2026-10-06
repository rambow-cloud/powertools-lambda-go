// Package awssdk supplies opt-in Powertools user-agent middleware for AWS SDK v2.
//
// Append [UserAgent] to an SDK client's APIOptions to identify the utility making
// requests. The returned stack modifier adds request metadata without initializing
// an SDK client, loading credentials, or performing network operations.
//
// # Optional integration
//
// This is a separate module so the core Commons package stays dependency-free.
// Configure middleware when constructing clients, then reuse those clients across
// Lambda invocations. The application owns SDK credentials, retries and endpoints.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/COMMONS.md
package awssdk
