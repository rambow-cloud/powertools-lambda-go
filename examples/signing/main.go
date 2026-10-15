// Command signing demonstrates standalone signatures with synthetic credentials.
// It sends no requests and logs no credentials or signature values.
package main

import (
	"context"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/signer"
)

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	if err := run(appLog); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func run(appLog *logger.Logger) error {
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
			return err
		}
		request, err := http.NewRequest(http.MethodGet, target.address, nil)
		if err != nil {
			return err
		}
		signed, err := s.Sign(request)
		if err != nil {
			return err
		}
		_ = signed.Body.Close()
		if err := appLog.Info("Request signed", logger.Fields{"target_service": target.service, "signed": signed.Header.Get("Authorization") != ""}); err != nil {
			return err
		}
	}
	return nil
}
