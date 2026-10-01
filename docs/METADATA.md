# Metadata

Metadata retrieves Lambda execution-environment information from the Lambda Metadata Service (LMDS). Import `github.com/rambow-cloud/powertools-lambda-go/commons/metadata`, an independent optional module. It is distinct from EC2 instance metadata and is not fetched automatically by Logger or Tracer.

## Complete example

Save this program in an empty directory inside the checkout and run `go run main.go` with `CGO_ENABLED=0`. The same default helper can be called with the invocation context inside a Lambda handler.

~~~go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	stdlog "log"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons/metadata"
)

func main() {
	value, err := metadata.GetMetadata(context.Background(),
		metadata.Options{Timeout: 500 * time.Millisecond})
	if err != nil {
		stdlog.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		stdlog.Fatal(err)
	}
	fmt.Println(string(encoded))
}
~~~

## Input and output

Outside Lambda, the program prints `{}` without sending a request. In a supported Lambda environment, it returns the endpoint's object, which can contain `AvailabilityZoneID`, for example `{"AvailabilityZoneID":"example-az1"}`. That value is illustrative; availability depends on the execution environment. Unknown response properties are retained. This helper returns data; it does not automatically write a structured log.

The default client reads `AWS_LAMBDA_METADATA_API` and `AWS_LAMBDA_METADATA_TOKEN` and sends a bearer-authenticated request to `/2026-01-15/metadata/execution-environment`. Do not print the token. Status, timeout, cancellation and decoding failures return an error rather than a successful metadata object.

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| Default helper | One lazily used process client; a nonempty successful result is cached across warm calls |
| `Client` | `metadata.New(Config)` creates an independent client with endpoint, token and optional HTTP client |
| `Options` / `ctx` | Per-call timeout bounded by the caller's cancellation/deadline; default timeout is one second |
| Returned map | Private snapshot; caller changes do not corrupt cached metadata |

Call `ClearMetadataCache()` to clear the default client, or `client.ClearCache()` for an explicit client. Failures and empty responses are not cached. Concurrent fetches on one client are coalesced; waiting callers can cancel, and a pre-clear request cannot refill the cleared cache. Explicit `Endpoint` enables local HTTP access outside Lambda. The client rejects redirects so authentication stays on the configured endpoint.

## TypeScript feature coverage

Compared with the [official v2.35.0 Metadata guide](https://github.com/aws-powertools/powertools-lambda-typescript/blob/7bcc27b1574493f9452688673658f52b80c53847/docs/features/metadata.md).

| TypeScript feature | Go API or approach | Compatibility scope |
| --- | --- | --- |
| Get execution-environment metadata | `GetMetadata(ctx, options...)` | Default environment endpoint/token and optional timeout |
| Available metadata | `map[string]any` | Retains `AvailabilityZoneID` and unknown fields; no assumed service availability |
| Local development | Empty default result outside Lambda | No automatic service request |
| Clear cache / testing | `ClearMetadataCache`, explicit `Client` and HTTP injection | Go adds isolated snapshots, coordinated fetches and redirect rejection |

[Metadata tests](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/commons/metadata/metadata_test.go) cover reference/local/HTTP/error/cache/concurrency behavior. Local Docker explicitly retrieves metadata from its authenticated fixture. It does not establish real LMDS availability, authentication or execution-environment semantics. See [feature comparison](FEATURE_PARITY.md), [Commons](COMMONS.md) and [remaining progress](CHECKLIST.md#commons-and-metadata).
