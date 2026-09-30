// Command signing demonstrates standalone signatures with synthetic credentials.
// It sends no requests and prints no credentials or signature values.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rambow-cloud/powertools-lambda-go/signer"
)

func main() {
	credentials := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "EXAMPLE", SecretAccessKey: "example-only"}, nil
	})
	for _, target := range []struct{ service, address string }{
		{"execute-api", "https://example.execute-api.ap-east-1.amazonaws.com/orders"},
		{"lambda", "https://example.lambda-url.ap-east-1.on.aws/"},
		{"appsync", "https://example.appsync-api.ap-east-1.amazonaws.com/graphql"},
	} {
		s, err := signer.New(signer.Config{Service: target.service, Region: "ap-east-1", Credentials: credentials})
		if err != nil {
			log.Fatal(err)
		}
		request, err := http.NewRequest(http.MethodGet, target.address, nil)
		if err != nil {
			log.Fatal(err)
		}
		signed, err := s.Sign(request)
		if err != nil {
			log.Fatal(err)
		}
		_ = signed.Body.Close()
		fmt.Println(target.service, signed.Header.Get("Authorization") != "")
	}
}
