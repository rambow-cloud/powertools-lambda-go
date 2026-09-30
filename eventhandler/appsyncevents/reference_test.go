package appsyncevents

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"unicode/utf16"
)

func normalized(value any) any {
	switch v := value.(type) {
	case string:
		if len(utf16.Encode([]rune(v))) > 1000 {
			return map[string]any{"utf8Bytes": len([]byte(v)), "sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(v)))}
		}
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalized(item)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = normalized(item)
		}
		return out
	}
	return value
}

func jsonValue(t *testing.T, value any) any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func multiset(t *testing.T, values []any) []string {
	t.Helper()
	result := []string{}
	for _, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, string(data))
	}
	sort.Strings(result)
	return result
}

func TestTypeScriptReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Steps []struct {
				Type, Kind, Path, Mode, Tag, Character string
				Aggregate                              bool
				Length                                 int
				Event                                  any
			}
			Warn    bool
			Results []any
			Calls   []any
			Logs    [][]any
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 100 {
		t.Fatalf("incomplete fixture: %d", len(fixture.Cases))
	}
	for index, item := range fixture.Cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			var mu sync.Mutex
			calls, logs, results := []any{}, [][]any{}, []any{}
			app := New(Options{WarnOnLargePayload: item.Warn, Diagnostic: func(ctx context.Context, level, message string, err error) {
				entry := []any{level, message}
				if err != nil {
					entry = append(entry, errorResponse(err)["error"])
				}
				mu.Lock()
				logs = append(logs, entry)
				mu.Unlock()
			}})
			type requestKey struct{}
			ctx := context.WithValue(context.Background(), requestKey{}, "request")
			for _, step := range item.Steps {
				if step.Type == "register" {
					handler := func(ctx context.Context, payload any, event Event) (any, error) {
						entry := map[string]any{"tag": step.Tag, "payload": normalized(payload), "path": event["info"].(map[string]any)["channel"].(map[string]any)["path"], "context": ctx.Value(requestKey{})}
						mu.Lock()
						calls = append(calls, entry)
						mu.Unlock()
						switch step.Mode {
						case "error":
							return nil, &NamedError{Name: "TypeError", Message: "handler failed"}
						case "unauthorized":
							return nil, &UnauthorizedError{Message: "denied"}
						case "unknown":
							panic(42)
						case "undefined":
							return Undefined{}, nil
						case "null":
							return nil, nil
						case "large":
							return strings.Repeat(step.Character, step.Length), nil
						case "largeAggregate":
							return []any{map[string]any{"id": "one", "payload": strings.Repeat(step.Character, step.Length)}}, nil
						case "tag":
							return map[string]any{"tag": step.Tag, "value": payload}, nil
						default:
							return payload, nil
						}
					}
					if step.Kind == "subscribe" {
						app.OnSubscribe(step.Path, func(ctx context.Context, event Event) error { _, err := handler(ctx, nil, event); return err })
					} else {
						app.OnPublish(step.Path, handler, PublishOptions{Aggregate: step.Aggregate})
					}
				} else {
					value, failure := app.Resolve(ctx, step.Event)
					var message any
					if failure != nil {
						message = errorResponse(failure)["error"]
					}
					results = append(results, map[string]any{"value": normalized(value), "error": message})
				}
			}
			if got := jsonValue(t, results); !reflect.DeepEqual(got, jsonValue(t, item.Results)) {
				t.Fatalf("results: %v; want %v", got, item.Results)
			}
			if !reflect.DeepEqual(multiset(t, calls), multiset(t, item.Calls)) {
				t.Fatalf("calls: %v; want %v", calls, item.Calls)
			}
			split := func(entries [][]any) ([]any, []any) {
				ordered, concurrent := []any{}, []any{}
				for _, entry := range entries {
					if entry[0] == "error" {
						concurrent = append(concurrent, entry)
					} else {
						ordered = append(ordered, entry)
					}
				}
				return ordered, concurrent
			}
			ordered, concurrent := split(logs)
			wantOrdered, wantConcurrent := split(item.Logs)
			if !reflect.DeepEqual(jsonValue(t, ordered), jsonValue(t, wantOrdered)) || !reflect.DeepEqual(multiset(t, concurrent), multiset(t, wantConcurrent)) {
				t.Fatalf("logs: %v; want %v", logs, item.Logs)
			}
		})
	}
}
