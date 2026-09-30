package datamasking

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type providerFuncs struct {
	encrypt func(context.Context, string, map[string]string) (string, error)
	decrypt func(context.Context, string, map[string]string) (string, error)
}

func (p providerFuncs) Encrypt(ctx context.Context, data string, aad map[string]string) (string, error) {
	return p.encrypt(ctx, data, aad)
}
func (p providerFuncs) Decrypt(ctx context.Context, data string, aad map[string]string) (string, error) {
	return p.decrypt(ctx, data, aad)
}

func normalize(value any) any {
	switch v := value.(type) {
	case Undefined:
		return map[string]any{"$": "undefined"}
	case map[string]any:
		result := map[string]any{}
		for key, item := range v {
			result[key] = normalize(item)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = normalize(item)
		}
		return result
	}
	return value
}

func TestTypeScriptReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	type call struct {
		Method, Data string
		Context      map[string]string
	}
	var corpus struct {
		Version      string
		AsyncFailure []string
		Cases        []struct {
			Name, Method, Data string
			Options            struct {
				Fields      []string
				Rules       []FieldRule
				CustomMask  *string
				DynamicMask *bool
				Context     map[string]string
			}
			IgnoreMissing, Provider bool
			Value                   any
			Error                   *struct{ Name, Message string }
			Warnings                []string
			Calls                   []call
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != "2.35.0" || len(corpus.Cases) != 240 {
		t.Fatal("unexpected corpus")
	}
	if !reflect.DeepEqual(corpus.AsyncFailure, []string{`start:"reject"`, `start:"pending"`, "caught:first rejection", "release", "completed"}) {
		t.Fatal("unexpected reference rejection lifecycle", corpus.AsyncFailure)
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			calls, warnings := []call{}, []string{}
			var mu sync.Mutex
			config := Config{IgnoreMissing: tc.IgnoreMissing, Warn: func(_ context.Context, message string) { warnings = append(warnings, message) }}
			if tc.Provider {
				config.Provider = providerFuncs{
					encrypt: func(_ context.Context, data string, aad map[string]string) (string, error) {
						mu.Lock()
						calls = append(calls, call{"encrypt", data, aad})
						mu.Unlock()
						if strings.Contains(data, "reject") {
							return "", errors.New("provider rejected")
						}
						return "cipher:" + data, nil
					},
					decrypt: func(_ context.Context, data string, aad map[string]string) (string, error) {
						mu.Lock()
						calls = append(calls, call{"decrypt", data, aad})
						mu.Unlock()
						if !strings.HasPrefix(data, "cipher:") {
							return "", errors.New("invalid test ciphertext")
						}
						return strings.TrimPrefix(data, "cipher:"), nil
					},
				}
			}
			masker := New(config)
			var result any
			var err error
			ctx := context.Background()
			input := json.RawMessage(tc.Data)
			switch tc.Method {
			case "erase":
				result, err = masker.Erase(ctx, input, EraseOptions{Fields: tc.Options.Fields, Rules: tc.Options.Rules, Rule: Rule{CustomMask: tc.Options.CustomMask, DynamicMask: tc.Options.DynamicMask}})
			case "encrypt":
				result, err = masker.Encrypt(ctx, input, TransformOptions{Fields: tc.Options.Fields, Context: tc.Options.Context})
			case "decrypt":
				result, err = masker.Decrypt(ctx, input, TransformOptions{Fields: tc.Options.Fields, Context: tc.Options.Context})
			default:
				t.Fatal("unknown method")
			}
			if tc.Error != nil {
				name := "Error"
				var named interface{ ErrorName() string }
				if errors.As(err, &named) {
					name = named.ErrorName()
				}
				if err == nil || name != tc.Error.Name || err.Error() != tc.Error.Message {
					t.Fatalf("got %s: %v; want %s: %s", name, err, tc.Error.Name, tc.Error.Message)
				}
			} else if err != nil || !reflect.DeepEqual(normalize(result), tc.Value) {
				t.Fatalf("got %#v (%v); want %#v", normalize(result), err, tc.Value)
			}
			if !reflect.DeepEqual(warnings, tc.Warnings) {
				t.Fatalf("warnings %#v; want %#v", warnings, tc.Warnings)
			}
			// Go providers start concurrently; compare the complete call multiset.
			canonical := func(items []call) []string {
				result := make([]string, len(items))
				for i, item := range items {
					data, _ := json.Marshal(item)
					result[i] = string(data)
				}
				slices.Sort(result)
				return result
			}
			if !reflect.DeepEqual(canonical(calls), canonical(tc.Calls)) {
				t.Fatalf("calls %#v; want %#v", calls, tc.Calls)
			}
		})
	}
	t.Logf("Verified %d actual Data Masking scenarios", len(corpus.Cases))
}

