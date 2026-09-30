package bedrock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func testEvent() Event {
	return Event{"messageVersion": "1.0", "actionGroup": "orders", "function": "tool", "agent": map[string]any{"name": "agent", "id": "id", "alias": "alias", "version": "1"}, "parameters": []any{map[string]any{"name": "value", "type": "number", "value": "1"}}, "inputText": "input", "sessionId": "session", "sessionAttributes": map[string]any{}, "promptSessionAttributes": map[string]any{}}
}
func quiet() Options { return Options{Diagnostic: func(context.Context, string, string, error) {}} }
func body(value any) any {
	return value.(map[string]any)["response"].(map[string]any)["functionResponse"].(map[string]any)["responseBody"].(map[string]any)["TEXT"].(map[string]any)["body"]
}

func TestConcurrentInvocationOwnership(t *testing.T) {
	type key struct{}
	type invocation struct {
		ctx context.Context
		id  int
	}
	app := New(quiet())
	entered, release := make(chan struct{}, 64), make(chan struct{})
	app.Tool(func(ctx context.Context, params *Parameters, event Event) (any, error) {
		state := ctx.Value(key{}).(*invocation)
		if ctx != state.ctx || ctx.Err() != context.Canceled || event["nonce"] != state.id {
			return nil, errors.New("context or event changed")
		}
		params.Set("value", state.id)
		copy := params.Values()
		copy["value"] = -1
		entered <- struct{}{}
		<-release
		return params, nil
	}, Configuration{Name: "tool"})
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			state := &invocation{id: i}
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key{}, state))
			state.ctx = ctx
			cancel()
			event := testEvent()
			event["nonce"] = i
			before := jsonValue(t, event)
			result, err := app.Resolve(ctx, event)
			if err != nil || body(result) != fmt.Sprintf(`{"value":%d}`, i) {
				t.Errorf("cross-invocation state: %v %v", result, err)
			}
			if !reflect.DeepEqual(before, jsonValue(t, event)) {
				t.Error("resolver mutated caller input")
			}
		})
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for range 64 {
		select {
		case <-entered:
		case <-deadline.C:
			close(release)
			wg.Wait()
			t.Fatal("handler callbacks serialized or context was lost")
		}
	}
	close(release)
	wg.Wait()
}

func TestErrorsSerializationAndReentrantDiagnostics(t *testing.T) {
	marker := errors.New("tool failed")
	var app *Resolver
	var observed error
	app = New(Options{Diagnostic: func(_ context.Context, level, message string, err error) {
		if level == "warn" && strings.HasPrefix(message, "Tool \"tool\"") {
			app.Tool(func(context.Context, *Parameters, Event) (any, error) { return "nested", nil }, Configuration{Name: "nested"})
		}
		if level == "error" {
			observed = err
		}
	}})
	app.Tool(func(context.Context, *Parameters, Event) (any, error) { panic(marker) }, Configuration{Name: "tool"})
	result, err := app.Resolve(context.Background(), testEvent())
	if err != nil || observed != marker || body(result) != "Unable to complete tool execution due to Error - tool failed" {
		t.Fatalf("error identity: %v %v %v", result, err, observed)
	}
	app.Tool(func(context.Context, *Parameters, Event) (any, error) { return func() {}, nil }, Configuration{Name: "tool"})
	event := testEvent()
	event["function"] = "nested"
	result, err = app.Resolve(context.Background(), event)
	if err != nil || body(result) != `"nested"` {
		t.Fatalf("reentrant registration: %v %v", result, err)
	}
	result, err = app.Resolve(context.Background(), testEvent())
	if err != nil || !strings.Contains(body(result).(string), "unsupported type") {
		t.Fatalf("serialization failure escaped: %v %v", result, err)
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	app.Tool(func(context.Context, *Parameters, Event) (any, error) { return cycle, nil }, Configuration{Name: "tool"})
	result, err = app.Resolve(context.Background(), testEvent())
	if err != nil || !strings.Contains(body(result).(string), "TypeError - Converting circular structure to JSON") {
		t.Fatalf("cycle failure: %v %v", result, err)
	}
	_, err = app.Resolve(context.Background(), nil)
	if typed, ok := err.(*Error); !ok || reflect.TypeOf(typed).Elem().Name() != "Error" {
		t.Fatalf("invalid event Runtime API type: %T", err)
	}
}

func TestResponseOwnershipAndParametersOrder(t *testing.T) {
	response := NewFunctionResponse("body")
	attrs := response.SessionAttributes.(map[string]any)
	attrs["one"] = "before"
	wire := response.Build("group", "function")
	attrs["one"] = "after"
	if wire["sessionAttributes"].(map[string]any)["one"] != "after" {
		t.Fatal("response unexpectedly cloned attributes")
	}
	parameters := &Parameters{}
	parameters.Set("b", 1)
	parameters.Set("2", 2)
	parameters.Set("a", 3)
	parameters.Delete("b")
	parameters.Set("b", 4)
	encoded, err := json.Marshal(parameters)
	if err != nil || string(encoded) != `{"2":2,"a":3,"b":4}` {
		t.Fatalf("ordered JSON: %s %v", encoded, err)
	}
	if !parameters.Has("b") || parameters.Get("missing") != nil {
		t.Fatal("parameter access failed")
	}
}

func TestNativeNilReturn(t *testing.T) {
	app := New(quiet())
	for _, value := range []any{(*FunctionResponse)(nil), map[string]any(nil), []any(nil)} {
		app.Tool(func(context.Context, *Parameters, Event) (any, error) { return value, nil }, Configuration{Name: "tool"})
		result, err := app.Resolve(context.Background(), testEvent())
		if err != nil || body(result) != "" {
			t.Fatalf("native null result: %v %v", result, err)
		}
	}
}

func TestDiagnosticPanicPropagation(t *testing.T) {
	marker := errors.New("diagnostic failed")
	app := New(Options{Diagnostic: func(_ context.Context, level, _ string, _ error) {
		if level == "error" {
			panic(marker)
		}
	}})
	app.Tool(func(context.Context, *Parameters, Event) (any, error) { return nil, errors.New("business") }, Configuration{Name: "tool"})
	defer func() {
		if got := recover(); got != marker {
			t.Errorf("diagnostic panic identity: %v", got)
		}
	}()
	_, _ = app.Resolve(context.Background(), testEvent())
	t.Fatal("diagnostic failure was swallowed")
}
