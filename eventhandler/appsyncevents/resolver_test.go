package appsyncevents

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func event(path, operation string, payloads ...any) Event {
	var messages any
	if operation == "PUBLISH" {
		items := make([]any, len(payloads))
		for i, payload := range payloads {
			items[i] = map[string]any{"id": fmt.Sprint(i), "payload": payload}
		}
		messages = items
	}
	return Event{"identity": nil, "result": nil, "request": map[string]any{"headers": map[string]any{}, "domainName": nil}, "error": nil, "prev": nil, "stash": map[string]any{}, "outErrors": []any{}, "events": messages, "info": map[string]any{"channel": map[string]any{"path": path, "segments": []any{"default"}}, "channelNamespace": map[string]any{"name": "default"}, "operation": operation}}
}

func quiet() Options { return Options{Diagnostic: func(context.Context, string, string, error) {}} }

func payload(result any) any {
	return result.(map[string]any)["events"].([]any)[0].(map[string]any)["payload"]
}

func TestConcurrentPublicationWaitsAndRetainsOrder(t *testing.T) {
	app := New(quiet())
	started, release := make(chan int, 8), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.OnPublish("/*", func(received context.Context, value any, e Event) (any, error) {
		if received != ctx {
			return nil, errors.New("context replaced")
		}
		started <- value.(int)
		<-release
		return value, nil
	})
	input := event("/default/test", "PUBLISH", 0, 1, 2, 3, 4, 5, 6, 7)
	done := make(chan any, 1)
	go func() {
		result, err := app.Resolve(ctx, input)
		if err != nil {
			done <- err
		} else {
			done <- result
		}
	}()
	for range 8 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("individual handlers did not run concurrently")
		}
	}
	select {
	case <-done:
		t.Fatal("returned before handlers finished")
	default:
	}
	cancel()
	close(release)
	result := <-done
	items := result.(map[string]any)["events"].([]any)
	for i, item := range items {
		if item.(map[string]any)["payload"] != i {
			t.Fatal("response order changed", result)
		}
	}
	if !reflect.DeepEqual(input, event("/default/test", "PUBLISH", 0, 1, 2, 3, 4, 5, 6, 7)) {
		t.Fatal("input changed")
	}
}

func TestConcurrentInvocationsAndMissingRouteWarning(t *testing.T) {
	var mu sync.Mutex
	warnings := 0
	app := New(Options{Diagnostic: func(ctx context.Context, level, message string, err error) {
		if level == "warn" {
			mu.Lock()
			warnings++
			mu.Unlock()
		}
	}})
	type requestKey struct{}
	app.OnPublish("/default/*", func(ctx context.Context, value any, e Event) (any, error) { return ctx.Value(requestKey{}), nil })
	var group sync.WaitGroup
	for i := range 64 {
		group.Go(func() {
			ctx := context.WithValue(context.Background(), requestKey{}, i)
			result, err := app.Resolve(ctx, event("/default/test", "PUBLISH", i))
			if err != nil || payload(result) != i {
				t.Errorf("%d: %v / %v", i, result, err)
			}
			if _, err := app.Resolve(ctx, event("/missing", "PUBLISH", nil)); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if warnings != 1 {
		t.Fatalf("missing-route warnings: %d", warnings)
	}
}

func TestRouteCacheRetainsRegistrationUntilEviction(t *testing.T) {
	app := New(quiet())
	register := func(tag string) {
		app.OnPublish("/*", func(context.Context, any, Event) (any, error) { return tag, nil })
	}
	resolve := func(path string) any {
		result, err := app.Resolve(context.Background(), event(path, "PUBLISH", 1))
		if err != nil {
			t.Fatal(err)
		}
		return payload(result)
	}
	register("old")
	if resolve("/default/a") != "old" {
		t.Fatal("initial resolution")
	}
	register("new")
	if resolve("/default/a") != "old" {
		t.Fatal("cached route replaced prematurely")
	}
	for i := range 100 {
		if resolve(fmt.Sprintf("/default/%d", i)) != "new" {
			t.Fatal("new lookup used stale registration")
		}
	}
	if app.publish.cache.Size() != 100 || resolve("/default/a") != "new" {
		t.Fatal("LRU capacity/eviction mismatch")
	}
}

func TestAuthorizationAndDiagnosticPanic(t *testing.T) {
	denied := &UnauthorizedError{Message: "denied"}
	if reflect.TypeOf(denied).Elem().Name() != "UnauthorizedException" {
		t.Fatal("Lambda errorType differs from the reference")
	}
	for _, aggregate := range []bool{false, true} {
		app := New(quiet())
		app.OnPublish("/*", func(context.Context, any, Event) (any, error) { return nil, denied }, PublishOptions{Aggregate: aggregate})
		result, err := app.Resolve(context.Background(), event("/default", "PUBLISH", 1))
		if err != denied || result != nil {
			t.Fatal("authorization identity lost", result, err)
		}
		app.OnSubscribe("/*", func(context.Context, Event) error { return denied })
		if _, err := app.Resolve(context.Background(), event("/default", "SUBSCRIBE")); err != denied {
			t.Fatal("subscription authorization swallowed")
		}
	}
	failure := &struct{}{}
	app := New(Options{Diagnostic: func(ctx context.Context, level, message string, err error) {
		if level == "error" {
			panic(failure)
		}
	}})
	app.OnPublish("/*", func(context.Context, any, Event) (any, error) { return nil, errors.New("business") })
	defer func() {
		if recover() != failure {
			t.Fatal("diagnostic panic was lost")
		}
	}()
	_, _ = app.Resolve(context.Background(), event("/default", "PUBLISH", 1, 2))
}
