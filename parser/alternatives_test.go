package parser_test

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

type aggregatingLeaf struct{ ordinary, safe int }

func (s *aggregatingLeaf) Validate(_ context.Context, input any) (string, []parser.Issue, error) {
	s.ordinary++
	return input.(string), nil, nil
}
func (s *aggregatingLeaf) ValidateSafe(_ context.Context, input any) (string, []parser.Issue, error) {
	s.safe++
	return input.(string), []parser.Issue{{Code: "custom", Message: "first", Path: []any{"first"}, Continuable: true}, {Code: "custom", Message: "second", Path: []any{"second"}, Continuable: true}}, nil
}

func TestCompositeSafeMode(t *testing.T) {
	cases := []struct {
		name   string
		build  func(parser.Schema[any]) parser.Schema[any]
		input  any
		count  int
		prefix []any
	}{
		{"nullable", func(s parser.Schema[any]) parser.Schema[any] { return parser.Nullable(s) }, "x", 2, nil},
		{"union", func(s parser.Schema[any]) parser.Schema[any] { return parser.Union(s, parser.Number()) }, "x", 2, nil},
		{"object", func(s parser.Schema[any]) parser.Schema[any] {
			return parser.Object(parser.Field{Name: "value", Schema: s})
		}, map[string]any{"value": "x"}, 2, []any{"value"}},
		{"array", func(s parser.Schema[any]) parser.Schema[any] { return parser.Any(parser.Array(s)) }, []any{"x"}, 2, []any{0}},
		{"dictionary", parser.Dictionary, map[string]any{"key": "x"}, 2, []any{"key"}},
		{"json", func(s parser.Schema[any]) parser.Schema[any] { return parser.JSONStringified(s) }, `"x"`, 2, nil},
		{"base64", func(s parser.Schema[any]) parser.Schema[any] { return parser.Base64Encoded(s) }, base64.StdEncoding.EncodeToString([]byte(`"x"`)), 2, nil},
		{"nested", func(s parser.Schema[any]) parser.Schema[any] {
			return parser.JSONStringified(parser.Nullable(parser.Union(parser.Any(parser.Array(s)), parser.Number())))
		}, `["x","y"]`, 4, []any{0}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			leaf := &aggregatingLeaf{}
			schema := item.build(parser.Any[string](leaf))
			safe, err := parser.SafeParse(context.Background(), item.input, schema)
			if err != nil || safe.Success || safe.Data != nil || safe.Error == nil || len(safe.Error.Issues) != item.count || leaf.ordinary != 0 || leaf.safe != item.count/2 {
				t.Fatalf("safe result=%+v leaf=%+v err=%v", safe, leaf, err)
			}
			expected := append(append([]any{}, item.prefix...), "first")
			if !reflect.DeepEqual(safe.Error.Issues[0].Path, expected) || !reflect.DeepEqual(safe.OriginalEvent, item.input) {
				t.Fatalf("paths/input changed: %+v", safe)
			}
			_, err = parser.Parse(context.Background(), item.input, schema)
			if err != nil || leaf.ordinary != item.count/2 {
				t.Fatalf("ordinary mode: %+v, %v", leaf, err)
			}
		})
	}
}

func TestNestedErrorOwnershipAndPrefix(t *testing.T) {
	original := []parser.Issue{{Code: "invalid_union", Message: "nested", Path: []any{"value"}, Errors: [][]parser.Issue{{{Code: "custom", Message: "leaf", Path: []any{"inner"}}}}}}
	leaf := parser.SchemaFunc[any](func(context.Context, any) (any, []parser.Issue, error) { return nil, original, nil })
	schema := parser.Object(parser.Field{Name: "payload", Schema: parser.Union(leaf, parser.String())})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			safe, err := parser.SafeParse(context.Background(), map[string]any{"payload": false}, schema)
			if err != nil || safe.Error == nil {
				t.Errorf("missing error: %v", err)
				return
			}
			issue := safe.Error.Issues[0]
			if !reflect.DeepEqual(issue.Path, []any{"payload"}) || !reflect.DeepEqual(issue.Errors[0][0].Path, []any{"value"}) || !reflect.DeepEqual(issue.Errors[0][0].Errors[0][0].Path, []any{"inner"}) {
				t.Errorf("branch-relative paths lost: %+v", issue)
				return
			}
			issue.Errors[0][0].Errors[0][0].Path[0] = "changed"
		}()
	}
	wg.Wait()
	if original[0].Errors[0][0].Path[0] != "inner" {
		t.Fatal("shared error tree was mutated")
	}
}

