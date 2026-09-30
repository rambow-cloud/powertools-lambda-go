package parameters_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/appconfig"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/secrets"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/ssm"
)

func TestDefaultHelpersAndGlobalClear(t *testing.T) {
	counts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		if target == "" {
			target = r.URL.Path
		}
		counts[target]++
		w.Header().Set("Content-Type", "application/json")
		switch target {
		case "AmazonSSM.GetParameter":
			fmt.Fprint(w, `{"Parameter":{"Value":"value"}}`)
		case "AmazonSSM.GetParametersByPath":
			fmt.Fprint(w, `{"Parameters":[{"Name":"/path/key","Value":"value"}]}`)
		case "AmazonSSM.GetParameters":
			fmt.Fprint(w, `{"Parameters":[{"Name":"batch-key","Value":"value"}]}`)
		case "AmazonSSM.PutParameter":
			fmt.Fprint(w, `{"Version":1}`)
		case "secretsmanager.GetSecretValue":
			fmt.Fprint(w, `{"SecretString":"value"}`)
		case "/configurationsessions":
			var input map[string]any
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input["ApplicationIdentifier"] != "app" || input["EnvironmentIdentifier"] != "test" {
				t.Errorf("default AppConfig identity: %v", input)
			}
			fmt.Fprint(w, `{"InitialConfigurationToken":"first"}`)
		case "/configuration":
			w.Header().Set("Next-Poll-Configuration-Token", "next")
			fmt.Fprint(w, "value")
		default:
			t.Error("unexpected operation", target)
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	for key, value := range map[string]string{
		"AWS_REGION": "ap-east-1", "AWS_ACCESS_KEY_ID": "LOCALTEST", "AWS_SECRET_ACCESS_KEY": "local-test-only", "AWS_SESSION_TOKEN": "",
		"AWS_ENDPOINT_URL": server.URL, "AWS_EC2_METADATA_DISABLED": "true", "AWS_PROFILE": "", "AWS_CONFIG_FILE": t.TempDir() + "/absent", "AWS_SHARED_CREDENTIALS_FILE": t.TempDir() + "/absent",
	} {
		t.Setenv(key, value)
	}
	ctx := context.Background()
	for i := range 3 {
		if i == 2 {
			parameters.ClearCaches()
		}
		value, err := ssm.GetParameter(ctx, "key", ssm.GetOptions{})
		if err != nil || value != "value" {
			t.Fatalf("default SSM: %v %v", value, err)
		}
		value, err = secrets.GetSecret(ctx, "secret", secrets.GetOptions{})
		if err != nil || value != "value" {
			t.Fatalf("default secret: %v %v", value, err)
		}
		value, err = appconfig.GetAppConfig(ctx, "flags", appconfig.DefaultOptions{Application: "app", Environment: "test"})
		if err != nil || string(value.([]byte)) != "value" {
			t.Fatalf("default AppConfig: %v %v", value, err)
		}
	}
	for _, key := range []string{"AmazonSSM.GetParameter", "secretsmanager.GetSecretValue", "/configuration"} {
		if counts[key] != 2 {
			t.Errorf("%s: calls=%d", key, counts[key])
		}
	}
	if counts["/configurationsessions"] != 1 {
		t.Fatal("clear reset session")
	}
	values, err := ssm.GetParameters(ctx, "/path", ssm.MultipleOptions{})
	if err != nil || values["key"] != "value" {
		t.Fatalf("default path: %v %v", values, err)
	}
	values, err = ssm.GetParametersByName(ctx, map[string]ssm.GetOptions{"batch-key": {}}, ssm.ByNameOptions{})
	if err != nil || values["batch-key"] != "value" {
		t.Fatalf("default batch: %v %v", values, err)
	}
	version, err := ssm.SetParameter(ctx, "key", "value", nil)
	if err != nil || version != 1 {
		t.Fatalf("default write: %d %v", version, err)
	}
}
