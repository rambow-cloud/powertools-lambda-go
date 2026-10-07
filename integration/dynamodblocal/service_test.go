package dynamodblocal_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	idempotent "github.com/rambow-cloud/powertools-lambda-go/idempotency/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	parameterddb "github.com/rambow-cloud/powertools-lambda-go/parameters/dynamodb"
)

type queryCounter struct {
	base  http.RoundTripper
	calls atomic.Int32
}

func (c *queryCounter) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Header.Get("X-Amz-Target") == "DynamoDB_20120810.Query" {
		c.calls.Add(1)
	}
	return c.base.RoundTrip(r)
}

func serviceClient(t *testing.T) (*sdk.Client, context.Context, *queryCounter) {
	t.Helper()
	endpoint := os.Getenv("POWERTOOLS_DYNAMODB_LOCAL_ENDPOINT")
	if endpoint == "" {
		t.Skip("run integration/local/dynamodb_run.py for real DynamoDB Local acceptance")
	}
	t.Setenv("POWERTOOLS_PARAMETERS_MAX_AGE", "60")
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		t.Fatalf("invalid local endpoint: %q", endpoint)
	}
	ip := net.ParseIP(u.Hostname())
	if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		t.Fatal("DynamoDB acceptance requires a loopback endpoint")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	t.Cleanup(transport.CloseIdleConnections)
	counter := &queryCounter{base: transport}
	client := sdk.New(sdk.Options{Region: "ap-east-1", BaseEndpoint: aws.String(endpoint), RetryMaxAttempts: 1,
		HTTPClient: &http.Client{Transport: counter, Timeout: 10 * time.Second},
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "LOCALTEST", SecretAccessKey: "local-test-only"}, nil
		}),
	})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	return client, ctx, counter
}

func table(t *testing.T, ctx context.Context, client *sdk.Client, partition, sort string) string {
	t.Helper()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := "powertools-local-" + hex.EncodeToString(suffix[:])
	input := &sdk.CreateTableInput{TableName: aws.String(name), BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{{AttributeName: aws.String(partition), AttributeType: types.ScalarAttributeTypeS}},
		KeySchema:            []types.KeySchemaElement{{AttributeName: aws.String(partition), KeyType: types.KeyTypeHash}},
	}
	if sort != "" {
		input.AttributeDefinitions = append(input.AttributeDefinitions, types.AttributeDefinition{AttributeName: aws.String(sort), AttributeType: types.ScalarAttributeTypeS})
		input.KeySchema = append(input.KeySchema, types.KeySchemaElement{AttributeName: aws.String(sort), KeyType: types.KeyTypeRange})
	}
	if _, err := client.CreateTable(ctx, input); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Test contexts are cancelled before cleanup; deletion has its own deadline.
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := client.DeleteTable(cleanup, &sdk.DeleteTableInput{TableName: aws.String(name)}); err != nil {
			t.Errorf("delete disposable table: %v", err)
		}
	})
	return name
}

