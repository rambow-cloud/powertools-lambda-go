package parser_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

func TestFailureContracts(t *testing.T) {
	input := map[string]any{"id": "a"}
	empty := parser.SchemaFunc[any](func(context.Context, any) (any, []parser.Issue, error) { return "partial", []parser.Issue{}, nil })
	result, err := parser.SafeParse(context.Background(), input, empty)
	if err != nil || result.Success || result.Error == nil || result.Data != nil {
		t.Fatalf("empty issues must fail: %+v, %v", result, err)
	}
	input["id"] = "changed"
	if result.OriginalEvent.(map[string]any)["id"] != "changed" {
		t.Fatal("original input reference was lost")
	}
	sentinel := errors.New("validator failed")
	broken := parser.SchemaFunc[string](func(context.Context, any) (string, []parser.Issue, error) { return "", nil, sentinel })
	if _, err := parser.SafeParse(context.Background(), input, broken); err != sentinel {
		t.Fatalf("operational error changed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := parser.Parse(ctx, input, empty); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if _, err := parser.Parse[any](context.Background(), input, nil); err == nil {
		t.Fatal("nil schema accepted")
	}
	issues := []parser.Issue{{Message: "invalid", Path: []any{"leaf"}}}
	prefixed := parser.Prefix(issues, "parent", 1)
	prefixed[0].Path[2] = "changed"
	if !reflect.DeepEqual(issues[0].Path, []any{"leaf"}) {
		t.Fatal("prefix mutated original issue")
	}
}

func TestObjectIsolationAndTypedOutput(t *testing.T) {
	defaultValue := map[string]any{"values": []any{"original"}}
	schema := parser.Object(parser.Field{Name: "id", Schema: parser.String()}, parser.Field{Name: "config", Schema: parser.Unknown()}.WithDefault(defaultValue))
	defaultValue["values"].([]any)[0] = "caller mutation"
	input := struct {
		ID string `json:"id"`
	}{ID: "a"}
	first, err := parser.Parse(context.Background(), input, schema)
	if err != nil {
		t.Fatal(err)
	}
	first.(map[string]any)["config"].(map[string]any)["values"].([]any)[0] = "result mutation"
	second, err := parser.Parse(context.Background(), input, schema)
	if err != nil {
		t.Fatal(err)
	}
	if second.(map[string]any)["config"].(map[string]any)["values"].([]any)[0] != "original" {
		t.Fatal("default was shared")
	}
	extended := schema.Extend(parser.Field{Name: "id", Schema: parser.Number()})
	if _, err := parser.Parse(context.Background(), input, extended); err == nil {
		t.Fatal("extension ignored replacement")
	}
	if _, err := parser.Parse(context.Background(), input, schema); err != nil {
		t.Fatalf("extension mutated parent: %v", err)
	}
	type order struct {
		ID     string  `json:"id"`
		Amount float64 `json:"amount"`
		Retry  int     `json:"retry"`
	}
	value, err := parser.Parse(context.Background(), map[string]any{"id": "a", "amount": 2}, parser.Typed[order](orderSchema()))
	if err != nil || value != (order{ID: "a", Amount: 2, Retry: 3}) {
		t.Fatalf("typed output: %+v, %v", value, err)
	}
}

func TestWrappersPreserveHandlerBehavior(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "request")
	sentinel := errors.New("business error")
	calls := 0
	handler := parser.WrapHandler[any, any, string](parser.String(), func(ctx context.Context, value any) (string, error) {
		calls++
		if ctx.Value(key{}) != "request" {
			t.Fatal("context lost")
		}
		return value.(string), sentinel
	})
	if _, err := handler(ctx, 1); err == nil || calls != 0 {
		t.Fatalf("invalid input reached handler: %v/%d", err, calls)
	}
	if value, err := handler(ctx, "ok"); value != "ok" || err != sentinel || calls != 1 {
		t.Fatalf("business result changed: %q/%v/%d", value, err, calls)
	}
	safe := parser.WrapSafeHandler[any, any, bool](parser.String(), func(_ context.Context, result parser.Result[any]) (bool, error) {
		return !result.Success && result.Error != nil && result.OriginalEvent == 1, nil
	})
	if value, err := safe(ctx, 1); !value || err != nil {
		t.Fatalf("safe handler: %v/%v", value, err)
	}
	panicValue := &struct{ Message string }{"panic"}
	panicking := parser.WrapHandler[any, any, string](parser.String(), func(context.Context, any) (string, error) { panic(panicValue) })
	func() {
		defer func() {
			if recovered := recover(); recovered != panicValue {
				t.Errorf("panic identity changed: %v", recovered)
			}
		}()
		_, _ = panicking(ctx, "ok")
	}()
}

func TestConcurrentSchemaReuse(t *testing.T) {
	schema := orderSchema()
	var workers sync.WaitGroup
	for i := range 100 {
		workers.Go(func() {
			id := fmt.Sprint(i)
			input := map[string]any{"id": id, "amount": i, "ignored": true}
			result, err := parser.Parse(context.Background(), input, schema)
			if err != nil {
				t.Error(err)
				return
			}
			value := result.(map[string]any)
			if value["id"] != id || len(value) != 3 {
				t.Errorf("cross-request output: %+v", value)
			}
			if _, ok := input["retry"]; ok {
				t.Error("schema mutated input")
			}
		})
	}
	workers.Wait()
}
