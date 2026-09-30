package commons_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/internal/testfixture"
)

func TestEnvironmentReference(t *testing.T) {
	f := testfixture.Load(t)
	for _, item := range f["env"].([]any) {
		test := item.(map[string]any)
		t.Run(test["mode"].(string)+"/"+stringOrMissing(test["raw"]), func(t *testing.T) {
			t.Setenv("PT_TEST_ENV", "")
			if test["raw"] == nil {
				_ = os.Unsetenv("PT_TEST_ENV")
			} else {
				t.Setenv("PT_TEST_ENV", test["raw"].(string))
			}
			var value any
			var err error
			switch test["mode"] {
			case "string":
				value, err = commons.StringEnv("PT_TEST_ENV")
			case "number":
				value, err = commons.NumberEnv("PT_TEST_ENV")
			case "boolean":
				value, err = commons.BoolEnv("PT_TEST_ENV", false)
			case "extended":
				value, err = commons.BoolEnv("PT_TEST_ENV", true)
			}
			testfixture.AssertOutcome(t, test, value, err)
			if err != nil {
				var env *commons.EnvironmentError
				if !errors.As(err, &env) {
					t.Fatal("untyped environment error")
				}
			}
		})
	}
	for _, item := range f["runtime"].([]any) {
		test := item.(map[string]any)
		t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "")
		if test["initialization"] == nil {
			_ = os.Unsetenv("AWS_LAMBDA_INITIALIZATION_TYPE")
		} else {
			t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", test["initialization"].(string))
		}
		t.Setenv("POWERTOOLS_DEV", test["dev"].(string))
		if commons.IsDevMode() != test["local"] || commons.IsRunningInLambda() != test["lambda"] {
			t.Fatalf("runtime: %v", test)
		}
	}
	t.Setenv("POWERTOOLS_SERVICE_NAME", "  app  ")
	if commons.ServiceName() != "app" || commons.ResolveServiceName("explicit", "fallback") != "explicit" {
		t.Fatal("service precedence")
	}
}
func stringOrMissing(value any) string {
	if value == nil {
		return "missing"
	}
	return value.(string)
}

func TestBase64Reference(t *testing.T) {
	for _, item := range testfixture.Load(t)["base64"].([]any) {
		test := item.(map[string]any)
		var encoding []string
		if test["encoding"] != nil {
			encoding = []string{test["encoding"].(string)}
		}
		value, err := commons.FromBase64(test["input"].(string), encoding...)
		testfixture.AssertOutcome(t, test, value, err)
	}
}

func TestMergeLRUAndTypesReference(t *testing.T) {
	f := testfixture.Load(t)
	source := f["source"].(map[string]any)
	before, _ := json.Marshal(source)
	merged := commons.DeepMerge(map[string]any{"nested": map[string]any{"a": float64(1)}, "array": []any{map[string]any{"a": float64(1)}, float64(2), float64(3)}, "nil": "old"}, source)
	if !reflect.DeepEqual(merged, f["merged"]) {
		t.Fatalf("merge: %v", merged)
	}
	merged["nested"].(map[string]any)["b"] = 99
	after, _ := json.Marshal(source)
	if string(before) != string(after) {
		t.Fatal("source mutated")
	}
	cycle := map[string]any{"value": float64(1)}
	cycle["self"] = cycle
	if got := commons.DeepMerge(nil, cycle); !reflect.DeepEqual(got, f["circular"]) {
		t.Fatal("cycle", got)
	}
	unsafe := commons.DeepMerge(nil, map[string]any{"__proto__": map[string]any{"polluted": true}, "constructor": float64(1), "safe": float64(2)})
	if !reflect.DeepEqual(unsafe, f["unsafe"]) {
		t.Fatal(unsafe)
	}
	lru := commons.NewLRUCache[string, int](2)
	lru.Add("a", 1)
	lru.Add("b", 2)
	lru.Get("a")
	lru.Add("c", 3)
	c, _ := lru.Get("c")
	got := map[string]any{"size": float64(lru.Size()), "a": lru.Has("a"), "b": lru.Has("b"), "c": float64(c)}
	if !reflect.DeepEqual(got, f["lru"]) {
		t.Fatal(got)
	}
	for _, item := range f["types"].([]any) {
		test := item.(map[string]any)
		value := test["value"]
		if commons.GetType(value) != test["type"] || commons.IsTruthy(value) != test["truthy"] || commons.IsIntegerNumber(value) != test["integer"] {
			t.Fatal(test)
		}
	}
}

func TestFunctionalConcurrencyAndRuntime(t *testing.T) {
	lru := commons.NewLRUCache[int, int](100)
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			lru.Add(i, i)
			if value, ok := lru.Get(i); !ok || value != i {
				t.Errorf("missing %d", i)
			}
		})
	}
	wg.Wait()
	if lru.Size() != 100 {
		t.Fatal("cache lost entries")
	}
	lru.Remove(1)
	if lru.Has(1) {
		t.Fatal("remove failed")
	}
	lru.Clear()
	if lru.Size() != 0 {
		t.Fatal("clear failed")
	}
	lru = commons.NewLRUCache[int, int](0)
	lru.Add(1, 1)
	if lru.Size() != 0 {
		t.Fatal("zero capacity retained value")
	}
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "on-demand")
	u := commons.NewUtility()
	if !u.GetColdStart() || u.GetColdStart() {
		t.Fatal("cold start consumed incorrectly")
	}
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "provisioned-concurrency")
	if commons.NewUtility().GetColdStart() {
		t.Fatal("provisioned cold start")
	}
	t.Setenv("_X_AMZN_TRACE_ID", "Root=environment;Sampled=1")
	t.Setenv("AWS_LAMBDA_MAX_CONCURRENCY", "10")
	if commons.XRayTraceID(context.Background()) != "" {
		t.Fatal("unsafe concurrent trace fallback")
	}
	ctx := context.WithValue(context.Background(), "x-amzn-trace-id", "Root=request;Sampled=1")
	if commons.XRayTraceID(ctx) != "request" || !commons.IsRequestXRaySampled(ctx) {
		t.Fatal("request trace extraction")
	}
}
