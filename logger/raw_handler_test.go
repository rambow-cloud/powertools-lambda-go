package logger

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-lambda-go/lambda"
)

func exactJSON(t *testing.T, data []byte) any {
	t.Helper()
	decoder := jsonv1.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestWrapRawHandlerRuntimePreservesEvent(t *testing.T) {
	cleanEnv(t)
	var output bytes.Buffer
	l := New(WithOutput(&output))
	enabled := true
	type event struct {
		Name  string  `json:"name"`
		Large int64   `json:"large"`
		Empty string  `json:"empty,omitempty"`
		Null  *string `json:"null,omitempty"`
	}
	payload := []byte(`{"name":"Alice","age":30,"large":9007199254740993,"extra":{"items":[null,"",{"unknown":true}]},"empty":"","null":null}`)
	calls := 0
	handler := WrapRawHandler(l, func(ctx context.Context, input event) (string, error) {
		calls++
		if input.Name != "Alice" || input.Large != 9007199254740993 || input.Empty != "" || input.Null != nil {
			t.Fatalf("typed input: %+v", input)
		}
		got := records(t, &output)
		if len(got) != 1 || got[0]["message"] != "Lambda invocation event" {
			t.Fatal("event was not logged before the business handler", got)
		}
		return input.Name, l.WithContext(ctx).Info("business")
	}, HandlerOptions{LogEvent: &enabled})
	response, err := lambda.NewHandler(handler).Invoke(context.Background(), payload)
	if err != nil || string(response) != `"Alice"` || calls != 1 {
		t.Fatal(string(response), err, calls)
	}
	decoder := jsonv1.NewDecoder(&output)
	var entry struct {
		Event jsonv1.RawMessage `json:"event"`
	}
	if err := decoder.Decode(&entry); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exactJSON(t, entry.Event), exactJSON(t, payload)) {
		t.Fatalf("raw event changed: %s", entry.Event)
	}
	var business struct {
		Event   jsonv1.RawMessage `json:"event"`
		Message string            `json:"message"`
	}
	if err := decoder.Decode(&business); err != nil {
		t.Fatal(err)
	}
	if business.Event != nil || business.Message != "business" {
		t.Fatal("event content leaked into the business record", business)
	}
}

