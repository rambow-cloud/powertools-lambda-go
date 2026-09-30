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
	"sync/atomic"
	"testing"
	"time"
)

func TestDiagnosticReference(t *testing.T) {
	data, err := os.ReadFile("testdata/warnings-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Now   int64
		Cases []struct {
			Name, Namespace    string
			Disabled, Required bool
			Defaults           Dimensions
			Steps              [][]json.RawMessage
			Results            []struct {
				Value any
				Error *string
			}
			Warnings []string
			Emitted  []map[string]any
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 203 {
		t.Fatalf("cases: %d", len(fixture.Cases))
	}
	t.Setenv("POWERTOOLS_METRICS_NAMESPACE", "")
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			var output bytes.Buffer
			warnings := []string{}
			m, err := New(WithNamespace(item.Namespace), WithServiceName("orders"), WithDefaultDimensions(item.Defaults), WithDisabled(item.Disabled), WithRequireMetrics(item.Required), WithOutput(&output), WithClock(func() time.Time { return time.UnixMilli(fixture.Now) }), WithWarningHandler(func(message string) { warnings = append(warnings, message) }))
			if err != nil {
				t.Fatal(err)
			}
			for index, step := range item.Steps {
				var operation string
				_ = json.Unmarshal(step[0], &operation)
				text := func(i int) string {
					var value string
					if err := json.Unmarshal(step[i], &value); err != nil {
						t.Fatal(err)
					}
					return value
				}
				var value any
				var failure error
				switch operation {
				case "addDimension":
					failure = m.AddDimension(text(1), text(2))
				case "addDimensions", "setDefaultDimensions":
					var dimensions Dimensions
					_ = json.Unmarshal(step[1], &dimensions)
					if operation == "addDimensions" {
						failure = m.AddDimensionSet(dimensions)
					} else {
						failure = m.SetDefaultDimensions(dimensions)
					}
				case "addMetadata":
					failure = m.AddMetadata(text(1), text(2))
				case "addMetric":
					var number float64
					_ = json.Unmarshal(step[3], &number)
					failure = m.AddMetric(text(1), Unit(text(2)), number)
				case "setTimestamp":
					var millis int64
					_ = json.Unmarshal(step[1], &millis)
					failure = m.SetTimestamp(time.UnixMilli(millis))
				case "publishStoredMetrics":
					failure = m.Flush()
				case "serializeMetrics":
					var serialized []byte
					serialized, failure = m.Serialize()
					if failure == nil {
						if err := json.Unmarshal(serialized, &value); err != nil {
							t.Fatal(err)
						}
					}
				default:
					t.Fatalf("unknown operation %s", operation)
				}
				var message *string
				if failure != nil {
					text := failure.Error()
					message = &text
				}
				want := item.Results[index]
				if !reflect.DeepEqual(message, want.Error) || !reflect.DeepEqual(value, want.Value) {
					t.Fatalf("%d/%s: value %#v, error %v; want %#v, %v", index, operation, value, message, want.Value, want.Error)
				}
			}
			if !reflect.DeepEqual(warnings, item.Warnings) {
				t.Fatalf("warnings:\ngot %#v\nwant %#v", warnings, item.Warnings)
			}
			if got := documents(t, &output); !reflect.DeepEqual(got, item.Emitted) {
				t.Fatalf("emission: %#v; want %#v", got, item.Emitted)
			}
		})
	}
}

func TestWarningsReleaseLocksAndCloseScopes(t *testing.T) {
	var m *Metrics
	var warnings atomic.Int64
	m, _ = setup(t, WithWarningHandler(func(string) { warnings.Add(1); _ = m.HasStoredMetrics() }))
	if err := m.AddDimension("root", ""); err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for i := range 64 {
		wait.Go(func() {
			ctx, finish := m.StartScope(context.Background())
			bound := m.WithContext(ctx)
			if err := bound.AddDimension(fmt.Sprint(i), ""); err != nil {
				t.Error(err)
			}
			if err := finish(); err != nil {
				t.Error(err)
			}
			if err := bound.AddDimension("closed", ""); !errors.Is(err, ErrInvocationClosed) {
				t.Errorf("closed: %v", err)
			}
		})
	}
	wait.Wait()
	if warnings.Load() != 129 {
		t.Fatalf("warning count: %d", warnings.Load())
	}
	var bound *Metrics
	closing, _ := setup(t, WithWarningHandler(func(string) {
		if bound.HasStoredMetrics() {
			t.Error("scope should be cleared before warning delivery")
		}
		if err := bound.AddMetric("Late", Count, 1); !errors.Is(err, ErrInvocationClosed) {
			t.Errorf("scope not closed: %v", err)
		}
	}))
	ctx, finish := closing.StartScope(context.Background())
	bound = closing.WithContext(ctx)
	if err := finish(); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticFlushDiagnostics(t *testing.T) {
	for _, distinct := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("distinct=%v/fail=%v", distinct, fail), func(t *testing.T) {
				var m *Metrics
				var output bytes.Buffer
				var writer io.Writer = &output
				if fail {
					writer = failingWriter{}
				}
				var warnings []string
				var occupied []bool
				m, _ = setup(t, WithOutput(writer), WithWarningHandler(func(message string) {
					warnings = append(warnings, message)
					occupied = append(occupied, m.HasStoredMetrics())
				}))
				if err := m.AddMetadata("service", "overwritten"); err != nil {
					t.Fatal(err)
				}
				limit := 100
				if distinct {
					limit++
				}
				for i := range limit {
					name := "Count"
					if distinct {
						name = fmt.Sprintf("Count%d", i)
					}
					err := m.AddMetric(name, Count, 1)
					if fail && i == limit-1 {
						if !errors.Is(err, io.ErrClosedPipe) {
							t.Fatalf("writer error: %v", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
				}
				want := []string{"EMF key \"service\" is defined as both a metadata and default dimension; the default dimension value will take precedence in the serialized output"}
				if !reflect.DeepEqual(warnings, want) || !reflect.DeepEqual(occupied, []bool{distinct && !fail}) {
					t.Fatalf("warnings=%v, occupied=%v", warnings, occupied)
				}
				if err := m.AddDimension("AfterFlush", "accepted"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
