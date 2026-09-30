package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestValueReference(t *testing.T) {
	data, err := os.ReadFile("testdata/values-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Now   int64
		Cases []struct {
			Name                       string
			Disabled, Required, Single bool
			Steps                      [][]json.RawMessage
			Results                    []struct {
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
	if len(fixture.Cases) != 641 {
		t.Fatalf("case count: %d", len(fixture.Cases))
	}
	for index, item := range fixture.Cases {
		t.Run(fmt.Sprintf("%d/%s", index, item.Name), func(t *testing.T) {
			var output bytes.Buffer
			warnings := []string{}
			m, err := New(WithNamespace("Values"), WithServiceName("orders"), WithDisabled(item.Disabled), WithRequireMetrics(item.Required), WithSingleMetric(item.Single), WithOutput(&output), WithClock(func() time.Time { return time.UnixMilli(fixture.Now) }), WithWarningHandler(func(message string) { warnings = append(warnings, message) }))
			if err != nil {
				t.Fatal(err)
			}
			for i, step := range item.Steps {
				text := func(i int) string {
					var value string
					if err := json.Unmarshal(step[i], &value); err != nil {
						t.Fatal(err)
					}
					return value
				}
				var result any
				var failure error
				switch operation := text(0); operation {
				case "addMetric":
					resolution := Standard
					if len(step) > 4 {
						if err := json.Unmarshal(step[4], &resolution); err != nil {
							t.Fatal(err)
						}
					}
					failure = m.AddMetric(text(1), Unit(text(2)), referenceMetricNumber(t, step[3]), resolution)
				case "repeatMetric", "fill":
					var count int
					if err := json.Unmarshal(step[1], &count); err != nil {
						t.Fatal(err)
					}
					for j := count; j > 0; j-- {
						if operation == "fill" {
							failure = m.AddMetric(fmt.Sprint(j), Count, float64(j))
						} else {
							failure = m.AddMetric(text(2), Unit(text(3)), referenceMetricNumber(t, step[4]))
						}
						if failure != nil {
							break
						}
					}
				case "addMetadata":
					var value any
					if err := json.Unmarshal(step[2], &value); err != nil {
						t.Fatal(err)
					}
					failure = m.AddMetadata(text(1), value)
				case "addDimension":
					failure = m.AddDimension(text(1), text(2))
				case "addDimensions", "setDefaultDimensions":
					var dimensions Dimensions
					if err := json.Unmarshal(step[1], &dimensions); err != nil {
						t.Fatal(err)
					}
					if operation == "addDimensions" {
						failure = m.AddDimensionSet(dimensions)
					} else {
						failure = m.SetDefaultDimensions(dimensions)
					}
				case "serializeMetrics":
					var data []byte
					data, failure = m.Serialize()
					if failure == nil {
						if err := json.Unmarshal(data, &result); err != nil {
							t.Fatal(err)
						}
					}
				case "publishStoredMetrics":
					failure = m.Flush()
				case "hasStoredMetrics":
					result = m.HasStoredMetrics()
				default:
					t.Fatalf("unknown operation: %s", operation)
				}
				var message *string
				if failure != nil {
					value := failure.Error()
					message = &value
				}
				want := item.Results[i]
				if !reflect.DeepEqual(result, want.Value) || !reflect.DeepEqual(message, want.Error) {
					got, _ := json.Marshal(map[string]any{"value": result, "error": message})
					expected, _ := json.Marshal(want)
					t.Fatalf("step %d/%s: %s; want %s", i, text(0), got, expected)
				}
			}
			if got := documents(t, &output); !reflect.DeepEqual(got, item.Emitted) || !reflect.DeepEqual(warnings, item.Warnings) {
				t.Fatalf("emitted=%#v\nwarnings=%#v\nwant emitted=%#v\nwarnings=%#v", got, warnings, item.Emitted, item.Warnings)
			}
		})
	}
}

func referenceMetricNumber(t *testing.T, data json.RawMessage) float64 {
	t.Helper()
	var value float64
	if json.Unmarshal(data, &value) == nil {
		return value
	}
	var tag struct{ Number string }
	if err := json.Unmarshal(data, &tag); err != nil {
		t.Fatal(err)
	}
	switch tag.Number {
	case "NaN":
		return math.NaN()
	case "Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	case "-0":
		return math.Copysign(0, -1)
	default:
		t.Fatalf("unknown number: %s", data)
		return 0
	}
}

func TestMetricNegativeZeroJSON(t *testing.T) {
	m, _ := setup(t)
	if err := m.AddMetric("Value", Count, math.Copysign(0, -1)); err != nil {
		t.Fatal(err)
	}
	data, err := m.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"Value":0`)) {
		t.Fatalf("negative zero encoding: %s", data)
	}
}
