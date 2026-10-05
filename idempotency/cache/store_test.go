package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	"github.com/redis/go-redis/v9"
)

type fakeClient struct {
	values         map[string]string
	calls          []map[string]any
	disappear      bool
	err            error
	beforeRecovery func()
}

func newFake() *fakeClient {
	return &fakeClient{values: map[string]string{}, calls: []map[string]any{}}
}
func (f *fakeClient) Get(ctx context.Context, key string) *redis.StringCmd {
	f.calls = append(f.calls, map[string]any{"operation": "get", "key": key})
	if f.err != nil {
		return redis.NewStringResult("", f.err)
	}
	if f.disappear {
		delete(f.values, key)
	}
	if value, ok := f.values[key]; ok {
		return redis.NewStringResult(value, nil)
	}
	return redis.NewStringResult("", redis.Nil)
}
func (f *fakeClient) SetArgs(ctx context.Context, key string, value any, args redis.SetArgs) *redis.StatusCmd {
	raw := fmt.Sprint(value)
	var decoded any
	_ = json.Unmarshal([]byte(raw), &decoded)
	f.calls = append(f.calls, map[string]any{"operation": "set", "key": key, "value": decoded, "ttl": int64(args.TTL / time.Second), "nx": args.Mode == "NX"})
	if f.err != nil {
		return redis.NewStatusResult("", f.err)
	}
	if _, ok := f.values[key]; ok && args.Mode == "NX" {
		return redis.NewStatusResult("", redis.Nil)
	}
	f.values[key] = raw
	return redis.NewStatusResult("OK", nil)
}
func (f *fakeClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	for _, key := range keys {
		f.calls = append(f.calls, map[string]any{"operation": "delete", "key": key})
		delete(f.values, key)
	}
	return redis.NewIntResult(int64(len(keys)), f.err)
}
func (f *fakeClient) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	if script != recoverScript || len(keys) != 1 {
		return redis.NewCmdResult(nil, errors.New("unexpected recovery script"))
	}
	if f.beforeRecovery != nil {
		f.beforeRecovery()
	}
	if f.err != nil {
		return redis.NewCmdResult(nil, f.err)
	}
	if f.values[keys[0]] != args[0] {
		return redis.NewCmdResult(int64(0), nil)
	}
	// Compare the successful recovery write with the reference SET. A separate
	// race test verifies the additional compare-and-set precondition.
	result := f.SetArgs(ctx, keys[0], args[1], redis.SetArgs{TTL: time.Duration(args[2].(int64)) * time.Second})
	return redis.NewCmdResult(int64(1), result.Err())
}

func equivalent(t *testing.T, got, want any) {
	t.Helper()
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(want)
	var x, y any
	_ = json.Unmarshal(a, &x)
	_ = json.Unmarshal(b, &y)
	if !reflect.DeepEqual(x, y) {
		t.Fatalf("got %s\nwant %s", a, b)
	}
}
func finalValues(f *fakeClient) map[string]any {
	result := map[string]any{}
	for key, value := range f.values {
		var decoded any
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			decoded = value
		}
		result[key] = decoded
	}
	return result
}

