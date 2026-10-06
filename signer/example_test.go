package signer_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rambow-cloud/powertools-lambda-go/signer"
)

func ExampleSigV4Signer_Sign() {
	// Synthetic credentials are only for demonstrating local signing.
	// Production callers supply their application-owned credentials provider.
	s, err := signer.New(signer.Config{
		Service: "execute-api", Region: "ap-east-1",
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "EXAMPLE", SecretAccessKey: "example-secret"}, nil
		}),
	})
	if err != nil {
		panic(err)
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.com/orders", nil)
	if err != nil {
		panic(err)
	}
	signed, err := s.Sign(request)
	if err != nil {
		panic(err)
	}
	if signed.Body != nil {
		defer signed.Body.Close()
	}
	fmt.Println(signed.Header.Get("Authorization") != "")
	fmt.Println(request.Header.Get("Authorization") == "")
	// Output:
	// true
	// true
}