func store(t *testing.T, client *sdk.Client, options idempotent.Options) *idempotent.Store {
	t.Helper()
	s, err := idempotent.New(client, options)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDynamoDBLocalConditionalClaimsAndReplay(t *testing.T) {
	client, ctx, _ := serviceClient(t)
	s := store(t, client, idempotent.Options{TableName: table(t, ctx, client, "id", "")})
	now := time.Unix(1800000000, 0)
	record := idempotency.Record{Key: "claim", Status: idempotency.InProgress, Expiration: now.Unix() + 3600, InProgressExpiration: now.UnixMilli() + 30000, Validation: "claim-validation"}
	start := make(chan struct{})
	results := make(chan error, 16)
	var workers sync.WaitGroup
	for range 16 {
		workers.Add(1)
		go func() { defer workers.Done(); <-start; results <- s.Put(ctx, record, now) }()
	}
	close(start)
	workers.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		if err == nil {
			winners++
			continue
		}
		var conflict *idempotency.AlreadyExistsError
		if !errors.As(err, &conflict) || conflict.Record == nil || conflict.Record.Key != "claim" || conflict.Record.Status != idempotency.InProgress || conflict.Record.Validation != "claim-validation" {
			t.Fatalf("conditional conflict lost prior item: %v", err)
		}
		conflicts++
	}
	if winners != 1 || conflicts != 15 {
		t.Fatalf("claim winners=%d conflicts=%d", winners, conflicts)
	}
	record.Status = idempotency.Completed
	record.InProgressExpiration = 0
	record.Data = json.RawMessage(`{"ok":true,"n":9007199254740993}`)
	if err := s.Update(ctx, record); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "claim")
	if err != nil || got.Status != idempotency.Completed || !sameJSON(got.Data, record.Data) {
		t.Fatalf("persisted completion: %+v/%v", got, err)
	}
	var conflict *idempotency.AlreadyExistsError
	if err := s.Put(ctx, record, now); !errors.As(err, &conflict) || conflict.Record == nil || !sameJSON(conflict.Record.Data, got.Data) {
		t.Fatalf("completed prior item: %v", err)
	}
	newManager := func() *idempotency.Manager {
		m, err := idempotency.New(s, idempotency.Options{KeyPrefix: "local-replay", EventKeyJMESPath: "id", PayloadValidationJMESPath: "amount", Now: func() time.Time { return now }, Diagnostic: func(string) {}})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	calls := 0
	handler := func(context.Context) (int, error) { calls++; return calls, nil }
	payload := map[string]any{"id": "order-a", "amount": 7}
	for range 2 {
		// A fresh manager prevents its process-local cache from proving replay.
		value, err := idempotency.Execute(ctx, newManager(), payload, handler)
		if err != nil || value != 1 || calls != 1 {
			t.Fatalf("persisted replay: %v/%v calls=%d", value, err, calls)
		}
	}
	_, err = idempotency.Execute(ctx, newManager(), map[string]any{"id": "order-a", "amount": 8}, handler)
	var validation *idempotency.ValidationError
	if !errors.As(err, &validation) || calls != 1 {
		t.Fatalf("payload validation: %v calls=%d", err, calls)
	}
}

func TestDynamoDBLocalExpiryLeaseAndCompositeKeys(t *testing.T) {
	client, ctx, _ := serviceClient(t)
	s := store(t, client, idempotent.Options{TableName: table(t, ctx, client, "id", "")})
	now := time.Unix(1800000000, 0)
	for _, item := range []struct {
		name          string
		status        idempotency.Status
		expiry, lease int64
		recover       bool
	}{
		{"expired", idempotency.Completed, now.Unix() - 1, 0, true},
		{"lease", idempotency.InProgress, now.Unix() + 3600, now.UnixMilli() - 1, true},
		{"expiry-boundary", idempotency.Completed, now.Unix(), 0, false},
		{"lease-boundary", idempotency.InProgress, now.Unix() + 3600, now.UnixMilli(), false},
		{"completed-old-lease", idempotency.Completed, now.Unix() + 3600, now.UnixMilli() - 1, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			old := idempotency.Record{Key: item.name, Status: item.status, Expiration: item.expiry, InProgressExpiration: item.lease, Validation: "old"}
			if err := s.Put(ctx, old, now.Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
			fresh := idempotency.Record{Key: item.name, Status: idempotency.InProgress, Expiration: now.Unix() + 3600, Validation: "fresh"}
			err := s.Put(ctx, fresh, now)
			if item.recover {
				if err != nil {
					t.Fatal(err)
				}
				got, err := s.Get(ctx, item.name)
				if err != nil || got.Validation != "fresh" {
					t.Fatalf("recovered state: %+v/%v", got, err)
				}
			} else {
				var conflict *idempotency.AlreadyExistsError
				if !errors.As(err, &conflict) || conflict.Record == nil || conflict.Record.Validation != "old" {
					t.Fatalf("boundary should retain old record: %v", err)
				}
			}
			if err := s.Delete(ctx, item.name); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Get(ctx, item.name); !errors.Is(err, idempotency.ErrNotFound) {
				t.Fatalf("deletion: %v", err)
			}
		})
	}
	t.Run("composite-tenants", func(t *testing.T) {
		name := table(t, ctx, client, "pk", "sk")
		a := store(t, client, idempotent.Options{TableName: name, KeyAttribute: "pk", SortKeyAttribute: "sk", StaticPartitionKey: "tenant-a"})
		b := store(t, client, idempotent.Options{TableName: name, KeyAttribute: "pk", SortKeyAttribute: "sk", StaticPartitionKey: "tenant-b"})
		for index, s := range []*idempotent.Store{a, b} {
			r := idempotency.Record{Key: "same", Status: idempotency.InProgress, Expiration: now.Unix() + 3600, Validation: fmt.Sprint(index)}
			if err := s.Put(ctx, r, now); err != nil {
				t.Fatal(err)
			}
			got, err := s.Get(ctx, "same")
			if err != nil || got.Key != "same" || got.Validation != fmt.Sprint(index) {
				t.Fatalf("tenant isolation: %+v/%v", got, err)
			}
		}
		if err := a.Delete(ctx, "same"); err != nil {
			t.Fatal(err)
		}
		if _, err := a.Get(ctx, "same"); !errors.Is(err, idempotency.ErrNotFound) {
			t.Fatalf("tenant deletion: %v", err)
		}
		if got, err := b.Get(ctx, "same"); err != nil || got.Validation != "1" {
			t.Fatalf("other tenant lost: %+v/%v", got, err)
		}
	})
}