func TestWrapRawHandlerJSONShapes(t *testing.T) {
	for _, payload := range []string{`null`, `true`, `false`, `"text"`, `9007199254740993`, `0.1234567890123456789`, `[]`, `[null,"",{"age":30}]`, `{}`} {
		t.Run(payload, func(t *testing.T) {
			cleanEnv(t)
			var output bytes.Buffer
			enabled := true
			l := New(WithOutput(&output))
			h := WrapRawHandler(l, func(_ context.Context, input jsonv1.RawMessage) (bool, error) {
				if !reflect.DeepEqual(exactJSON(t, input), exactJSON(t, []byte(payload))) {
					t.Fatal("raw business input changed", string(input))
				}
				return true, nil
			}, HandlerOptions{LogEvent: &enabled})
			if result, err := lambda.NewHandler(h).Invoke(context.Background(), []byte(payload)); err != nil || string(result) != "true" {
				t.Fatal(string(result), err)
			}
			var entry struct {
				Event jsonv1.RawMessage `json:"event"`
			}
			if err := jsonv1.Unmarshal(output.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(exactJSON(t, entry.Event), exactJSON(t, []byte(payload))) {
				t.Fatal("logged JSON value changed", output.String())
			}
		})
	}
}

func TestWrapRawHandlerDecodeFailures(t *testing.T) {
	for _, payload := range []string{`{"name":123}`, `{`, `{"name":`} {
		t.Run(payload, func(t *testing.T) {
			cleanEnv(t)
			var output bytes.Buffer
			enabled := true
			calls := 0
			h := WrapRawHandler(New(WithOutput(&output)), func(_ context.Context, input struct {
				Name string `json:"name"`
			}) (int, error) { calls++; return 1, nil }, HandlerOptions{LogEvent: &enabled})
			result, err := lambda.NewHandler(h).Invoke(context.Background(), []byte(payload))
			if err == nil || len(result) != 0 || calls != 0 {
				t.Fatal(string(result), err, calls)
			}
			if payload == `{"name":123}` {
				var semantic *json.SemanticError
				if !errors.As(err, &semantic) || len(records(t, &output)) != 1 {
					t.Fatal("valid JSON must be logged before typed decoding fails", err, output.String())
				}
			} else if output.Len() != 0 {
				t.Fatal("runtime-rejected syntax reached event logging", output.String())
			}
		})
	}
}

func TestWrapRawHandlerLoggingPolicies(t *testing.T) {
	for _, mode := range []string{"redact", "formatter", "disabled", "level", "environment"} {
		t.Run(mode, func(t *testing.T) {
			cleanEnv(t)
			var output bytes.Buffer
			options := []Option{WithOutput(&output)}
			enabled := mode != "disabled"
			handlerOptions := HandlerOptions{LogEvent: &enabled}
			if mode == "redact" {
				options = append(options, WithReplacer(func(key string, value any) any {
					if key == "password" {
						return "[REDACTED]"
					}
					return value
				}))
			}
			if mode == "formatter" {
				options = append(options, WithFormatter(func(fields Fields) (any, error) {
					return Fields{"incoming": fields["event"]}, nil
				}))
			}
			if mode == "level" {
				options = append(options, WithLevel(WarnLevel))
			}
			if mode == "disabled" || mode == "environment" {
				t.Setenv("POWERTOOLS_LOGGER_LOG_EVENT", "true")
			}
			if mode == "environment" {
				handlerOptions.LogEvent = nil
			}
			l := New(options...)
			h := WrapRawHandler(l, func(_ context.Context, _ struct{}) (int, error) { return 7, nil }, handlerOptions)
			if result, err := h(context.Background(), jsonv1.RawMessage(`{"password":"secret","large":9007199254740993}`)); result != 7 || err != nil {
				t.Fatal(result, err)
			}
			if mode == "disabled" || mode == "level" {
				if output.Len() != 0 {
					t.Fatal(output.String())
				}
			} else {
				if !strings.Contains(output.String(), `9007199254740993`) {
					t.Fatal(output.String())
				}
				if mode == "redact" && (strings.Contains(output.String(), "secret") || !strings.Contains(output.String(), "[REDACTED]")) {
					t.Fatal(output.String())
				}
				if mode == "formatter" && !strings.Contains(output.String(), `"incoming":{`) {
					t.Fatal(output.String())
				}
			}
		})
	}
}

func TestWrapRawHandlerCorrelationAndNestedLifetime(t *testing.T) {
	for _, outcome := range []string{"success", "decode", "error", "panic", "cancel"} {
		t.Run(outcome, func(t *testing.T) {
			cleanEnv(t)
			var output bytes.Buffer
			l := New(WithOutput(&output), WithBuffer(BufferOptions{}))
			enabled, disabled := true, false
			business := errors.New("business failure")
			panicValue := &struct{}{}
			var retained *Logger
			calls := 0
			inner := WrapRawHandler(l, func(ctx context.Context, _ struct {
				Name string `json:"name"`
			}) (int, error) { calls++; if l.WithContext(ctx).GetCorrelationID() != "raw-id" {
				t.Fatal("raw-only correlation member lost")
			}; if err := l.WithContext(ctx).Debug("detail"); err != nil {
				return 0, err
			}; switch outcome {
			case "panic":
				panic(panicValue)
			case "error":
				return 9, business
			case "cancel":
				return 9, ctx.Err()
			}; return 9, nil }, HandlerOptions{LogEvent: &enabled, CorrelationSource: EventBridge})
			outer := WrapHandler(l, func(ctx context.Context, raw jsonv1.RawMessage) (int, error) {
				retained = l.WithContext(ctx)
				result, err := inner(ctx, raw)
				if lateErr := retained.Info("after inner"); lateErr != nil {
					t.Fatal("inner closed shared scope", lateErr)
				}
				return result, err
			}, HandlerOptions{LogEvent: &disabled, FlushBufferOnError: true})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if outcome == "cancel" {
				cancel()
			}
			payload := jsonv1.RawMessage(`{"name":"Alice","id":"raw-id"}`)
			if outcome == "decode" {
				payload = jsonv1.RawMessage(`{"name":123,"id":"raw-id"}`)
			}
			var result int
			var err error
			func() {
				defer func() {
					failure := recover()
					if (outcome == "panic" && failure != panicValue) || (outcome != "panic" && failure != nil) {
						t.Fatalf("panic changed: %v", failure)
					}
				}()
				result, err = outer(ctx, payload)
			}()
			if outcome == "decode" {
				if err == nil || result != 0 || calls != 0 {
					t.Fatal(result, err, calls)
				}
			} else if calls != 1 {
				t.Fatal("business call count", calls)
			}
			if outcome == "error" && (err != business || result != 9) {
				t.Fatal(result, err)
			}
			if outcome == "cancel" && (err != context.Canceled || result != 9) {
				t.Fatal(result, err)
			}
			if outcome == "success" && (err != nil || result != 9) {
				t.Fatal(result, err)
			}
			if !errors.Is(retained.Info("late"), ErrInvocationClosed) || !errors.Is(retained.FlushBuffer(), ErrInvocationClosed) {
				t.Fatal("invocation state remained open")
			}
			got := records(t, &output)
			if got[0]["correlation_id"] != "raw-id" {
				t.Fatal(got)
			}
			if outcome != "success" && got[len(got)-1]["message"] != "Uncaught error detected, flushing log buffer before exit" {
				t.Fatal("failure did not flush", got)
			}
		})
	}
}

func TestWrapRawHandlerConcurrentInvocations(t *testing.T) {
	cleanEnv(t)
	var output bytes.Buffer
	l := New(WithOutput(&output))
	enabled := true
	h := WrapRawHandler(l, func(ctx context.Context, input struct {
		Name string `json:"name"`
	}) (string, error) { bound := l.WithContext(ctx); bound.AppendKeys(Fields{"owner": input.Name}); return input.Name, bound.Info("business") }, HandlerOptions{LogEvent: &enabled, CorrelationID: func(value any) any {
		raw, ok := value.(jsonv1.RawMessage)
		if !ok {
			t.Error("callback did not receive RawMessage")
			return nil
		}
		var input struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			t.Error(err)
		}
		return input.ID
	}})
	var wg sync.WaitGroup
	for n := range 32 {
		wg.Go(func() {
			name := fmt.Sprint(n)
			payload := jsonv1.RawMessage(fmt.Sprintf(`{"name":%q,"id":%q,"extra":true}`, name, name))
			result, err := h(context.Background(), payload)
			if result != name || err != nil {
				t.Errorf("request %s: %s, %v", name, result, err)
			}
		})
	}
	wg.Wait()
	got := records(t, &output)
	if len(got) != 64 {
		t.Fatal("record count", len(got))
	}
	events, business := map[string]bool{}, map[string]bool{}
	for _, entry := range got {
		id := entry["correlation_id"].(string)
		if entry["message"] == "Lambda invocation event" {
			input := entry["event"].(map[string]any)
			if input["name"] != id || input["extra"] != true || entry["owner"] != nil || events[id] {
				t.Fatal("event isolation failed", entry)
			}
			events[id] = true
		} else {
			if entry["owner"] != id || business[id] {
				t.Fatal("business isolation failed", entry)
			}
			business[id] = true
		}
	}
	if len(events) != 32 || len(business) != 32 {
		t.Fatal(events, business)
	}
}
