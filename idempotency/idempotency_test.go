package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memoryStore is an atomic test double, not a durable production backend.
type memoryStore struct {
	mu                           sync.Mutex
	records                      map[string]Record
	operations                   []string
	putErr, updateErr, deleteErr error
	conflicts                    int
}

func newMemory() *memoryStore {
	return &memoryStore{records: map[string]Record{}, operations: []string{}}
}
func (s *memoryStore) Put(ctx context.Context, r Record, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "put")
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.putErr != nil {
		return s.putErr
	}
	if s.conflicts > 0 {
		s.conflicts--
		return &AlreadyExistsError{}
	}
	if old, ok := s.records[r.Key]; ok && !old.IsExpired(now) && !(old.Status == InProgress && old.InProgressExpiration != 0 && old.InProgressExpiration < now.UnixMilli()) {
		copy := old.Clone()
		return &AlreadyExistsError{&copy}
	}
	s.records[r.Key] = r.Clone()
	return nil
}
func (s *memoryStore) Get(ctx context.Context, key string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "get")
	if r, ok := s.records[key]; ok {
		return r.Clone(), nil
	}
	return Record{}, ErrNotFound
}
func (s *memoryStore) Update(ctx context.Context, r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "update")
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.updateErr != nil {
		return s.updateErr
	}
	s.records[r.Key] = r.Clone()
	return nil
}
func (s *memoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "delete")
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.records, key)
	return nil
}

type fixtureConfig struct {
	EventKeyJMESPath          string  `json:"eventKeyJmesPath"`
	PayloadValidationJMESPath string  `json:"payloadValidationJmesPath"`
	HashFunction              string  `json:"hashFunction"`
	ThrowOnNoKey              bool    `json:"throwOnNoIdempotencyKey"`
	ExpiresAfterSeconds       float64 `json:"expiresAfterSeconds"`
	UseLocalCache             bool    `json:"useLocalCache"`
}

func (c fixtureConfig) options(now func() time.Time) Options {
	o := Options{KeyPrefix: "operation", EventKeyJMESPath: c.EventKeyJMESPath, PayloadValidationJMESPath: c.PayloadValidationJMESPath, HashFunction: c.HashFunction, ThrowOnNoKey: c.ThrowOnNoKey, UseLocalCache: c.UseLocalCache, Now: now, Diagnostic: func(string) {}}
	if c.ExpiresAfterSeconds != 0 {
		ttl := time.Duration(c.ExpiresAfterSeconds * float64(time.Second))
		o.ExpiresAfter = &ttl
	}
	return o
}

type fixtures struct {
	Keys []struct {
		Raw, Canonical, Key, Validation string
		Config                          fixtureConfig
	}
	Scenarios []struct {
		Name, Seed string
		Config     fixtureConfig
		Steps      []struct {
			Event         json.RawMessage
			Fail          bool
			Advance, Work int64
		}
		Results []struct {
			Response     json.RawMessage
			Error, Cause string
		}
		Calls      int
		Operations []string
		Records    []Record
	}
}

func readFixtures(t *testing.T) fixtures {
	t.Helper()
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var result fixtures
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func equivalentJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("JSON mismatch\ngot: %s\nwant: %s", got, want)
	}
}

func TestReferenceKeys(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	for _, fixture := range readFixtures(t).Keys {
		t.Run(fixture.Raw+fixture.Config.HashFunction, func(t *testing.T) {
			m, err := New(newMemory(), fixture.Config.options(nil))
			if err != nil {
				t.Fatal(err)
			}
			key, validation, skip, err := m.Key(json.RawMessage(fixture.Raw))
			if fixture.Raw == `{"same":1,"same":2}` {
				// JSON v2 rejects duplicate members before a key can be created.
				if err == nil || key != "" || !strings.Contains(err.Error(), "duplicate object member name") {
					t.Fatalf("duplicate payload accepted: key=%s error=%v", key, err)
				}
				return
			}
			if err != nil || skip || key != fixture.Key || validation != fixture.Validation {
				t.Fatalf("key=%s validation=%s skip=%v error=%v; expected %s %s", key, validation, skip, err, fixture.Key, fixture.Validation)
			}
			if fixture.Canonical != "" {
				encoded, err := CanonicalJSON(json.RawMessage(fixture.Raw))
				if err != nil || string(encoded) != fixture.Canonical {
					t.Fatalf("canonical: %s (%v), want %s", encoded, err, fixture.Canonical)
				}
			}
		})
	}
}

