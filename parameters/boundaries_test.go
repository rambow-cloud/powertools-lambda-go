package parameters_test

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

func TestParametersDurationBoundaryReference(t *testing.T) {
	raw, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string `json:"version"`
		Cases   []struct {
			Mode  string  `json:"mode"`
			Input *string `json:"input"`
			Name  string  `json:"name"`
			Age   float64 `json:"age"`
			Steps []struct {
				At    int64    `json:"at"`
				Force bool     `json:"force"`
				Clear bool     `json:"clear"`
				Age   *float64 `json:"age"`
			} `json:"steps"`
			Expected jsontext.Value `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "2.35.0" || len(fixture.Cases) != 19 {
		t.Fatal("unexpected duration corpus")
	}
	ages := map[string]time.Duration{"missing": 5 * time.Second, "": 0, "0": 0, "-1": 0, "0.1": 100 * time.Millisecond, "1e-9": time.Nanosecond, "0x10": 16 * time.Second, "1e309": time.Duration(1<<63 - 1), "-Infinity": 0, "Infinity": time.Duration(1<<63 - 1), "invalid": 5 * time.Second, "NaN": 5 * time.Second, "9.223372036854776e9": time.Duration(1<<63 - 1), "5": 5 * time.Second, "1e-324": 0}
	for _, item := range fixture.Cases {
		t.Run(item.Mode+"/"+item.Name, func(t *testing.T) {
			now := time.UnixMilli(10000)
			cache := parameters.NewCache(func() time.Time { return now })
			calls := 0
			fetch := func(context.Context) (any, error) { calls++; return fmt.Sprintf("value-%d", calls), nil }
			if item.Mode == "environment" {
				t.Setenv("POWERTOOLS_PARAMETERS_MAX_AGE", "")
				key := "missing"
				if item.Input == nil {
					if err := os.Unsetenv("POWERTOOLS_PARAMETERS_MAX_AGE"); err != nil {
						t.Fatal(err)
					}
				} else {
					key = *item.Input
					t.Setenv("POWERTOOLS_PARAMETERS_MAX_AGE", key)
				}
				if actual := (parameters.Options{}).Lifetime(); actual != ages[key] {
					t.Fatalf("native duration %q: %s; want %s", key, actual, ages[key])
				}
				var expected struct {
					Value  string `json:"value"`
					Error  string `json:"error"`
					Calls  int    `json:"calls"`
					Cached bool   `json:"cached"`
				}
				if err := json.Unmarshal(item.Expected, &expected); err != nil {
					t.Fatal(err)
				}
				value, err := cache.Get(context.Background(), "key", parameters.Options{}, fetch)
				if expected.Error != "" {
					var config *commons.EnvironmentError
					var get *parameters.GetParameterError
					if !errors.As(err, &config) || !errors.As(err, &get) || calls != expected.Calls {
						t.Fatalf("configuration error/calls: %v/%d", err, calls)
					}
					return
				}
				if err != nil || value != expected.Value || calls != expected.Calls {
					t.Fatalf("environment result %v/%v/%d", value, err, calls)
				}
				if _, cached := cache.Lookup("key", parameters.Options{}); cached != expected.Cached {
					t.Fatalf("cache insertion %q=%t; upstream=%t", key, cached, expected.Cached)
				}
				return
			}
			var expected []struct {
				Value string `json:"value"`
				Calls int    `json:"calls"`
			}
			if err := json.Unmarshal(item.Expected, &expected); err != nil {
				t.Fatal(err)
			}
			for index, step := range item.Steps {
				now = time.UnixMilli(step.At)
				if step.Clear {
					cache.ClearCache()
				}
				want := expected[index]
				// Preserve upstream observations: Go keeps fractional durations
				// whereas Date.setSeconds rounds fractional seconds. Nonpositive
				// age skips storage in both. Go also bypasses existing warm entries.
				if item.Name == "fractional" && index == 1 {
					if want.Calls != 2 {
						t.Fatal("reference fractional behavior changed")
					}
					want.Value, want.Calls = "value-1", 1
				}
				if item.Name == "warm-nonpositive" && index > 0 {
					if want.Calls != 1 {
						t.Fatal("pinned warm zero/negative behavior changed")
					}
					want.Calls = min(index+1, 3)
					if index < 3 {
						want.Value = fmt.Sprintf("value-%d", want.Calls)
					}
				}
				age := item.Age
				if step.Age != nil {
					age = *step.Age
				}
				value, err := cache.Get(context.Background(), "key", parameters.Options{MaxAge: parameters.Age(time.Duration(age * float64(time.Second))), ForceFetch: step.Force}, fetch)
				if err != nil || value != want.Value || calls != want.Calls {
					t.Fatalf("step%d=%v/%v/%d; want %v", index, value, err, calls, want)
				}
			}
		})
	}
}

func TestMultipleClearDuringFetchAndNativeSnapshot(t *testing.T) {
	cache := parameters.NewCache(nil)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	done := make(chan error, 1)
	go func() {
		_, err := cache.GetMultiple(context.Background(), "path", parameters.Options{}, func(context.Context) (map[string]any, error) {
			close(entered)
			<-release
			return map[string]any{"bytes": []byte{0, 255}}, nil
		})
		done <- err
	}()
	<-entered
	cache.ClearCache()
	once.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	calls := 0
	fetch := func(context.Context) (map[string]any, error) {
		calls++
		return map[string]any{"bytes": []byte{1, 255}}, nil
	}
	value, err := cache.GetMultiple(context.Background(), "path", parameters.Options{}, fetch)
	if err != nil || calls != 1 {
		t.Fatal("cleared in-flight batch refilled cache")
	}
	value["bytes"].([]byte)[0] = 9
	next, err := cache.GetMultiple(context.Background(), "path", parameters.Options{}, fetch)
	if err != nil || !reflect.DeepEqual(next["bytes"], []byte{1, 255}) || calls != 1 {
		t.Fatalf("native batch snapshot %v/%v/%d", next, err, calls)
	}
	for _, options := range []parameters.Options{{MaxAge: parameters.Age(-time.Second)}, {MaxAge: parameters.Age(0)}, {ForceFetch: true}} {
		if _, err := cache.GetMultiple(context.Background(), "path", options, fetch); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 4 {
		t.Fatalf("bypass/force calls=%d", calls)
	}
}