func TestUnionOperationalContracts(t *testing.T) {
	ctx := context.Background()
	for _, schema := range []parser.Schema[any]{parser.Union(), parser.Union(parser.String(), nil), parser.Nullable[any](nil), parser.Any(parser.Array[any](nil)), parser.Dictionary(nil), parser.JSONStringified[any](nil), parser.Base64Encoded[any](nil)} {
		if _, err := parser.SafeParse(ctx, nil, schema); err == nil {
			t.Fatal("invalid nested schema accepted")
		}
	}
	branches := []parser.Schema[any]{parser.String(), parser.Number()}
	snapshot := parser.Union(branches...)
	branches[0] = nil
	if value, err := parser.Parse(ctx, "x", snapshot); err != nil || value != "x" {
		t.Fatalf("constructor retained caller slice: %v", err)
	}
	sentinel := errors.New("validator failed")
	calls := 0
	failing := parser.SchemaFunc[any](func(context.Context, any) (any, []parser.Issue, error) { return nil, nil, sentinel })
	later := parser.SchemaFunc[any](func(context.Context, any) (any, []parser.Issue, error) { calls++; return "later", nil, nil })
	if _, err := parser.SafeParse(ctx, "x", parser.Union(failing, later)); err != sentinel || calls != 0 {
		t.Fatalf("operational error identity/stop lost: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	defer cancel()
	first := parser.SchemaFunc[any](func(context.Context, any) (any, []parser.Issue, error) { cancel(); return nil, []parser.Issue{}, nil })
	if _, err := parser.Parse(canceled, "x", parser.Union(first, later)); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancellation lost: %v", err)
	}
	empty := parser.SchemaFunc[any](func(context.Context, any) (any, []parser.Issue, error) { return nil, []parser.Issue{}, nil })
	safe, err := parser.SafeParse(ctx, "x", parser.Union(empty, parser.Number()))
	if err != nil || safe.Success || safe.Error == nil || len(safe.Error.Issues[0].Errors) != 2 || safe.Error.Issues[0].Errors[0] == nil {
		t.Fatalf("empty failure became success: %+v %v", safe, err)
	}
	called := false
	refined := parser.Refine(empty, func(any) bool { called = true; return true }, "unused")
	if result, err := parser.SafeParse(ctx, "x", refined); err != nil || result.Success || called {
		t.Fatalf("refinement ran after aborted validation: %+v %v", result, err)
	}
	t.Run("panic", func(t *testing.T) {
		defer func() {
			if recover() != sentinel {
				t.Error("panic identity lost")
			}
		}()
		_, _ = parser.Parse(ctx, "x", parser.Union(parser.SchemaFunc[any](func(context.Context, any) (any, []parser.Issue, error) { panic(sentinel) }), later))
	})
}

func TestNestedParseErrorComposition(t *testing.T) {
	failure := &parser.ParseError{Message: "custom validation", Issues: []parser.Issue{{Code: "custom", Message: "invalid", Path: []any{"inner"}, Continuable: true}}}
	leaf := parser.SchemaFunc[any](func(context.Context, any) (any, []parser.Issue, error) { return nil, nil, failure })
	ctx := context.Background()
	if got, err := parser.Parse(ctx, "fallback", parser.Union(leaf, parser.String())); err != nil || got != "fallback" {
		t.Fatalf("validation error prevented fallback: %v", err)
	}
	called := false
	schema := parser.Refine[any](parser.Object(parser.Field{Name: "nested", Schema: leaf}, parser.Field{Name: "other", Schema: parser.Number()}), func(any) bool { called = true; return true }, "unused")
	result, err := parser.SafeParse(ctx, map[string]any{"nested": "bad", "other": false}, schema)
	if err != nil || result.Success || result.Error == nil || len(result.Error.Issues) != 2 || called {
		t.Fatalf("nested validation errors: %+v, %v", result, err)
	}
	if !reflect.DeepEqual(result.Error.Issues[0].Path, []any{"nested", "inner"}) || !reflect.DeepEqual(result.Error.Issues[1].Path, []any{"other"}) {
		t.Fatalf("nested error paths: %+v", result.Error.Issues)
	}
	if !failure.Issues[0].Continuable || !reflect.DeepEqual(failure.Issues[0].Path, []any{"inner"}) {
		t.Fatal("caller-owned error mutated")
	}
}
