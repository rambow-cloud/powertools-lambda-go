package appconfigagent_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/appconfigagent"
)

func TestAgentHTTPAndLocalFallback(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.EscapedPath() != "/applications/app%2Fname/environments/test/configurations/flags%2Fjson" {
			t.Errorf("escaped path: %s", r.URL.EscapedPath())
		}
		fmt.Fprint(w, `{"enabled":true}`)
	}))
	defer server.Close()
	opts := appconfigagent.Options{Endpoint: server.URL, Application: "app/name", Environment: "test", Transform: parameters.JSON}
	for range 2 {
		value, err := appconfigagent.GetConfig(context.Background(), "flags/json", opts)
		if err != nil || !reflect.DeepEqual(value, map[string]any{"enabled": true}) {
			t.Fatalf("agent: %v %v", value, err)
		}
	}
	if calls != 2 {
		t.Fatal("agent response cached by library")
	}
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "unknown")
	t.Setenv("POWERTOOLS_APPCONFIG_AGENT_RETURN_VALUE", `{"local":true}`)
	value, err := appconfigagent.GetConfig(context.Background(), "flags", appconfigagent.Options{Transform: parameters.JSON})
	if err != nil || !reflect.DeepEqual(value, map[string]any{"local": true}) {
		t.Fatalf("local fallback: %v %v", value, err)
	}
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "on-demand")
	t.Setenv("POWERTOOLS_DEV", "true")
	_, err = appconfigagent.GetConfig(context.Background(), "flags", appconfigagent.Options{Transform: parameters.JSON})
	if err != nil {
		t.Fatal("development mode did not use local value", err)
	}
}

func TestAgentErrorsAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/applications/app/environments/test/configurations/missing":
			w.WriteHeader(404)
		case "/applications/app/environments/test/configurations/slow":
			<-r.Context().Done()
		default:
			w.WriteHeader(500)
			fmt.Fprint(w, "sensitive-response")
		}
	}))
	defer server.Close()
	opts := appconfigagent.Options{Endpoint: server.URL, Application: "app", Environment: "test", ThrowOnMissing: true, Timeout: 20 * time.Millisecond}
	_, err := appconfigagent.GetConfig(context.Background(), "missing", opts)
	var missing *parameters.ParameterNotFoundError
	if !errors.As(err, &missing) {
		t.Fatal(err)
	}
	opts.ThrowOnMissing = false
	value, err := appconfigagent.GetConfig(context.Background(), "missing", opts)
	if err != nil || value != nil {
		t.Fatalf("missing: %v %v", value, err)
	}
	_, err = appconfigagent.GetConfig(context.Background(), "slow", opts)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout cause lost: %v", err)
	}
	_, err = appconfigagent.GetConfig(context.Background(), "error", opts)
	var get *parameters.GetParameterError
	if !errors.As(err, &get) || err.Error() != `unable to get parameter "error": AppConfig Agent returned HTTP 500` {
		t.Fatalf("status error: %v", err)
	}
}