func TestReferenceLifecycle(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	for _, fixture := range readFixtures(t).Scenarios {
		t.Run(fixture.Name, func(t *testing.T) {
			now := time.UnixMilli(1800000000250)
			calls := 0
			store := newMemory()
			m, err := New(store, fixture.Config.options(func() time.Time { return now }))
			if err != nil {
				t.Fatal(err)
			}
			if fixture.Seed != "" {
				key, _, _, _ := m.Key(json.RawMessage(`{"id":"a"}`))
				deadline := now.UnixMilli() + 5000
				if fixture.Seed == "expired" {
					deadline = now.UnixMilli() - 1
				}
				store.records[key] = Record{Key: key, Status: InProgress, Expiration: 1800003600, InProgressExpiration: deadline}
			}
			for i, step := range fixture.Steps {
				now = now.Add(time.Duration(step.Advance) * time.Millisecond)
				ctx, cancel := context.WithDeadline(context.Background(), now.Add(5*time.Second))
				response, err := Execute(ctx, m, step.Event, func(context.Context) (map[string]any, error) {
					calls++
					if step.Fail {
						return nil, errors.New("business failure")
					}
					now = now.Add(time.Duration(step.Work) * time.Millisecond)
					return map[string]any{"call": calls, "event": step.Event}, nil
				})
				cancel()
				want := fixture.Results[i]
				if want.Error != "" {
					if err == nil {
						t.Fatalf("step %d expected %s", i, want.Error)
					}
					var keyError *KeyError
					var validationError *ValidationError
					var progressError *AlreadyInProgressError
					switch want.Error {
					case "IdempotencyPersistenceLayerError":
						if want.Cause != "IdempotencyKeyError" || !errors.As(err, &keyError) {
							t.Fatalf("unexpected error: %v", err)
						}
					case "IdempotencyValidationError":
						if !errors.As(err, &validationError) {
							t.Fatal(err)
						}
					case "IdempotencyAlreadyInProgressError":
						if !errors.As(err, &progressError) {
							t.Fatal(err)
						}
					case "Error":
						if err.Error() != "business failure" {
							t.Fatal(err)
						}
					default:
						t.Fatalf("unmapped reference error: %s", want.Error)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					encoded, _ := json.Marshal(response)
					equivalentJSON(t, encoded, want.Response)
				}
			}
			if calls != fixture.Calls || !reflect.DeepEqual(store.operations, fixture.Operations) {
				t.Fatalf("calls=%d ops=%v; want %d %v", calls, store.operations, fixture.Calls, fixture.Operations)
			}
			if len(store.records) != len(fixture.Records) {
				t.Fatalf("records=%v", store.records)
			}
			for _, want := range fixture.Records {
				got := store.records[want.Key]
				a, _ := json.Marshal(got)
				b, _ := json.Marshal(want)
				equivalentJSON(t, a, b)
			}
		})
	}
}

func TestConcurrentAcquisitionAndReplayIsolation(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	store := newMemory()
	m, err := New(store, Options{KeyPrefix: "shared", UseLocalCache: true, Diagnostic: func(string) {}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	var executions atomic.Int32
	handler := func(context.Context) (map[string]any, error) {
		executions.Add(1)
		close(started)
		<-release
		return map[string]any{"nested": map[string]any{"ok": true}}, nil
	}
	go func() { _, err := Execute(ctx, m, "same", handler); done <- err }()
	<-started
	var wg sync.WaitGroup
	failures := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Execute(ctx, m, "same", func(context.Context) (map[string]any, error) { executions.Add(1); return nil, nil })
			var progress *AlreadyInProgressError
			if !errors.As(err, &progress) {
				failures <- err
			}
		}()
	}
	wg.Wait()
	close(failures)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for err := range failures {
		t.Errorf("expected contention error, got %v", err)
	}
	if executions.Load() != 1 {
		t.Fatalf("executions=%d", executions.Load())
	}
	first, err := Execute(ctx, m, "same", handler)
	if err != nil {
		t.Fatal(err)
	}
	first["nested"].(map[string]any)["ok"] = false
	second, err := Execute(ctx, m, "same", handler)
	if err != nil || second["nested"].(map[string]any)["ok"] != true {
		t.Fatalf("cache alias: %v %v", second, err)
	}
}

func TestFailureBoundariesAndRetries(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	business, persist := errors.New("business"), errors.New("persistence")
	for _, stage := range []string{"put", "update", "delete", "panic", "retry", "exhausted"} {
		t.Run(stage, func(t *testing.T) {
			store := newMemory()
			m, _ := New(store, Options{Diagnostic: func(string) {}})
			calls := 0
			switch stage {
			case "put":
				store.putErr = persist
			case "update":
				store.updateErr = persist
			case "delete":
				store.deleteErr = persist
			case "retry":
				store.conflicts = 2
			case "exhausted":
				store.conflicts = 3
			}
			var panicValue any
			var err error
			func() {
				defer func() { panicValue = recover() }()
				_, err = Execute(ctx, m, "key", func(context.Context) (string, error) {
					calls++
					if stage == "panic" {
						panic(business)
					}
					if stage == "delete" {
						return "", business
					}
					return "ok", nil
				})
			}()
			switch stage {
			case "put":
				if !errors.Is(err, persist) || calls != 0 {
					t.Fatalf("%v calls=%d", err, calls)
				}
			case "update":
				if !errors.Is(err, persist) || len(store.records) != 1 {
					t.Fatalf("%v records=%v", err, store.records)
				}
			case "delete":
				if !errors.Is(err, persist) || !errors.Is(err, business) {
					t.Fatal(err)
				}
			case "panic":
				if panicValue != business || len(store.records) != 0 {
					t.Fatalf("panic=%v records=%v", panicValue, store.records)
				}
			case "retry":
				if err != nil || calls != 1 {
					t.Fatalf("%v calls=%d", err, calls)
				}
			case "exhausted":
				var inconsistent *InconsistentStateError
				if !errors.As(err, &inconsistent) || calls != 0 {
					t.Fatalf("%v calls=%d", err, calls)
				}
			}
		})
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	store := newMemory()
	m, _ := New(store, Options{})
	_, err := Execute(canceled, m, "key", func(context.Context) (int, error) { t.Fatal("executed after cancellation"); return 0, nil })
	if !errors.Is(err, context.Canceled) || len(store.operations) != 0 {
		t.Fatal(err)
	}
}

func TestOperationIsolationDisabledAndHooks(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	store := newMemory()
	a, _ := New(store, Options{KeyPrefix: "a", UseLocalCache: true, Diagnostic: func(string) {}})
	b, _ := New(store, Options{KeyPrefix: "b", EventKeyJMESPath: "id", Diagnostic: func(string) {}})
	ctx := context.Background()
	calls := 0
	handler := func(context.Context) (int, error) { calls++; return calls, nil }
	first, _ := Execute(ctx, a, map[string]any{"id": 1}, handler)
	second, _ := Execute(ctx, b, map[string]any{"id": 1}, handler)
	if first != 1 || second != 2 || len(store.records) != 2 {
		t.Fatal("shared store leaked operation configuration")
	}
	replayed, err := Execute(ctx, a, map[string]any{"id": 1}, handler, func(_ context.Context, response int, record Record) (int, error) {
		record.Data[0] = '9'
		return response + 10, nil
	})
	again, _ := Execute(ctx, a, map[string]any{"id": 1}, handler)
	if err != nil || replayed != 11 || again != 1 || calls != 2 {
		t.Fatalf("replay=%d again=%d calls=%d error=%v", replayed, again, calls, err)
	}
	a.ClearCache()
	_, err = Execute(ctx, a, map[string]any{"id": 1}, handler)
	if err != nil || calls != 2 {
		t.Fatal(err)
	}
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "yes")
	disabled, _ := New(store, Options{KeyPrefix: "a", EventKeyJMESPath: "id"})
	before := len(store.operations)
	_, _ = Execute(ctx, disabled, make(chan int), handler)
	if len(store.operations) != before || calls != 3 {
		t.Fatal("disabled path performed idempotency work")
	}
}

func TestExpiryBoundaryAndInvalidState(t *testing.T) {
	now := time.Unix(1800000000, 0)
	r := Record{Status: Completed, Expiration: now.Unix()}
	if r.IsExpired(now) || !r.IsExpired(now.Add(time.Nanosecond)) {
		t.Fatal("expiry is strictly greater than timestamp")
	}
	r.Status = "invalid"
	if _, err := r.CurrentStatus(now); err == nil {
		t.Fatal("accepted invalid status")
	}
	if status, err := r.CurrentStatus(now.Add(time.Second)); err != nil || status != Expired {
		t.Fatalf("%s %v", status, err)
	}
}
