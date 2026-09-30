package appsyncgraphql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sync"
	"testing"
)

func event(field string, value any) Event {
	return Event{"arguments": map[string]any{"value": value}, "identity": nil, "source": nil, "request": map[string]any{"headers": map[string]any{}, "domainName": nil}, "prev": nil, "info": map[string]any{"fieldName": field, "parentTypeName": "Query", "variables": map[string]any{}}, "stash": map[string]any{}}
}
func quiet() Options { return Options{Diagnostic: func(context.Context, string, string, error) {}} }

func TestConcurrentContextAndSequentialBatch(t *testing.T) {
	app := New(quiet())
	type contextKey struct{}
	app.OnBatchQuery("first", func(ctx context.Context, input, raw any) (any, error) {
		state := ctx.Value(contextKey{}).(*[]string)
		_, field := eventRoute(raw)
		*state = append(*state, field)
		if ctx.Err() != context.Canceled {
			return nil, errors.New("cancellation lost")
		}
		return input, nil
	}, BatchOptions{Individual: true})
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			seen := []string{}
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, &seen))
			cancel()
			input := []Event{event("first", i), event("second", i), event("third", i)}
			before := jsonValue(t, input)
			result, err := app.Resolve(ctx, input)
			if err != nil || !reflect.DeepEqual(seen, []string{"first", "second", "third"}) {
				t.Errorf("order/context lost: %v %v", seen, err)
				return
			}
			for _, item := range result.([]any) {
				if item.(map[string]any)["value"] != i {
					t.Errorf("cross-invocation result: %v", item)
				}
			}
			if !reflect.DeepEqual(before, jsonValue(t, input)) {
				t.Error("resolver mutated the event")
			}
		})
	}
	wg.Wait()
}

func TestErrorIdentityAndFallback(t *testing.T) {
	app := New(quiet())
	_, empty := app.Resolve(context.Background(), []any{})
	if typed, ok := empty.(*TypeError); !ok || reflect.TypeOf(typed).Elem().Name() != "TypeError" || errorName(empty) != "TypeError" {
		t.Fatalf("empty batch runtime error type: %T", empty)
	}
	marker := &ResolverNotFoundException{Message: "missing"}
	app.OnQuery("first", func(context.Context, any, any) (any, error) { return nil, marker })
	app.OnException([]string{"ResolverNotFoundException"}, func(context.Context, error) (any, error) { t.Error("framework exception intercepted"); return nil, nil })
	if _, err := app.Resolve(context.Background(), event("first", nil)); err != marker {
		t.Fatalf("identity lost: %v", err)
	}
	if reflect.TypeOf(marker).Elem().Name() != "ResolverNotFoundException" {
		t.Fatal("runtime error type changed")
	}
	business := errors.New("original")
	app.OnQuery("first", func(context.Context, any, any) (any, error) { panic(business) })
	app.OnException([]string{"Error"}, func(_ context.Context, err error) (any, error) {
		if err != business {
			t.Error("panic error identity lost")
		}
		panic("secondary")
	})
	value, err := app.Resolve(context.Background(), event("first", nil))
	if err != nil || value.(map[string]any)["error"] != "Error - original" {
		t.Fatalf("fallback: %v %v", value, err)
	}
}

func TestDiagnosticReentrancyAndFailure(t *testing.T) {
	var app *Resolver
	app = New(Options{Diagnostic: func(_ context.Context, level, message string, _ error) {
		if level == "warn" {
			app.OnQuery("extra", func(context.Context, any, any) (any, error) { return "extra", nil })
		}
		if message == "Looking for resolver for type=Query, field=panic" {
			panic(errors.New("diagnostic failed"))
		}
	}})
	app.OnQuery("first", nil)
	app.OnQuery("first", nil)
	value, err := app.Resolve(context.Background(), event("extra", nil))
	if err != nil || value != "extra" {
		t.Fatalf("reentrant registration: %v %v", value, err)
	}
	value, err = app.Resolve(context.Background(), event("panic", nil))
	if err != nil || value.(map[string]any)["error"] != "Error - diagnostic failed" {
		t.Fatalf("diagnostic fallback: %v %v", value, err)
	}
}

func TestAggregateNativeArrayAndUUID(t *testing.T) {
	app := New(quiet())
	app.OnBatchQuery("first", func(context.Context, any, any) (any, error) { return []byte{1, 2}, nil })
	value, err := app.Resolve(context.Background(), []any{event("first", nil)})
	if err != nil || !reflect.DeepEqual(jsonValue(t, value), []any{float64(1), float64(2)}) {
		t.Fatalf("not a JSON array: %v %v", value, err)
	}
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for range 128 {
		id, err := MakeID()
		if err != nil || !pattern.MatchString(id) || seen[id] {
			t.Fatal(fmt.Sprint("invalid UUID: ", id, err))
		}
		seen[id] = true
	}
}
