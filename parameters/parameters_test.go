package parameters_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

func TestReferenceFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name    string
			Options struct {
				Transform                  parameters.Transform
				ForceFetch, ThrowOnMissing bool
			}
			Value any
			Error string
			Calls int
		}
		Multiple    map[string]any
		StrictError string
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	cache := parameters.NewCache(nil)
	store := map[string]any{"plain": "hello", "json": `{"enabled":true,"count":2}`, "binary": "aGVsbG8=", "empty": "", "null": "null", "invalid": "{"}
	calls := 0
	for _, test := range fixture.Cases {
		options := parameters.Options{Transform: test.Options.Transform, ForceFetch: test.Options.ForceFetch, ThrowOnMissing: test.Options.ThrowOnMissing}
		value, err := cache.Get(context.Background(), test.Name, options, func(context.Context) (any, error) { calls++; return store[test.Name], nil })
		if errorName(err) != test.Error || !reflect.DeepEqual(value, test.Value) || calls != test.Calls {
			t.Fatalf("%s: value=%#v error=%v calls=%d; expected %#v %s %d", test.Name, value, err, calls, test.Value, test.Error, test.Calls)
		}
	}
	fetch := func(context.Context) (map[string]any, error) {
		return map[string]any{"flags.JSON": `{"enabled":true}`, "text.binary": "aGVsbG8=", "plain": "untouched", "broken.json": "{"}, nil
	}
	value, err := cache.GetMultiple(context.Background(), "/fixture", parameters.Options{Transform: parameters.Auto}, fetch)
	if err != nil || !reflect.DeepEqual(value, fixture.Multiple) {
		t.Fatalf("multiple: %#v, %v", value, err)
	}
	_, err = cache.GetMultiple(context.Background(), "/fixture", parameters.Options{Transform: parameters.Auto, ForceFetch: true, ThrowOnTransformError: true}, fetch)
	if errorName(err) != fixture.StrictError {
		t.Fatal(err)
	}
}

func errorName(err error) string {
	if err == nil {
		return ""
	}
	var missing *parameters.ParameterNotFoundError
	var transform *parameters.TransformParameterError
	if errors.As(err, &missing) {
		return "ParameterNotFoundError"
	}
	if errors.As(err, &transform) {
		return "TransformParameterError"
	}
	return "GetParameterError"
}

func TestCacheExpiryEnvironmentAndIsolation(t *testing.T) {
	now := time.Unix(100, 0)
	cache := parameters.NewCache(func() time.Time { return now })
	calls := 0
	fetch := func(context.Context) (any, error) { calls++; return `{"nested":[{"value":1}]}`, nil }
	get := func(options parameters.Options) any {
		t.Helper()
		value, err := cache.Get(context.Background(), "config", options, fetch)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	t.Setenv("POWERTOOLS_PARAMETERS_MAX_AGE", "2")
	opts := parameters.Options{Transform: parameters.JSON}
	first := get(opts).(map[string]any)
	first["nested"].([]any)[0].(map[string]any)["value"] = float64(99)
	now = now.Add(2 * time.Second)
	second := get(opts).(map[string]any)
	if calls != 1 || second["nested"].([]any)[0].(map[string]any)["value"] != float64(1) {
		t.Fatalf("cache boundary or snapshot: %v, %d", second, calls)
	}
	now = now.Add(time.Millisecond)
	get(opts)
	if calls != 2 {
		t.Fatal("expired entry reused")
	}
	opts.ForceFetch = true
	get(opts)
	if calls != 3 {
		t.Fatal("force fetch ignored")
	}
	cache.ClearCache()
	opts.ForceFetch = false
	opts.MaxAge = parameters.Age(0)
	get(opts)
	get(opts)
	if calls != 5 {
		t.Fatal("zero age cached")
	}
	opts.MaxAge = nil
	t.Setenv("POWERTOOLS_PARAMETERS_MAX_AGE", "invalid")
	if opts.Lifetime() != 5*time.Second {
		t.Fatal("invalid environment default")
	}
	t.Setenv("POWERTOOLS_PARAMETERS_MAX_AGE", "0")
	get(opts)
	get(opts)
	if calls != 7 {
		t.Fatal("environment zero age cached")
	}
}

func TestCacheMissingErrorsAndOperationKeys(t *testing.T) {
	cache := parameters.NewCache(nil)
	calls := 0
	missing := func(context.Context) (any, error) { calls++; return nil, nil }
	for range 2 {
		_, _ = cache.Get(context.Background(), "missing", parameters.Options{}, missing)
	}
	if calls != 2 {
		t.Fatal("missing value cached")
	}
	boom := errors.New("upstream failed")
	_, err := cache.Get(context.Background(), "error", parameters.Options{}, func(context.Context) (any, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Fatal("cause lost")
	}
	_, _ = cache.Get(context.Background(), "same", parameters.Options{}, func(context.Context) (any, error) { return "single", nil })
	for range 2 {
		values, err := cache.GetMultiple(context.Background(), "same", parameters.Options{}, func(context.Context) (map[string]any, error) { calls++; return map[string]any{}, nil })
		if err != nil || len(values) != 0 {
			t.Fatal("single/multiple cache collision")
		}
	}
	if calls != 4 {
		t.Fatal("empty collection cached")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = cache.Get(ctx, "same", parameters.Options{}, missing)
	if !errors.Is(err, context.Canceled) || calls != 4 {
		t.Fatal("cancelled call fetched")
	}
}

func TestClearDuringFetchAndConcurrentSnapshots(t *testing.T) {
	cache := parameters.NewCache(nil)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_, _ = cache.Get(context.Background(), "key", parameters.Options{}, func(context.Context) (any, error) { close(started); <-release; return "old", nil })
	}()
	<-started
	cache.ClearCache()
	close(release)
	<-done
	if _, found := cache.Lookup("key", parameters.Options{}); found {
		t.Fatal("in-flight fetch repopulated cleared cache")
	}
	_, err := cache.Get(context.Background(), "key", parameters.Options{Transform: parameters.JSON}, func(context.Context) (any, error) { return `{"values":[1,2]}`, nil })
	if err != nil {
		t.Fatal(err)
	}
	var failed atomic.Bool
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			value, ok := cache.Lookup("key", parameters.Options{Transform: parameters.JSON})
			if !ok {
				failed.Store(true)
				return
			}
			array := value.(map[string]any)["values"].([]any)
			if array[0] != float64(1) {
				failed.Store(true)
			}
			array[0] = float64(9)
		})
	}
	wg.Wait()
	if failed.Load() {
		t.Fatal("concurrent cache values shared mutable state")
	}
}

func TestTransformCases(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		mode  parameters.Transform
		want  any
		fail  bool
	}{
		{"raw", []byte("data"), "", []byte("data"), false},
		{"mixed.JSON", []byte(`false`), parameters.Auto, false, false},
		{"text", "aGVs bG8\n", parameters.Binary, nil, true},
		{"url", "8J-SqQ", parameters.Binary, nil, true},
		{"object", map[string]any{"a": true}, parameters.JSON, map[string]any{"a": true}, false},
		{"json", "{} trailing", parameters.JSON, nil, true},
		{"binary", "%%%", parameters.Binary, nil, true},
		{"invalid", "x", "unknown", nil, true},
	} {
		value, err := parameters.TransformValue(test.name, test.value, test.mode)
		if (err != nil) != test.fail || !reflect.DeepEqual(value, test.want) {
			t.Errorf("%s: %#v, %v", test.name, value, err)
		}
	}
}