func TestReferenceCacheScenarios(t *testing.T) {
	encoded, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Now       int64
		Scenarios []struct {
			Name, Operation, Seed, Error string
			Locked, Disappear, Custom    bool
			Record                       idempotency.Record
			Response                     *idempotency.Record
			Calls                        []map[string]any
			Final                        map[string]any
		}
	}
	if err = json.Unmarshal(encoded, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Scenarios {
		t.Run(fixture.Name, func(t *testing.T) {
			f := newFake()
			key := fixture.Record.Key
			if fixture.Seed != "" {
				f.values[key] = fixture.Seed
			}
			if fixture.Locked {
				f.values[key+":lock"] = "true"
			}
			f.disappear = fixture.Disappear
			now := time.UnixMilli(fixtures.Now)
			options := Options{Now: func() time.Time { return now }, OmitValidationOnSuccess: true}
			if fixture.Custom {
				options.StatusAttribute = "s"
				options.ExpiryAttribute = "e"
				options.InProgressExpiryAttribute = "ip"
				options.DataAttribute = "d"
				options.ValidationAttribute = "v"
			}
			store, err := New(f, options)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.Name == "missing-lease" {
				// Issue #52 deliberately rejects this unsafe v2.35.0 takeover.
				// Keep the upstream fixture intact and assert the Go correction.
				err := store.Put(context.Background(), fixture.Record, now)
				var conflict *idempotency.AlreadyExistsError
				if !errors.As(err, &conflict) || conflict.Record == nil || f.values[key] != fixture.Seed || len(f.values) != 1 {
					t.Fatalf("record without deadline was not preserved: %v", err)
				}
				return
			}
			var result idempotency.Record
			switch fixture.Operation {
			case "put":
				err = store.Put(context.Background(), fixture.Record, now)
			case "get":
				result, err = store.Get(context.Background(), key)
			case "update":
				err = store.Update(context.Background(), fixture.Record)
			case "delete":
				err = store.Delete(context.Background(), key)
			}
			if fixture.Error == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var exists *idempotency.AlreadyExistsError
				var invalid *idempotency.InvalidStatusError
				var corrupt *ConsistencyError
				switch fixture.Error {
				case "IdempotencyItemAlreadyExistsError":
					if !errors.As(err, &exists) {
						t.Fatal(err)
					}
				case "IdempotencyInvalidStatusError":
					if !errors.As(err, &invalid) {
						t.Fatal(err)
					}
				case "IdempotencyPersistenceConsistencyError":
					if !errors.As(err, &corrupt) {
						t.Fatal(err)
					}
				case "IdempotencyItemNotFoundError":
					if fixture.Disappear {
						if !errors.As(err, &exists) {
							t.Fatal(err)
						}
					} else if !errors.Is(err, idempotency.ErrNotFound) {
						t.Fatal(err)
					}
				default:
					t.Fatalf("unknown reference error: %s", fixture.Error)
				}
			}
			if fixture.Response != nil {
				equivalent(t, result, *fixture.Response)
			}
			equivalent(t, f.calls, fixture.Calls)
			equivalent(t, finalValues(f), fixture.Final)
		})
	}
}

func TestRecoveryCannotOverwriteChangedRecord(t *testing.T) {
	now := time.Unix(1800000000, 0)
	f := newFake()
	f.values["key"] = `{"status":"INPROGRESS","expiration":1800003600,"in_progress_expiration":1}`
	winner := `{"status":"COMPLETED","expiration":1800003600,"data":"winner"}`
	f.beforeRecovery = func() { f.values["key"] = winner }
	store, _ := New(f, Options{})
	err := store.Put(context.Background(), idempotency.Record{Key: "key", Status: idempotency.InProgress, Expiration: 1800003600}, now)
	var conflict *idempotency.AlreadyExistsError
	if !errors.As(err, &conflict) || f.values["key"] != winner {
		t.Fatalf("error=%v record=%s", err, f.values["key"])
	}
}

func TestValidationPreservedAndResponseIsolated(t *testing.T) {
	now := time.Unix(1800000000, 0)
	f := newFake()
	store, _ := New(f, Options{Now: func() time.Time { return now }})
	r := idempotency.Record{Key: "key", Status: idempotency.Completed, Expiration: 1800003600, Data: json.RawMessage(`{"large":9007199254740993}`), Validation: "hash"}
	if err := store.Update(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	first, err := store.Get(context.Background(), "key")
	if err != nil || first.Validation != "hash" {
		t.Fatal(err)
	}
	first.Data[0] = '['
	second, err := store.Get(context.Background(), "key")
	if err != nil || string(second.Data) != string(r.Data) {
		t.Fatalf("%s %v", second.Data, err)
	}
}

func TestConfigurationAndFailures(t *testing.T) {
	f := newFake()
	if _, err := New(nil, Options{}); err == nil {
		t.Fatal("nil client")
	}
	if _, err := New(f, Options{DataAttribute: "status"}); err == nil {
		t.Fatal("duplicate attributes")
	}
	store, _ := New(f, Options{})
	ctx := context.Background()
	now := time.Now()
	r := idempotency.Record{Key: "key", Status: idempotency.InProgress, Expiration: now.Unix() + 60}
	sentinel := errors.New("connection failed")
	f.err = sentinel
	if err := store.Put(ctx, r, now); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "key"); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if err := store.Update(ctx, r); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "key"); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	f.err = nil
	r.Status = idempotency.Completed
	if err := store.Put(ctx, r, now); err == nil {
		t.Fatal("non-progress acquisition")
	}
	r.Status = idempotency.InProgress
	r.Expiration = now.Unix()
	if err := store.Put(ctx, r, now); err == nil {
		t.Fatal("zero TTL")
	}
}
