package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestTimestampReference(t *testing.T) {
	data, err := os.ReadFile("testdata/timestamps-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Clock              int64
			Kind, Lifecycle    string
			Value              json.RawMessage
			Disabled, Required bool
			Steps              []string
			Results            []struct {
				Value      any
				Error      *string
				ClockCalls int
			}
			Warnings []string
			Emitted  []map[string]any
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 968 {
		t.Fatalf("case count: %d", len(fixture.Cases))
	}
	for index, item := range fixture.Cases {
		t.Run(fmt.Sprintf("%d/%s/%s", index, item.Kind, item.Lifecycle), func(t *testing.T) {
			t.Setenv("POWERTOOLS_METRICS_DISABLED", fmt.Sprint(item.Disabled))
			var output bytes.Buffer
			warnings := []string{}
			clockCalls := 0
			parent, err := New(WithNamespace("Timestamp"), WithServiceName("orders"), WithRequireMetrics(item.Required), WithOutput(&output), WithClock(func() time.Time { clockCalls++; return time.UnixMilli(item.Clock) }), WithWarningHandler(func(message string) { warnings = append(warnings, message) }))
			if err != nil {
				t.Fatal(err)
			}
			m := parent
			for i, operation := range item.Steps {
				var value any
				var failure error
				switch operation {
				case "setTimestamp":
					if item.Kind == "iso" {
						var text string
						if err := json.Unmarshal(item.Value, &text); err != nil {
							t.Fatal(err)
						}
						instant, err := time.Parse(time.RFC3339Nano, text)
						if err != nil {
							t.Fatal(err)
						}
						failure = m.SetTimestamp(instant)
					} else {
						number := referenceMetricNumber(t, item.Value)
						if item.Kind == "number" {
							failure = m.SetTimestampMillis(number)
						} else {
							instant := maximumDate.Add(time.Millisecond)
							if !math.IsNaN(number) && math.Abs(number) <= 8640000000000000 {
								instant = time.UnixMilli(int64(math.Trunc(number)))
							}
							failure = m.SetTimestamp(instant)
						}
					}
				case "addMetric":
					failure = m.AddMetric("Count", Count, 1)
				case "clearMetrics":
					failure = m.ClearMetrics()
				case "clearDimensions":
					failure = m.ClearDimensions()
				case "singleMetric":
					m, failure = m.SingleMetric()
				case "serializeMetrics":
					var data []byte
					data, failure = m.Serialize()
					if failure == nil {
						if err := json.Unmarshal(data, &value); err != nil {
							t.Fatal(err)
						}
					}
				case "publishStoredMetrics":
					failure = m.Flush()
				case "publishParent":
					failure = parent.Flush()
				default:
					t.Fatalf("unknown operation: %s", operation)
				}
				var message *string
				if failure != nil {
					text := failure.Error()
					message = &text
				}
				want := item.Results[i]
				if !reflect.DeepEqual(value, want.Value) || !reflect.DeepEqual(message, want.Error) || clockCalls != want.ClockCalls {
					got, _ := json.Marshal(map[string]any{"value": value, "error": message, "clockCalls": clockCalls})
					expected, _ := json.Marshal(want)
					t.Fatalf("%d/%s: %s; want %s", i, operation, got, expected)
				}
			}
			if got := documents(t, &output); !reflect.DeepEqual(got, item.Emitted) || !reflect.DeepEqual(warnings, item.Warnings) {
				t.Fatalf("emitted=%#v warnings=%#v; want %#v / %#v", got, warnings, item.Emitted, item.Warnings)
			}
		})
	}
}

func TestTimestampScopesAndClosedCalls(t *testing.T) {
	m, _ := setup(t, WithWarningHandler(func(string) {}))
	var group sync.WaitGroup
	for i := range 64 {
		group.Go(func() {
			ctx, finish := m.StartScope(context.Background())
			bound := m.WithContext(ctx)
			value := float64(i)
			if err := bound.SetTimestampMillis(value); err != nil {
				t.Error(err)
			}
			data, err := bound.Serialize()
			if err != nil {
				t.Error(err)
				return
			}
			var document struct {
				AWS struct{ Timestamp float64 } `json:"_aws"`
			}
			if err := json.Unmarshal(data, &document); err != nil || document.AWS.Timestamp != value {
				t.Errorf("timestamp: %s; %v", data, err)
			}
			if err := finish(); err != nil {
				t.Error(err)
			}
			if !errors.Is(bound.SetTimestampMillis(math.NaN()), ErrInvocationClosed) || !errors.Is(bound.SetTimestamp(time.Now()), ErrInvocationClosed) {
				t.Error("closed timestamp accepted")
			}
		})
	}
	group.Wait()
}
