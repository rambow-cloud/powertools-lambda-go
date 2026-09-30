package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestStoreReference(t *testing.T) {
	data, err := os.ReadFile("testdata/stores-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Now   int64
		Cases []struct {
			Name               string
			Disabled, Required bool
			Steps              [][]json.RawMessage
			Results            []struct {
				EmptyError bool
				Value      any
			}
			Emitted []map[string]any
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 104 {
		t.Fatalf("cases: %d", len(fixture.Cases))
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			m, output := setup(t, WithNamespace("StoreTest"), WithClock(func() time.Time { return time.UnixMilli(fixture.Now) }), WithDisabled(item.Disabled), WithRequireMetrics(item.Required))
			t.Setenv("POWERTOOLS_METRICS_DISABLED", fmt.Sprint(item.Disabled))
			for index, step := range item.Steps {
				var operation string
				_ = json.Unmarshal(step[0], &operation)
				text := func(i int) string {
					var s string
					if err := json.Unmarshal(step[i], &s); err != nil {
						t.Fatal(err)
					}
					return s
				}
				var value any
				var err error
				switch operation {
				case "addDimension":
					err = m.AddDimension(text(1), text(2))
				case "addDimensions":
					var dims Dimensions
					_ = json.Unmarshal(step[1], &dims)
					err = m.AddDimensionSet(dims)
				case "addMetadata":
					err = m.AddMetadata(text(1), text(2))
				case "addMetric":
					var number float64
					_ = json.Unmarshal(step[3], &number)
					resolution := Standard
					if len(step) > 4 {
						_ = json.Unmarshal(step[4], &resolution)
					}
					err = m.AddMetric(text(1), Unit(text(2)), number, resolution)
				case "setTimestamp":
					var millis int64
					_ = json.Unmarshal(step[1], &millis)
					err = m.SetTimestamp(time.UnixMilli(millis))
				case "setThrowOnEmptyMetrics":
					var enabled bool
					_ = json.Unmarshal(step[1], &enabled)
					err = m.SetThrowOnEmptyMetrics(enabled)
				case "throwOnEmptyMetrics":
					err = m.ThrowOnEmptyMetrics()
				case "clearDimensions":
					err = m.ClearDimensions()
				case "clearMetadata":
					err = m.ClearMetadata()
				case "clearMetrics":
					err = m.ClearMetrics()
				case "clearDefaultDimensions":
					err = m.ClearDefaultDimensions()
				case "hasStoredMetrics":
					value = m.HasStoredMetrics()
				case "singleMetric":
					m, err = m.SingleMetric()
				case "publishStoredMetrics":
					err = m.Flush()
				case "serializeMetrics":
					var data []byte
					data, err = m.Serialize()
					if err == nil {
						_ = json.Unmarshal(data, &value)
					}
				default:
					t.Fatalf("unknown operation: %s", operation)
				}
				want := item.Results[index]
				if err != nil && !errors.Is(err, ErrEmptyMetrics) {
					t.Fatalf("%d/%s: unexpected error: %v", index, operation, err)
				}
				if errors.Is(err, ErrEmptyMetrics) != want.EmptyError || !reflect.DeepEqual(value, want.Value) {
					t.Fatalf("%d/%s: got %#v, %v; want %#v, empty error=%v", index, operation, value, err, want.Value, want.EmptyError)
				}
			}
			got := documents(t, output)
			for _, document := range got {
				normalizeStoreDimensions(document)
			}
			if !reflect.DeepEqual(got, item.Emitted) {
				t.Fatalf("emission: got %#v, want %#v", got, item.Emitted)
			}
		})
	}
}

func normalizeStoreDimensions(document map[string]any) {
	for _, directive := range document["_aws"].(map[string]any)["CloudWatchMetrics"].([]any) {
		for _, names := range directive.(map[string]any)["Dimensions"].([]any) {
			values := names.([]any)
			sort.Slice(values, func(i, j int) bool { return values[i].(string) < values[j].(string) })
		}
	}
}

func TestStoreScopeIsolationAndClosure(t *testing.T) {
	m, _ := setup(t)
	if err := m.SetThrowOnEmptyMetrics(true); err != nil {
		t.Fatal(err)
	}
	ctx, finish := m.StartScope(context.Background())
	bound := m.WithContext(ctx)
	if err := m.SetThrowOnEmptyMetrics(false); err != nil {
		t.Fatal(err)
	}
	if _, err := bound.Serialize(); !errors.Is(err, ErrEmptyMetrics) {
		t.Fatal("active scope did not retain its policy snapshot")
	}
	if err := bound.SetThrowOnEmptyMetrics(false); err != nil {
		t.Fatal(err)
	}
	if err := bound.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := finish(); err != nil {
		t.Fatal(err)
	}
	for _, action := range []func() error{bound.ClearDimensions, bound.ClearMetadata, bound.ClearMetrics, bound.ThrowOnEmptyMetrics, func() error { return bound.SetThrowOnEmptyMetrics(false) }} {
		if err := action(); !errors.Is(err, ErrInvocationClosed) {
			t.Fatalf("late mutation: %v", err)
		}
	}
	if bound.HasStoredMetrics() {
		t.Fatal("closed scope retained metrics")
	}
	var wait sync.WaitGroup
	for index := range 64 {
		wait.Go(func() {
			ctx, close := m.StartScope(context.Background())
			request := m.WithContext(ctx)
			if err := request.SetThrowOnEmptyMetrics(index%2 == 0); err != nil {
				t.Error(err)
			}
			if err := request.AddMetric("Request", Count, 1); err != nil {
				t.Error(err)
			}
			if err := request.ClearMetrics(); err != nil {
				t.Error(err)
			}
			if request.HasStoredMetrics() {
				t.Error("metrics not cleared")
			}
			if err := close(); errors.Is(err, ErrEmptyMetrics) != (index%2 == 0) {
				t.Errorf("scope policy: %d: %v", index, err)
			}
		})
	}
	wait.Wait()
	if err := m.Flush(); err != nil {
		t.Fatalf("child policy leaked into parent: %v", err)
	}
}
