---
description: "Sign Go HTTP requests using AWS Signature Version 4 with Powertools Signer, credentials and reusable signing transports."
---

# Signer

Add AWS Signature Version 4 authentication to HTTP requests, or use a client that signs before sending.

## Install

Use Go 1.27 or newer:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/signer@v1.2.0
```

## Sign a request without sending it

The example uses synthetic credentials and checks that the signed copy contains an Authorization header. It sends no HTTP request and prints no credential or signature values.

**Input**

```text
GET /orders → execute-api, region ap-east-1
```

**Code**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rambow-cloud/powertools-lambda-go/signer"
)

func main() {
	credentials := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "EXAMPLE", SecretAccessKey: "example-only"}, nil
	})
	s, err := signer.New(signer.Config{Service: "execute-api", Region: "ap-east-1", Credentials: credentials})
	if err != nil {
		log.Fatal(err)
	}
	request, err := http.NewRequest("GET", "https://example.execute-api.ap-east-1.amazonaws.com/orders", nil)
	if err != nil {
		log.Fatal(err)
	}
	signed, err := s.Sign(request)
	if err != nil {
		log.Fatal(err)
	}
	defer signed.Body.Close()
	fmt.Println(signed.Method, signed.URL.RequestURI())
	fmt.Println("Authorization:", strings.Split(signed.Header.Get("Authorization"), " ")[0])
	fmt.Println("Signed date:", signed.Header.Get("X-Amz-Date") != "")
}
```

**Output**

```text
GET /orders
Authorization: AWS4-HMAC-SHA256
Signed date: true
```

## Send requests with a signing client

```go
client := signer.HTTPClient(s, &http.Client{Timeout: 5 * time.Second})
response, err := client.Get("https://example.execute-api.ap-east-1.amazonaws.com/orders")
```

Here `s` is your configured signer. Use real credentials with permission for the target service. Unlike `Sign`, the HTTP client sends the request; close the response body after reading it.

## Choose a service and credentials

| Target | Service name |
| --- | --- |
| API Gateway IAM authentication | `execute-api` |
| Lambda Function URL with IAM authentication | `lambda` |
| AppSync IAM authentication | `appsync` |

Supply an `aws.CredentialsProvider` for profiles, roles or cached credentials. Without one, Signer reads the standard AWS credential environment variables for each signing operation.

## Combine with tracing

```go
client := signer.HTTPClient(s, tr.HTTPClient(baseClient))
```

AWS SDK clients already sign their own requests; do not add a second signing layer to them.

## Things to know

- `Sign` returns a signed copy; close bodies as described in the API notes. Input streams without `GetBody` may be buffered.
- Redirects are disabled by default. Allow only trusted destinations with an explicit `CheckRedirect` policy.
- Signing is not proof of service-side authorization. IAM and endpoint configuration still apply.

## Next steps

See the [API and behavior notes](reference/SIGNER.md) for advanced options, and [module installation](MODULES.md) for dependencies.
