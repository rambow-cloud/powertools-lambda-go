package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestColdStartReference(t *testing.T) {
	data, err := os.ReadFile("testdata/coldstart-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Now   int64
		Cases []struct {
			Initialization, Lifecycle         string
			Disabled                          bool
			EnvName, Option, Setter, Argument *string
			Parent                            map[string]any
			Emitted                           []map[string]any
			Warnings                          []string
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 302 {
		t.Fatalf("case count: %d", len(fixture.Cases))
	}
	for index, item := range fixture.Cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			for _, key := range []string{"POWERTOOLS_METRICS_NAMESPACE", "POWERTOOLS_SERVICE_NAME", "POWERTOOLS_DEV", "POWERTOOLS_METRICS_FUNCTION_NAME"} {
				t.Setenv(key, "")
			}
			t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", item.Initialization)
			t.Setenv("POWERTOOLS_METRICS_DISABLED", fmt.Sprint(item.Disabled))
			if item.EnvName != nil {
				t.Setenv("POWERTOOLS_METRICS_FUNCTION_NAME", *item.EnvName)
			}
			var output bytes.Buffer
			warnings := []string{}
			opts := []Option{WithNamespace("ColdTest"), WithServiceName("orders"), WithDefaultDimensions(Dimensions{"environment": "test"}), WithOutput(&output), WithClock(func() time.Time { return time.UnixMilli(fixture.Now) }), WithWarningHandler(func(message string) { warnings = append(warnings, message) })}
			if item.Option != nil {
				opts = append(opts, WithFunctionName(*item.Option))
			}
			m, err := New(opts...)
			if err != nil {
				t.Fatal(err)
			}
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(m.AddDimension("request", "parent"))
			must(m.AddMetadata("context", "parent"))
			must(m.SetTimestamp(time.UnixMilli(fixture.Now - 3600000)))
			must(m.AddMetric("Parent", Count, 2))
			if item.Setter != nil {
				must(m.SetFunctionName(*item.Setter))
			}
			if item.Lifecycle == "clear" {
				must(m.ClearMetrics())
			}
			target := m
			if item.Lifecycle == "single" {
				target, err = m.SingleMetric()
				must(err)
			}
			var argument []string
			if item.Argument != nil {
				argument = []string{*item.Argument}
			}
			must(target.CaptureColdStartMetric(argument...))
			if item.Lifecycle == "setter-after" {
				must(target.SetFunctionName("late"))
			}
			must(target.CaptureColdStartMetric("second"))
			serialized, err := m.Serialize()
			must(err)
			var parent map[string]any
			must(json.Unmarshal(serialized, &parent))
			must(m.Flush())
			if !reflect.DeepEqual(parent, item.Parent) || !reflect.DeepEqual(documents(t, &output), item.Emitted) || !reflect.DeepEqual(warnings, item.Warnings) {
				t.Fatalf("parent=%#v\nemitted=%s\nwarnings=%#v\nwant parent=%#v\nemitted=%#v\nwarnings=%#v", parent, output.String(), warnings, item.Parent, item.Emitted, item.Warnings)
			}
		})
	}
}

func TestColdStartConcurrentAndFailedCapture(t *testing.T) {
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "on-demand")
	m, output := setup(t)
	var group sync.WaitGroup
	for range 64 {
		group.Go(func() {
			if err := m.CaptureColdStartMetric("concurrent"); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if got := documents(t, output); len(got) != 1 || got[0]["ColdStart"] != float64(1) {
		t.Fatalf("emission: %#v", got)
	}
	failing, _ := setup(t, WithOutput(failingWriter{}))
	if err := failing.CaptureColdStartMetric(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write error: %v", err)
	}
	if err := failing.CaptureColdStartMetric(); err != nil {
		t.Fatalf("failed capture retried: %v", err)
	}
}

func TestColdStartScopeNameIsolationAndClosure(t *testing.T) {
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "on-demand")
	m, _ := setup(t, WithWarningHandler(func(string) {}))
	if err := m.SetFunctionName("parent"); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := range 64 {
		group.Go(func() {
			ctx, finish := m.StartScope(context.Background())
			bound := m.WithContext(ctx)
			if got := *bound.current.functionName; got != "parent" {
				t.Errorf("inherited name: %s", got)
			}
			name := fmt.Sprint(i)
			if err := bound.SetFunctionName(name); err != nil {
				t.Error(err)
			}
			if err := bound.Clear(); err != nil {
				t.Error(err)
			}
			if got := *bound.current.functionName; got != name {
				t.Errorf("cleared name: %s", got)
			}
			if err := finish(); err != nil {
				t.Error(err)
			}
			if !errors.Is(bound.SetFunctionName("late"), ErrInvocationClosed) || !errors.Is(bound.CaptureColdStartMetric(), ErrInvocationClosed) {
				t.Error("late operation accepted")
			}
		})
	}
	group.Wait()
	if got := *m.current.functionName; got != "parent" {
		t.Fatalf("parent mutated: %s", got)
	}
}
