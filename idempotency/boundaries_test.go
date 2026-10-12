package idempotency

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"math"
	"os"
	"testing"
	"time"
)

func TestCanonicalBoundaryReference(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	data, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Version string `json:"version"`
		Keys    []struct {
			Raw       string `json:"raw"`
			Canonical string `json:"canonical"`
			Key       string `json:"key"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if fixtures.Version != "2.35.0" || len(fixtures.Keys) != 6 {
		t.Fatal("unexpected boundary corpus")
	}
	manager, err := New(newMemory(), Options{KeyPrefix: "operation", Diagnostic: func(string) {}})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range fixtures.Keys {
		t.Run(row.Raw, func(t *testing.T) {
			raw := jsontext.Value(row.Raw)
			canonical, err := CanonicalJSON(raw)
			if err != nil || string(canonical) != row.Canonical {
				t.Fatalf("canonical=%s error=%v want=%s", canonical, err, row.Canonical)
			}
			key, _, skip, err := manager.Key(raw)
			if err != nil || skip || key != row.Key {
				t.Fatalf("key=%s skip=%v error=%v want=%s", key, skip, err, row.Key)
			}
		})
	}
}

func TestKeyRejectsInvalidNativeValuesBeforePersistence(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	for name, value := range map[string]any{
		"duplicate": jsontext.Value(`{"x":1,"x":2}`), "surrogate": jsontext.Value(`"\ud800"`),
		"overflow": jsontext.Value(`1e400`), "trailing": jsontext.Value(`{} {}`),
		"utf8": string([]byte{0xff}), "nan": math.NaN(), "infinity": math.Inf(1), "channel": make(chan int),
	} {
		t.Run(name, func(t *testing.T) {
			store := newMemory()
			manager, _ := New(store, Options{Diagnostic: func(string) {}})
			_, err := Execute(context.Background(), manager, value, func(context.Context) (int, error) { t.Fatal("invalid key executed"); return 0, nil })
			var keyErr *KeyError
			if !errors.As(err, &keyErr) || len(store.operations) != 0 {
				t.Fatalf("error=%v operations=%v", err, store.operations)
			}
		})
	}
}

func TestReplayValidationAndMalformedResponses(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	now := time.UnixMilli(1800000000250)
	for _, localCache := range []bool{false, true} {
		for _, state := range []string{"completed", "live", "no-lease", "invalid-status", "missing-data", "bad-data", "validation-mismatch"} {
			t.Run(state+map[bool]string{true: "-cache", false: "-store"}[localCache], func(t *testing.T) {
				store := newMemory()
				manager, err := New(store, Options{KeyPrefix: "audit", EventKeyJMESPath: "id", PayloadValidationJMESPath: "amount", UseLocalCache: localCache, Now: func() time.Time { return now }, Diagnostic: func(string) {}})
				if err != nil {
					t.Fatal(err)
				}
				payload := map[string]any{"id": "a", "amount": 7}
				key, validation, _, err := manager.Key(payload)
				if err != nil {
					t.Fatal(err)
				}
				r := Record{Key: key, Status: Completed, Expiration: 1800003600, Data: jsontext.Value(`{"owner":"stored","items":[]}`), Validation: validation}
				switch state {
				case "live":
					r.Status = InProgress
					r.InProgressExpiration = now.UnixMilli()
				case "no-lease":
					r.Status = InProgress
				case "invalid-status":
					r.Status = "UNKNOWN"
				case "missing-data":
					r.Data = nil
				case "bad-data":
					r.Data = jsontext.Value(`{"broken"`)
				case "validation-mismatch":
					r.Validation = "different"
				}
				store.records[key] = r.Clone()
				result, err := Execute(context.Background(), manager, payload, func(context.Context) (jsontext.Value, error) {
					t.Fatal("stored record invoked business handler")
					return nil, nil
				})
				switch state {
				case "completed":
					if err != nil || !result.IsValid() {
						t.Fatalf("response=%s error=%v", result, err)
					}
					result[0] = '['
					again, replayErr := Execute(context.Background(), manager, payload, func(context.Context) (jsontext.Value, error) { t.Fatal("replay missed"); return nil, nil })
					if replayErr != nil || !again.IsValid() {
						t.Fatalf("replay=%s error=%v", again, replayErr)
					}
				case "live", "no-lease":
					var progress *AlreadyInProgressError
					if !errors.As(err, &progress) {
						t.Fatal(err)
					}
				case "invalid-status":
					var invalid *InvalidStatusError
					if !errors.As(err, &invalid) {
						t.Fatal(err)
					}
				case "validation-mismatch":
					var invalid *ValidationError
					if !errors.As(err, &invalid) {
						t.Fatal(err)
					}
				default:
					var invalid *PersistenceError
					if !errors.As(err, &invalid) || invalid.Operation != "decode response" {
						t.Fatal(err)
					}
				}
			})
		}
	}
}
