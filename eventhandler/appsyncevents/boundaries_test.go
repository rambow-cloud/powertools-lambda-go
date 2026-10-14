package appsyncevents

import (
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"errors"
	"math"
	"strings"
	"testing"
)

type boundaryJSONError struct{ cause error }

func (v boundaryJSONError) MarshalJSON() ([]byte, error) { return nil, v.cause }

func TestNativeOutputBoundaries(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	cause := errors.New("native marshaler failed")
	for _, test := range []struct {
		name  string
		value any
		wire  string
		bad   bool
	}{
		{"null", nil, "null", false},
		{"false", false, "false", false},
		{"zero", 0, "0", false},
		{"empty string", "", `""`, false},
		{"nil typed slice", []int(nil), "[]", false},
		{"nil typed map", map[string]int(nil), "{}", false},
		{"binary", []byte{1, 2}, `"AQI="`, false},
		{"raw integer", jsonv1.RawMessage(`9007199254740993`), "9007199254740993", false},
		{"unicode", map[string]any{"é": "😀\u2028<"}, `{"é":"😀` + "\u2028" + `<"}`, false},
		{"nan", math.NaN(), "", true},
		{"infinity", math.Inf(1), "", true},
		{"invalid utf8", string([]byte{0xff}), "", true},
		{"duplicate raw member", jsonv1.RawMessage(`{"a":1,"a":2}`), "", true},
		{"cycle", cycle, "", true},
		{"unsupported", make(chan int), "", true},
		{"marshaler failure", boundaryJSONError{cause}, "", true},
	} {
		for _, warn := range []bool{false, true} {
			name := test.name + "/plain"
			if warn {
				name = test.name + "/size-check"
			}
			t.Run(name, func(t *testing.T) {
				ctx := context.WithValue(context.Background(), struct{}{}, "invocation")
				var diagnosed error
				app := New(Options{WarnOnLargePayload: warn, Diagnostic: func(got context.Context, level, _ string, err error) {
					if level == "error" {
						if got != ctx {
							t.Error("serialization diagnostic lost invocation context")
						}
						diagnosed = err
					}
				}})
				app.OnPublish("/*", func(context.Context, any, Event) (any, error) { return test.value, nil })
				value, err := app.Resolve(ctx, event("/native", "PUBLISH", "input"))
				if err != nil {
					t.Fatal(err)
				}
				wire, encodeErr := json.Marshal(value, json.Deterministic(true))
				if test.bad && !warn {
					if encodeErr == nil || diagnosed != nil {
						t.Fatalf("plain resolver must leave native encoding to the caller: wire=%s diagnostic=%v", wire, diagnosed)
					}
					return
				}
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				if test.bad {
					item := value.(map[string]any)["events"].([]any)[0].(map[string]any)
					if item["id"] != "0" || !strings.HasPrefix(item["error"].(string), "TypeError - ") || diagnosed == nil {
						t.Fatalf("bad native value was not an item error: %s, %v", wire, diagnosed)
					}
					var named *NamedError
					if !errors.As(diagnosed, &named) || named.Cause == nil {
						t.Fatalf("native encoding cause lost: %v", diagnosed)
					}
					if test.name == "marshaler failure" && !errors.Is(diagnosed, cause) {
						t.Fatal("custom marshaler cause lost")
					}
					return
				}
				want := `{"events":[{"id":"0","payload":` + test.wire + `}]}`
				if string(wire) != want || diagnosed != nil {
					t.Fatalf("wire=%s, want=%s, diagnostic=%v", wire, want, diagnosed)
				}
			})
		}
	}
}

func TestAggregateNativeArraySizeFailure(t *testing.T) {
	cause := errors.New("aggregate marshaler failed")
	var diagnosed error
	app := New(Options{WarnOnLargePayload: true, Diagnostic: func(_ context.Context, level, _ string, err error) {
		if level == "error" {
			diagnosed = err
		}
	}})
	app.OnPublish("/*", func(context.Context, any, Event) (any, error) {
		return [2]any{map[string]any{"id": "valid"}, boundaryJSONError{cause}}, nil
	}, PublishOptions{Aggregate: true})
	value, err := app.Resolve(context.Background(), event("/aggregate", "PUBLISH", "input"))
	if err != nil || !errors.Is(diagnosed, cause) {
		t.Fatalf("aggregate failure=%v, diagnostic=%v", err, diagnosed)
	}
	if _, partial := value.(map[string]any)["events"]; partial {
		t.Fatal("aggregate failure returned partial events")
	}
	if !strings.HasPrefix(value.(map[string]any)["error"].(string), "TypeError - ") {
		t.Fatalf("wrong aggregate error: %v", value)
	}
}

func TestTypedCollectionsFollowDecodedEventContract(t *testing.T) {
	input := event("/typed", "PUBLISH", "input")
	input["events"] = []map[string]any{{"id": "one", "payload": "input"}}
	app := New(quiet())
	app.OnPublish("/*", func(context.Context, any, Event) (any, error) {
		t.Error("native typed message slice is not a decoded JSON publication")
		return nil, nil
	})
	called := false
	app.OnSubscribe("/*", func(_ context.Context, got Event) error {
		called = true
		if got["events"].([]map[string]any)[0]["id"] != "one" {
			t.Fatal("fallback lost the original event")
		}
		return nil
	})
	value, err := app.Resolve(context.Background(), input)
	if value != nil || err != nil || !called {
		t.Fatalf("malformed-publish subscription fallback: %v, %v, %v", value, err, called)
	}
}
