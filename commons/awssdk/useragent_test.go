package awssdk_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go/middleware"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/commons/awssdk"
)

func TestUserAgentCompositionWithoutGlobalMutation(t *testing.T) {
	t.Setenv("AWS_SDK_UA_APP_ID", "application-owned")
	t.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_go")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.Header.Get("User-Agent")
		if strings.Count(value, "PT/") != 1 || !strings.Contains(value, "PT/tracer/"+commons.Version) || !strings.Contains(value, "PTEnv/AWS_Lambda_go") || !strings.Contains(value, "aws-sdk-go-v2/") {
			t.Errorf("user agent: %s", value)
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		fmt.Fprint(w, `{"Parameter":{"Value":"test"}}`)
	}))
	defer server.Close()
	client := ssm.New(ssm.Options{Region: "ap-east-1", BaseEndpoint: aws.String(server.URL), Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "LOCALTEST", SecretAccessKey: "test-only"}, nil
	}), APIOptions: []func(*middleware.Stack) error{awssdk.UserAgent("tracer"), awssdk.UserAgent("parameters")}})
	_, err := client.GetParameter(context.Background(), &ssm.GetParameterInput{Name: aws.String("test")})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("AWS_SDK_UA_APP_ID") != "application-owned" {
		t.Fatal("global user-agent configuration changed")
	}
}