func TestDynamoDBLocalParameters(t *testing.T) {
	client, ctx, counter := serviceClient(t)
	name := table(t, ctx, client, "id", "")
	put := func(key string, value types.AttributeValue) {
		t.Helper()
		_, err := client.PutItem(ctx, &sdk.PutItemInput{TableName: aws.String(name), Item: map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: key}, "value": value}})
		if err != nil {
			t.Fatal(err)
		}
	}
	put("config", &types.AttributeValueMemberS{Value: `{"enabled":true}`})
	p, err := parameterddb.New(client, parameterddb.Config{TableName: name})
	if err != nil {
		t.Fatal(err)
	}
	opts := parameterddb.GetOptions{Options: parameters.Options{Transform: parameters.JSON, MaxAge: parameters.Age(time.Minute)}}
	get := func(want bool, options parameterddb.GetOptions) {
		t.Helper()
		got, err := p.Get(ctx, "config", options)
		if err != nil || !reflect.DeepEqual(got, map[string]any{"enabled": want}) {
			t.Fatalf("configuration: %#v/%v", got, err)
		}
	}
	get(true, opts)
	put("config", &types.AttributeValueMemberS{Value: `{"enabled":false}`})
	get(true, opts)
	opts.ForceFetch = true
	get(false, opts)
	p.ClearCache()
	opts.ForceFetch = false
	get(false, opts)
	put("binary", &types.AttributeValueMemberB{Value: []byte{0, 1, 255}})
	got, err := p.Get(ctx, "binary", parameterddb.GetOptions{})
	if err != nil || !reflect.DeepEqual(got, []byte{0, 1, 255}) {
		t.Fatalf("native binary: %#v/%v", got, err)
	}
	if got, err := p.Get(ctx, "missing", parameterddb.GetOptions{}); err != nil || got != nil {
		t.Fatalf("missing value: %v/%v", got, err)
	}
	_, err = p.Get(ctx, "missing", parameterddb.GetOptions{Options: parameters.Options{ThrowOnMissing: true}})
	var missing *parameters.ParameterNotFoundError
	if !errors.As(err, &missing) {
		t.Fatalf("strict missing: %v", err)
	}
	queryTable := table(t, ctx, client, "pk", "name")
	for key, value := range map[string]string{"a.json": "true", "b.binary": "aGVsbG8="} {
		_, err := client.PutItem(ctx, &sdk.PutItemInput{TableName: aws.String(queryTable), Item: map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "group"}, "name": &types.AttributeValueMemberS{Value: key}, "data": &types.AttributeValueMemberS{Value: value}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	query, err := parameterddb.New(client, parameterddb.Config{TableName: queryTable, KeyAttribute: "pk", SortAttribute: "name", ValueAttribute: "data"})
	if err != nil {
		t.Fatal(err)
	}
	values, err := query.GetMultiple(ctx, "group", parameterddb.MultipleOptions{Options: parameters.Options{Transform: parameters.Auto}, SDKOptions: &sdk.QueryInput{Limit: aws.Int32(1), ConsistentRead: aws.Bool(true)}})
	if err != nil || !reflect.DeepEqual(values, map[string]any{"a.json": true, "b.binary": "hello"}) {
		t.Fatalf("stateful query: %#v/%v", values, err)
	}
	// DynamoDB may return a final continuation key and require an empty last page.
	if calls := counter.calls.Load(); calls < 2 || calls > 3 {
		t.Fatalf("expected real Limit=1 pagination, got %d requests", calls)
	}
}
