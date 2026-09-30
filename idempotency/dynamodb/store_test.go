package dynamodb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) Do(r *http.Request) (*http.Response, error) { return f(r) }
func client(t *testing.T, send transport) *sdk.Client {
	t.Helper()
	return sdk.New(sdk.Options{Region: "ap-east-1", BaseEndpoint: aws.String("https://dynamodb.local"), Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	}), HTTPClient: send, RetryMaxAttempts: 1})
}
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": []string{"application/x-amz-json-1.0"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestReferenceDynamoDBCommands(t *testing.T) {
	data, err := os.ReadFile("../testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Wire []struct {
			Composite bool
			Commands  []struct {
				Type  string
				Input map[string]any
			}
		}
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Wire {
		t.Run(map[bool]string{true: "composite", false: "simple"}[fixture.Composite], func(t *testing.T) {
			index := 0
			sdkClient := client(t, func(r *http.Request) (*http.Response, error) {
				if index >= len(fixture.Commands) {
					t.Fatal("unexpected request")
				}
				want := fixture.Commands[index]
				index++
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want.Input) {
					actual, _ := json.Marshal(got)
					expected, _ := json.Marshal(want.Input)
					t.Fatalf("request mismatch\ngot: %s\nwant: %s", actual, expected)
				}
				if !strings.HasSuffix(r.Header.Get("X-Amz-Target"), strings.TrimSuffix(want.Type, "Command")) {
					t.Fatal(r.Header)
				}
				if strings.Count(r.Header.Get("User-Agent"), "PT/idempotency/") != 1 {
					t.Fatal("missing or duplicated SDK identity")
				}
				return response(200, `{}`), nil
			})
			o := Options{TableName: "records"}
			if fixture.Composite {
				o = Options{TableName: "records", KeyAttribute: "pk", SortKeyAttribute: "sk", StaticPartitionKey: "tenant", StatusAttribute: "s", ExpiryAttribute: "e", InProgressExpiryAttribute: "ip", DataAttribute: "d", ValidationAttribute: "v"}
			}
			store, err := New(sdkClient, o)
			if err != nil {
				t.Fatal(err)
			}
			now := time.UnixMilli(1800000000250)
			m, err := idempotency.New(store, idempotency.Options{KeyPrefix: "operation", EventKeyJMESPath: "id", PayloadValidationJMESPath: "amount", Now: func() time.Time { return now }, Diagnostic: func(string) {}})
			if err != nil {
				t.Fatal(err)
			}
			key, validation, _, err := m.Key(map[string]any{"id": "a", "amount": 7})
			if err != nil {
				t.Fatal(err)
			}
			r := idempotency.Record{Key: key, Status: idempotency.InProgress, Expiration: 1800003600, InProgressExpiration: 1800000005250, Validation: validation}
			if err = store.Put(context.Background(), r, now); err != nil {
				t.Fatal(err)
			}
			r.Status = idempotency.Completed
			r.InProgressExpiration = 0
			r.Data = json.RawMessage(`{"ok":true,"amount":1.5,"nested":[null,3]}`)
			if err = store.Update(context.Background(), r); err != nil {
				t.Fatal(err)
			}
			if err = store.Delete(context.Background(), key); err != nil {
				t.Fatal(err)
			}
			if index != 3 {
				t.Fatalf("requests=%d", index)
			}
		})
	}
}

func TestConsistentReadsAndConditionalConflict(t *testing.T) {
	item := `{"id":{"S":"key"},"status":{"S":"COMPLETED"},"expiration":{"N":"1800003600"},"validation":{"S":"hash"},"data":{"M":{"integer":{"N":"9007199254740993"},"items":{"L":[{"N":"1.5"},{"NULL":true}]}}}}`
	for _, operation := range []string{"get", "conflict", "not-found", "no-old-item", "malformed", "service-error"} {
		t.Run(operation, func(t *testing.T) {
			sdkClient := client(t, func(r *http.Request) (*http.Response, error) {
				var input map[string]any
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Fatal(err)
				}
				if operation == "get" && input["ConsistentRead"] != true {
					t.Fatal("read must be strongly consistent")
				}
				switch operation {
				case "get":
					return response(200, `{"Item":`+item+`}`), nil
				case "conflict":
					return response(400, `{"__type":"ConditionalCheckFailedException","Item":`+item+`}`), nil
				case "not-found":
					return response(200, `{}`), nil
				case "no-old-item":
					return response(400, `{"__type":"ConditionalCheckFailedException"}`), nil
				case "malformed":
					return response(200, `{"Item":{"status":{"N":"1"}}}`), nil
				default:
					return response(400, `{"__type":"AccessDeniedException","message":"denied"}`), nil
				}
			})
			store, _ := New(sdkClient, Options{TableName: "records"})
			if operation == "conflict" || operation == "no-old-item" {
				err := store.Put(context.Background(), idempotency.Record{Key: "key", Status: idempotency.InProgress, Expiration: 1800003600}, time.Unix(1800000000, 0))
				var conflict *idempotency.AlreadyExistsError
				if !errors.As(err, &conflict) {
					t.Fatal(err)
				}
				if operation == "conflict" {
					if conflict.Record == nil || conflict.Record.Validation != "hash" {
						t.Fatal(conflict)
					}
				} else if conflict.Record != nil {
					t.Fatal("unexpected old item")
				}
				return
			}
			r, err := store.Get(context.Background(), "key")
			switch operation {
			case "get":
				if err != nil {
					t.Fatal(err)
				}
				if string(r.Data) != `{"integer":9007199254740993,"items":[1.5,null]}` || r.Key != "key" || r.Status != idempotency.Completed {
					t.Fatalf("%+v %s", r, r.Data)
				}
			case "not-found":
				if !errors.Is(err, idempotency.ErrNotFound) {
					t.Fatal(err)
				}
			default:
				if err == nil {
					t.Fatal("expected error")
				}
			}
		})
	}
}

func TestConfigurationAndCompositeReads(t *testing.T) {
	sdkClient := client(t, func(r *http.Request) (*http.Response, error) {
		var input struct{ Key map[string]map[string]string }
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.Key["pk"]["S"] != "tenant" || input.Key["sk"]["S"] != "logical" {
			t.Fatal(input)
		}
		return response(200, `{"Item":{"pk":{"S":"tenant"},"sk":{"S":"logical"},"status":{"S":"COMPLETED"},"data":{"NULL":true}}}`), nil
	})
	store, err := New(sdkClient, Options{TableName: "records", KeyAttribute: "pk", SortKeyAttribute: "sk", StaticPartitionKey: "tenant"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.Get(context.Background(), "logical")
	if err != nil || r.Key != "logical" || string(r.Data) != "null" {
		t.Fatalf("%+v %v", r, err)
	}
	for _, options := range []Options{{}, {TableName: "records", KeyAttribute: "same", SortKeyAttribute: "same"}, {TableName: "records", DataAttribute: "status"}} {
		if _, err := New(sdkClient, options); err == nil {
			t.Fatalf("accepted %+v", options)
		}
	}
}
