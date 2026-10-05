package parameters_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	appconfigsdk "github.com/aws/aws-sdk-go-v2/service/appconfigdata"
	appconfigtypes "github.com/aws/aws-sdk-go-v2/service/appconfigdata/types"
	ddbsdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	secretssdk "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	secretstypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	ssmsdk "github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/appconfig"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/secrets"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/ssm"
)

type serviceStep struct {
	Operation, Method, Path, Target, Body string
	Request                               map[string]any
	Query                                 url.Values
	Status                                int
	Headers                               map[string]string
}
type serviceScenario struct {
	Name, Service, Provenance string
	SDKVersion                string   `json:"sdk_version"`
	SourceURLs                []string `json:"source_urls"`
	Steps                     []serviceStep
}

// Exact request matching and consumed steps detect wrong selectors, tokens and retries.
func scenarioServer(t *testing.T, steps []serviceStep) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if index >= len(steps) {
			t.Error("unexpected extra SDK request")
			http.Error(w, "unexpected request", 400)
			return
		}
		want := steps[index]
		index++
		var body map[string]any
		data, err := io.ReadAll(r.Body)
		if err == nil && len(data) > 0 {
			err = json.Unmarshal(data, &body)
		}
		if err != nil || r.Method != want.Method || r.URL.Path != want.Path || r.Header.Get("X-Amz-Target") != want.Target || !reflect.DeepEqual(r.URL.Query(), want.Query) || !reflect.DeepEqual(body, want.Request) {
			t.Errorf("%s step %d: unexpected %s %s target=%q body=%s error=%v", want.Operation, index, r.Method, r.URL, r.Header.Get("X-Amz-Target"), data, err)
			http.Error(w, "unexpected request", 400)
			return
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("SDK did not sign request")
		}
		for key, value := range want.Headers {
			w.Header().Set(key, value)
		}
		w.WriteHeader(want.Status)
		_, _ = io.WriteString(w, want.Body)
	}))
	t.Cleanup(func() {
		server.Close()
		mu.Lock()
		defer mu.Unlock()
		if index != len(steps) {
			t.Errorf("consumed %d/%d HTTP response steps", index, len(steps))
		}
	})
	return server
}

func executeServiceScenario(t *testing.T, ctx context.Context, cfg aws.Config, item serviceScenario) (any, error) {
	t.Helper()
	switch item.Service {
	case "ssm":
		p := ssm.New(ssmsdk.NewFromConfig(cfg))
		if item.Name == "ssm-pages" {
			return p.GetMultiple(ctx, "/app", ssm.MultipleOptions{Options: parameters.Options{Transform: parameters.JSON}, Recursive: aws.Bool(true), Decrypt: aws.Bool(true)})
		}
		return p.Get(ctx, "config", ssm.GetOptions{Options: parameters.Options{Transform: parameters.JSON}, Decrypt: aws.Bool(true)})
	case "secrets":
		p := secrets.New(secretssdk.NewFromConfig(cfg))
		return p.Get(ctx, "config", secrets.GetOptions{SDKOptions: &secretssdk.GetSecretValueInput{VersionStage: aws.String("AWSPREVIOUS")}})
	case "dynamodb":
		p, err := dynamodb.New(ddbsdk.NewFromConfig(cfg), dynamodb.Config{TableName: "settings", KeyAttribute: "pk", SortAttribute: "name", ValueAttribute: "data"})
		if err != nil {
			t.Fatal(err)
		}
		if item.Name == "dynamodb-pages" {
			return p.GetMultiple(ctx, "group", dynamodb.MultipleOptions{Options: parameters.Options{Transform: parameters.Auto}, SDKOptions: &ddbsdk.QueryInput{Limit: aws.Int32(1)}})
		}
		return p.Get(ctx, "config", dynamodb.GetOptions{SDKOptions: &ddbsdk.GetItemInput{ConsistentRead: aws.Bool(true)}})
	case "appconfig":
		p, err := appconfig.New(appconfigsdk.NewFromConfig(cfg), appconfig.Config{Application: "orders", Environment: "test"})
		if err != nil {
			t.Fatal(err)
		}
		opts := appconfig.GetOptions{Options: parameters.Options{Transform: parameters.JSON, ForceFetch: true}, SDKOptions: &appconfigsdk.StartConfigurationSessionInput{RequiredMinimumPollIntervalInSeconds: aws.Int32(15)}}
		value, err := p.Get(ctx, "flags", opts)
		if item.Name != "appconfig-unchanged" {
			return value, err
		}
		if err != nil || !reflect.DeepEqual(value, map[string]any{"enabled": true}) {
			t.Fatalf("initial configuration: %v/%v", value, err)
		}
		return p.Get(ctx, "flags", opts)
	default:
		t.Fatalf("unknown service %q", item.Service)
		return nil, nil
	}
}

