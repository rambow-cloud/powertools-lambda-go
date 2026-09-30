package parameters_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddbsdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	secretssdk "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	ssmsdk "github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/secrets"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/ssm"
)

// These tests execute real SDK serialization, signing, pagination, and error decoding.
func sdkConfig(endpoint string) aws.Config {
	return aws.Config{Region: "ap-east-1", BaseEndpoint: aws.String(endpoint), RetryMaxAttempts: 1, Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "LOCALTEST", SecretAccessKey: "local-test-only"}, nil
	})}
}

func TestSSMProtocolAndBatch(t *testing.T) {
	var single, batch, pages, writes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing SDK signature")
		}
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		switch r.Header.Get("X-Amz-Target") {
		case "AmazonSSM.GetParameter":
			single++
			if input["Name"] == "missing" {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"__type":"ParameterNotFound","message":"not found"}`)
				return
			}
			if input["WithDecryption"] != true {
				t.Error("decryption precedence")
			}
			fmt.Fprint(w, `{"Parameter":{"Name":"config","Value":"{\"enabled\":true}"}}`)
		case "AmazonSSM.GetParametersByPath":
			pages++
			if input["Path"] != "/app" || input["Recursive"] != true || input["WithDecryption"] != false {
				t.Errorf("path input: %v", input)
			}
			if input["NextToken"] == nil {
				fmt.Fprint(w, `{"Parameters":[{"Name":"/app/first.json","Value":"1"}],"NextToken":"next"}`)
			} else {
				fmt.Fprint(w, `{"Parameters":[{"Name":"/app/nested/second.json","Value":"2"}]}`)
			}
		case "AmazonSSM.GetParameters":
			batch++
			names := input["Names"].([]any)
			if len(names) > 10 {
				t.Error("batch exceeds ten")
			}
			values := []map[string]string{}
			invalid := []string{}
			for _, name := range names {
				if name == "missing" {
					invalid = append(invalid, name.(string))
				} else {
					values = append(values, map[string]string{"Name": name.(string), "Value": "true"})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"Parameters": values, "InvalidParameters": invalid})
		case "AmazonSSM.PutParameter":
			writes++
			if input["Name"] != "config" || input["Value"] != "new" || input["Type"] != "String" || input["Tier"] != "Standard" || input["Overwrite"] != false {
				t.Errorf("write defaults: %v", input)
			}
			fmt.Fprint(w, `{"Version":7}`)
		default:
			t.Errorf("unexpected target %q", r.Header.Get("X-Amz-Target"))
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	p := ssm.New(ssmsdk.NewFromConfig(sdkConfig(server.URL)))
	ctx := context.Background()
	t.Setenv("POWERTOOLS_PARAMETERS_SSM_DECRYPT", "on")
	sdkOptions := &ssmsdk.GetParameterInput{Name: aws.String("ignored"), WithDecryption: aws.Bool(false)}
	opts := ssm.GetOptions{Options: parameters.Options{Transform: parameters.JSON}, Decrypt: aws.Bool(true), SDKOptions: sdkOptions}
	for range 2 {
		value, err := p.Get(ctx, "config", opts)
		if err != nil || !reflect.DeepEqual(value, map[string]any{"enabled": true}) {
			t.Fatalf("get: %v %v", value, err)
		}
	}
	if single != 1 || *sdkOptions.Name != "ignored" || *sdkOptions.WithDecryption {
		t.Fatal("cache or input mutation")
	}
	values, err := p.GetMultiple(ctx, "/app", ssm.MultipleOptions{Options: parameters.Options{Transform: parameters.Auto}, Recursive: aws.Bool(true), Decrypt: aws.Bool(false)})
	if err != nil || pages != 2 || !reflect.DeepEqual(values, map[string]any{"first.json": float64(1), "nested/second.json": float64(2)}) {
		t.Fatalf("pagination: %v %v, pages=%d", values, err, pages)
	}
	names := make(map[string]ssm.GetOptions)
	for i := range 21 {
		names[fmt.Sprintf("name-%02d", i)] = ssm.GetOptions{}
	}
	values, err = p.GetParametersByName(ctx, names, ssm.ByNameOptions{Options: parameters.Options{Transform: parameters.JSON}, Decrypt: aws.Bool(false)})
	if err != nil || len(values) != 21 || batch != 3 {
		t.Fatalf("batch: %v %v calls=%d", values, err, batch)
	}
	_, err = p.GetParametersByName(ctx, names, ssm.ByNameOptions{Options: parameters.Options{Transform: parameters.JSON}, Decrypt: aws.Bool(false)})
	if err != nil || batch != 3 {
		t.Fatal("batch cache missed")
	}
	if names["name-00"].Transform != "" || names["name-00"].MaxAge != nil {
		t.Fatal("caller options mutated")
	}
	values, err = p.GetParametersByName(ctx, map[string]ssm.GetOptions{"missing": {}, "ok": {}}, ssm.ByNameOptions{Decrypt: aws.Bool(false), ThrowOnError: aws.Bool(false)})
	if err != nil || !reflect.DeepEqual(values["_errors"], []string{"missing"}) || values["ok"] != "true" {
		t.Fatalf("partial errors: %v %v", values, err)
	}
	_, err = p.GetParametersByName(ctx, map[string]ssm.GetOptions{"missing": {}}, ssm.ByNameOptions{})
	if err == nil {
		t.Fatal("strict invalid parameters accepted")
	}
	before := batch
	_, err = p.GetParametersByName(ctx, map[string]ssm.GetOptions{"_errors": {}}, ssm.ByNameOptions{ThrowOnError: aws.Bool(false)})
	if err == nil || batch != before {
		t.Fatal("reserved name not rejected before I/O")
	}
	_, err = p.Get(ctx, "missing", ssm.GetOptions{Options: parameters.Options{ThrowOnMissing: true}})
	var missing *parameters.ParameterNotFoundError
	if !errors.As(err, &missing) {
		t.Fatalf("typed missing: %v", err)
	}
	_, err = p.Get(ctx, "missing", ssm.GetOptions{})
	var native *ssmtypes.ParameterNotFound
	var get *parameters.GetParameterError
	if !errors.As(err, &get) || !errors.As(err, &native) {
		t.Fatalf("native cause: %v", err)
	}
	version, err := p.Set(ctx, "config", "new", nil)
	if err != nil || version != 7 || writes != 1 {
		t.Fatalf("set: %d %v", version, err)
	}
	before = single
	_, _ = p.Get(ctx, "config", opts)
	if single != before {
		t.Fatal("set unexpectedly invalidated cache")
	}
	p.ClearCache()
	_, _ = p.Get(ctx, "config", opts)
	if single != before+1 {
		t.Fatal("clear did not invalidate cache")
	}
}

func TestSSMBatchReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Batches  []map[string]any
		SDKCalls []any
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	var calls []any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input map[string]any
		_ = json.NewDecoder(r.Body).Decode(&input)
		operation := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "AmazonSSM.")
		// The SDK omits WithDecryption for unencrypted reference batches.
		if input["WithDecryption"] == false {
			delete(input, "WithDecryption")
		}
		calls = append(calls, map[string]any{"operation": operation + "Command", "input": input})
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if operation == "GetParameter" {
			fmt.Fprint(w, `{"Parameter":{"Value":"{\"secret\":true}"}}`)
			return
		}
		values := []map[string]string{}
		invalid := []string{}
		for _, name := range input["Names"].([]any) {
			if name == "missing" {
				invalid = append(invalid, "missing")
				continue
			}
			value := `{"enabled":true}`
			if name == "empty" {
				value = ""
			}
			values = append(values, map[string]string{"Name": name.(string), "Value": value})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Parameters": values, "InvalidParameters": invalid})
	}))
	defer server.Close()
	p := ssm.New(ssmsdk.NewFromConfig(sdkConfig(server.URL)))
	for i, names := range []map[string]ssm.GetOptions{
		{"plain": {}, "secret": {Decrypt: aws.Bool(true), Options: parameters.Options{Transform: parameters.JSON}}},
		{"plain": {}, "secret": {Decrypt: aws.Bool(true), Options: parameters.Options{Transform: parameters.JSON}}},
		{"empty": {}, "missing": {}},
	} {
		values, err := p.GetParametersByName(context.Background(), names, ssm.ByNameOptions{Options: parameters.Options{Transform: parameters.JSON}, ThrowOnError: aws.Bool(false)})
		encoded, _ := json.Marshal(values)
		var normalized map[string]any
		_ = json.Unmarshal(encoded, &normalized)
		if err != nil || !reflect.DeepEqual(normalized, fixture.Batches[i]) {
			t.Fatalf("batch %d: %v %v, want %v", i, values, err, fixture.Batches[i])
		}
	}
	if !reflect.DeepEqual(calls, fixture.SDKCalls) {
		t.Fatalf("SDK sequence: %#v; want %#v", calls, fixture.SDKCalls)
	}
}

func TestSecretsProtocol(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var input map[string]any
		_ = json.NewDecoder(r.Body).Decode(&input)
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		switch input["SecretId"] {
		case "json":
			if input["VersionStage"] != "AWSPREVIOUS" {
				t.Error("version option lost")
			}
			fmt.Fprint(w, `{"SecretString":"{\"password\":\"fixture\"}"}`)
		case "binary":
			fmt.Fprint(w, `{"SecretBinary":"aGVsbG8="}`)
		case "empty":
			fmt.Fprint(w, `{"SecretString":""}`)
		default:
			w.WriteHeader(400)
			fmt.Fprint(w, `{"__type":"ResourceNotFoundException","message":"missing"}`)
		}
	}))
	defer server.Close()
	p := secrets.New(secretssdk.NewFromConfig(sdkConfig(server.URL)))
	opts := secrets.GetOptions{Options: parameters.Options{Transform: parameters.JSON}, SDKOptions: &secretssdk.GetSecretValueInput{VersionStage: aws.String("AWSPREVIOUS")}}
	for range 2 {
		value, err := p.Get(context.Background(), "json", opts)
		if err != nil || !reflect.DeepEqual(value, map[string]any{"password": "fixture"}) {
			t.Fatalf("secret: %v %v", value, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("secret not cached")
	}
	value, err := p.Get(context.Background(), "binary", secrets.GetOptions{})
	if err != nil || string(value.([]byte)) != "hello" {
		t.Fatalf("raw binary: %v %v", value, err)
	}
	value, err = p.Get(context.Background(), "empty", secrets.GetOptions{})
	if err != nil || value != nil {
		t.Fatalf("empty string fallback: %v %v", value, err)
	}
	_, err = p.Get(context.Background(), "missing", secrets.GetOptions{Options: parameters.Options{ThrowOnMissing: true}})
	var missing *parameters.ParameterNotFoundError
	if !errors.As(err, &missing) {
		t.Fatal(err)
	}
}

func TestDynamoDBProtocol(t *testing.T) {
	var pages, singles int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input map[string]any
		_ = json.NewDecoder(r.Body).Decode(&input)
		if input["TableName"] != "settings" {
			t.Error("table option")
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		switch r.Header.Get("X-Amz-Target") {
		case "DynamoDB_20120810.GetItem":
			singles++
			if input["ProjectionExpression"] != "#value" || input["ConsistentRead"] != true {
				t.Error("projection or SDK option")
			}
			fmt.Fprint(w, `{"Item":{"data":{"M":{"count":{"N":"2"},"enabled":{"BOOL":true},"names":{"SS":["a","b"]}}}}}`)
		case "DynamoDB_20120810.Query":
			pages++
			if input["KeyConditionExpression"] != "#key = :key" || input["Limit"] != float64(1) || input["ExpressionAttributeNames"].(map[string]any)["#sk"] != "name" {
				t.Errorf("query: %v", input)
			}
			if input["ExclusiveStartKey"] == nil {
				fmt.Fprint(w, `{"Items":[{"name":{"S":"a.json"},"data":{"S":"true"}}],"LastEvaluatedKey":{"pk":{"S":"group"},"name":{"S":"a.json"}}}`)
			} else {
				fmt.Fprint(w, `{"Items":[{"name":{"S":"b.binary"},"data":{"S":"aGVsbG8="}}]}`)
			}
		default:
			t.Error("unexpected SDK operation")
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	p, err := dynamodb.New(ddbsdk.NewFromConfig(sdkConfig(server.URL)), dynamodb.Config{TableName: "settings", KeyAttribute: "pk", SortAttribute: "name", ValueAttribute: "data"})
	if err != nil {
		t.Fatal(err)
	}
	opts := dynamodb.GetOptions{Options: parameters.Options{Transform: parameters.JSON}, SDKOptions: &ddbsdk.GetItemInput{ConsistentRead: aws.Bool(true)}}
	value, err := p.Get(context.Background(), "key", opts)
	if err != nil || value.(map[string]any)["count"] != float64(2) {
		t.Fatalf("native value: %v %v", value, err)
	}
	value.(map[string]any)["names"].([]string)[0] = "changed"
	value, err = p.Get(context.Background(), "key", opts)
	if err != nil || singles != 1 || value.(map[string]any)["names"].([]string)[0] != "a" {
		t.Fatal("DynamoDB set snapshot failed")
	}
	values, err := p.GetMultiple(context.Background(), "group", dynamodb.MultipleOptions{Options: parameters.Options{Transform: parameters.Auto}, SDKOptions: &ddbsdk.QueryInput{Limit: aws.Int32(1)}})
	if err != nil || pages != 2 || !reflect.DeepEqual(values, map[string]any{"a.json": true, "b.binary": "hello"}) {
		t.Fatalf("query: %v %v pages=%d", values, err, pages)
	}
}