func TestIsolationCancellationAndReplacement(t *testing.T) {
	ctx := context.Background()
	masker := New(Config{})
	input := map[string]any{"secret": []any{"one", "two"}}
	dynamic := true
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			value, err := masker.Erase(ctx, input, EraseOptions{Rule: Rule{DynamicMask: &dynamic}})
			if err != nil {
				t.Error(err)
				return
			}
			value.(map[string]any)["secret"].([]any)[0] = "mutated"
		})
	}
	wg.Wait()
	if input["secret"].([]any)[0] != "one" {
		t.Fatal("input was mutated")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := masker.Erase(canceled, input, EraseOptions{}); err != context.Canceled {
		t.Fatal(err)
	}
	sentinel := errors.New("replacement failed")
	if _, err := masker.Erase(ctx, "value", EraseOptions{Rule: Rule{Replace: func(string) (string, error) { return "", sentinel }}}); err != sentinel {
		t.Fatal("replacement error identity lost", err)
	}
	if _, err := masker.Erase(ctx, func() {}, EraseOptions{Fields: []string{}}); err == nil {
		t.Fatal("unsupported input accepted")
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	if value, err := masker.Erase(ctx, cycle, EraseOptions{}); err != nil || value != DefaultMask {
		t.Fatal("whole erase unnecessarily cloned a cycle", value, err)
	}
	if value, err := masker.Erase(ctx, []any{func() {}}, EraseOptions{}); err != nil || !reflect.DeepEqual(value, []any{DefaultMask}) {
		t.Fatal("whole array erase inspected an unsupported element", value, err)
	}
}

func TestProviderConcurrencyContextAndOwnership(t *testing.T) {
	type key struct{}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), key{}, "invocation"), 3*time.Second)
	defer cancel()
	started := make(chan struct{}, 2)
	proceed := make(chan struct{})
	input := map[string]any{"first": "one", "second": "two"}
	aad := map[string]string{"tenant": "example"}
	provider := providerFuncs{encrypt: func(ctx context.Context, value string, context map[string]string) (string, error) {
		if ctx.Value(key{}) != "invocation" || context["tenant"] != "example" {
			t.Error("provider context changed")
		}
		context["tenant"] = "mutated"
		started <- struct{}{}
		select {
		case <-proceed:
			return "cipher:" + value, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	go func() {
		for range 2 {
			select {
			case <-started:
			case <-ctx.Done():
				return
			}
		}
		close(proceed)
	}()
	value, err := New(Config{Provider: provider}).Encrypt(ctx, input, TransformOptions{Fields: []string{"*"}, Context: aad})
	if err != nil {
		t.Fatal("provider operations did not overlap", err)
	}
	if !reflect.DeepEqual(value, map[string]any{"first": `cipher:"one"`, "second": `cipher:"two"`}) || input["first"] != "one" || aad["tenant"] != "example" {
		t.Fatal("result or caller ownership changed", value, input, aad)
	}
	sentinel := errors.New("provider failure")
	provider.encrypt = func(context.Context, string, map[string]string) (string, error) { return "", sentinel }
	if _, err := New(Config{Provider: provider}).Encrypt(ctx, input, TransformOptions{Fields: []string{"*"}}); err != sentinel {
		t.Fatal("provider error identity changed", err)
	}
	marker := &struct{ reason string }{"provider panic"}
	provider.encrypt = func(context.Context, string, map[string]string) (string, error) { panic(marker) }
	func() {
		defer func() {
			if recovered := recover(); recovered != marker {
				t.Errorf("provider panic identity changed: %#v", recovered)
			}
		}()
		_, _ = New(Config{Provider: provider}).Encrypt(ctx, input, TransformOptions{Fields: []string{"*"}})
	}()
}

func TestFirstRejectionDoesNotWaitForSiblingProvider(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan struct{}, 2)
	reject := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	defer close(release)
	sentinel := errors.New("first rejection")
	provider := providerFuncs{encrypt: func(ctx context.Context, data string, _ map[string]string) (string, error) {
		started <- struct{}{}
		if data == `"reject"` {
			<-reject
			return "", sentinel
		}
		select {
		case <-release:
			close(done)
			return "cipher:" + data, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	result := make(chan error, 1)
	go func() {
		_, err := New(Config{Provider: provider}).Encrypt(ctx, map[string]any{"a": "reject", "b": "pending"}, TransformOptions{Fields: []string{"*"}})
		result <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("provider did not start")
		}
	}
	close(reject)
	select {
	case err := <-result:
		if err != sentinel {
			t.Fatal("first error identity changed", err)
		}
	case <-ctx.Done():
		t.Fatal("first rejection waited for a pending sibling")
	}
	select {
	case <-done:
		t.Fatal("pending provider completed before release")
	default:
	}
}