func TestSourcedServiceHTTPScenarios(t *testing.T) {
	data, err := os.ReadFile("testdata/aws-http-scenarios.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []serviceScenario }
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 13 {
		t.Fatalf("unexpected scenario inventory: %d", len(corpus.Cases))
	}
	for _, item := range corpus.Cases {
		t.Run(item.Name, func(t *testing.T) {
			if item.Provenance != "synthetic" || len(item.SourceURLs) == 0 || item.SDKVersion == "" || len(item.Steps) == 0 {
				t.Fatal("missing fixture provenance")
			}
			server := scenarioServer(t, item.Steps)
			cfg := sdkConfig(server.URL)
			cfg.RetryMaxAttempts = 3
			cfg.Retryer = func() aws.Retryer {
				return retry.NewStandard(func(o *retry.StandardOptions) {
					o.MaxAttempts = 3
					o.Backoff = retry.BackoffDelayerFunc(func(int, error) (time.Duration, error) { return 0, nil })
				})
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := executeServiceScenario(t, ctx, cfg, item)
			// Provider expectations are maintained independently of HTTP response samples.
			codes := map[string]string{"ssm-retry-exhausted": "InternalServerError", "ssm-missing": "ParameterNotFound", "secrets-missing": "ResourceNotFoundException", "dynamodb-missing-table": "ResourceNotFoundException", "appconfig-error": "BadRequestException"}
			if code := codes[item.Name]; code != "" {
				var api smithy.APIError
				var wrapped *parameters.GetParameterError
				if !errors.As(err, &api) || api.ErrorCode() != code || !errors.As(err, &wrapped) {
					t.Fatalf("SDK service error lost: %v", err)
				}
				switch item.Name {
				case "ssm-missing":
					var missing *ssmtypes.ParameterNotFound
					if !errors.As(err, &missing) {
						t.Fatalf("SDK did not decode typed missing: %v", err)
					}
				case "ssm-retry-exhausted":
					var failure *ssmtypes.InternalServerError
					if !errors.As(err, &failure) {
						t.Fatalf("untyped SSM error: %v", err)
					}
				case "secrets-missing":
					var failure *secretstypes.ResourceNotFoundException
					if !errors.As(err, &failure) {
						t.Fatalf("untyped Secrets error: %v", err)
					}
				case "dynamodb-missing-table":
					var failure *ddbtypes.ResourceNotFoundException
					if !errors.As(err, &failure) {
						t.Fatalf("untyped DynamoDB error: %v", err)
					}
				case "appconfig-error":
					var failure *appconfigtypes.BadRequestException
					if !errors.As(err, &failure) {
						t.Fatalf("untyped AppConfig error: %v", err)
					}
				}
				return
			}
			if item.Name == "secrets-malformed-binary" {
				var decode *smithy.DeserializationError
				if !errors.As(err, &decode) {
					t.Fatalf("malformed wire binary accepted: %v", err)
				}
				return
			}
			wanted := map[string]any{
				"ssm-json": map[string]any{"enabled": true}, "ssm-retry-success": map[string]any{"enabled": true},
				"ssm-pages": map[string]any{"a.json": true, "b.json": float64(2)}, "secrets-binary": []byte("hello"),
				"dynamodb-native": map[string]any{"count": float64(2), "enabled": true}, "dynamodb-pages": map[string]any{"a.json": true, "b.binary": "hello"},
				"appconfig-unchanged": map[string]any{"enabled": true},
			}
			want, exists := wanted[item.Name]
			if !exists || err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("provider result=%#v/%v want=%#v", got, err, want)
			}
		})
	}
}

func TestSDKHTTPCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	p := ssm.New(ssmsdk.NewFromConfig(sdkConfig(server.URL)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := p.Get(ctx, "config", ssm.GetOptions{}); finished <- err }()
	select {
	case <-entered:
		cancel()
	case <-ctx.Done():
		t.Fatal("SDK did not reach local HTTP server")
	}
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("HTTP cancellation lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SDK did not stop after cancellation")
	}
}
