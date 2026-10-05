package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
)

func TestAcquisitionWithoutExecutionDeadline(t *testing.T) {
	now := time.Unix(1800000000, 0)
	for _, tc := range []struct {
		name     string
		stored   string
		conflict bool
	}{
		{"absent deadline", `{"status":"INPROGRESS","expiration":1800003600}`, true},
		{"zero deadline", `{"status":"INPROGRESS","expiration":1800003600,"in_progress_expiration":0}`, true},
		{"future deadline", `{"status":"INPROGRESS","expiration":1800003600,"in_progress_expiration":1800000060000}`, true},
		{"completed", `{"status":"COMPLETED","expiration":1800003600,"data":"done"}`, true},
		{"expired record without deadline", `{"status":"INPROGRESS","expiration":1799999999}`, false},
		{"expired execution deadline", `{"status":"INPROGRESS","expiration":1800003600,"in_progress_expiration":1799999999000}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newFake()
			client.values["key"] = tc.stored
			store, err := New(client, Options{})
			if err != nil {
				t.Fatal(err)
			}
			proposed := idempotency.Record{Key: "key", Status: idempotency.InProgress, Expiration: now.Unix() + 3600, InProgressExpiration: now.Add(time.Minute).UnixMilli()}
			err = store.Put(context.Background(), proposed, now)
			if tc.conflict {
				var conflict *idempotency.AlreadyExistsError
				if !errors.As(err, &conflict) || conflict.Record == nil || conflict.Record.Key != "key" {
					t.Fatalf("expected existing record, got %v", err)
				}
				if client.values["key"] != tc.stored || len(client.values) != 1 {
					t.Fatal("active record was changed or a recovery lock was acquired")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				got, err := store.Get(context.Background(), "key")
				if err != nil {
					t.Fatal(err)
				}
				equivalent(t, got, proposed)
			}
		})
	}
}

func TestOverlappingExecutionWithoutDeadline(t *testing.T) {
	now := time.Unix(1800000000, 0)
	client := newFake()
	store, err := New(client, Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := idempotency.New(store, idempotency.Options{KeyPrefix: "deadline-test", Now: func() time.Time { return now }, Diagnostic: func(string) {}})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	type outcome struct {
		value string
		err   error
	}
	done := make(chan outcome, 1)
	payload := map[string]any{"id": "same-operation"}
	go func() {
		value, err := idempotency.Execute(context.Background(), manager, payload, func(context.Context) (string, error) {
			calls.Add(1)
			close(entered)
			<-release
			return "owner-result", nil
		})
		done <- outcome{value, err}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("owner did not enter the callback")
	}
	// The owner is blocked outside persistence calls, keeping the fake client
	// deterministic while the two Execute lifetimes overlap.
	_, err = idempotency.Execute(context.Background(), manager, payload, func(context.Context) (string, error) {
		calls.Add(1)
		return "duplicate-result", nil
	})
	var inProgress *idempotency.AlreadyInProgressError
	if !errors.As(err, &inProgress) {
		t.Errorf("overlapping caller must be rejected, got %v", err)
	}
	close(release)
	select {
	case result := <-done:
		if result.err != nil || result.value != "owner-result" {
			t.Fatalf("owner result=%q error=%v", result.value, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("owner did not complete")
	}
	replay, err := idempotency.Execute(context.Background(), manager, payload, func(context.Context) (string, error) {
		calls.Add(1)
		return "unexpected-replay", nil
	})
	if err != nil || replay != "owner-result" || calls.Load() != 1 {
		t.Fatalf("replay=%q error=%v business calls=%d", replay, err, calls.Load())
	}
}
