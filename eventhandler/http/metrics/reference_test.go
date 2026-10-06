package metrics_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"testing"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
	powermetrics "github.com/rambow-cloud/powertools-lambda-go/metrics"
)

func TestReference(t *testing.T) {
	data, err := os.ReadFile("testdata/metrics-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name, Action string
			Event        json.RawMessage
			Status       int
			Documents    []map[string]any
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 176 {
		t.Fatalf("reference cases: %d", len(fixture.Cases))
	}
	for _, item := range fixture.Cases {
		for _, capture := range []bool{false, true} {
			name := item.Name + "/default"
			if capture {
				name = item.Name + "/request-count"
			}
			t.Run(name, func(t *testing.T) {
				var output bytes.Buffer
				m := newMetrics(t, &output)
				app := httpapi.New(httpapi.Options{})
				middleware := httpmetrics.New(m)
				if capture {
					middleware = httpmetrics.New(m, httpmetrics.Options{CaptureRequestCount: true})
				}
				app.Use(middleware, httpapi.CORS(httpapi.CORSOptions{}))
				if err := app.Get("/items/:id", func(r *httpapi.RequestContext) (any, error) {
					switch item.Action {
					case "http-error":
						return nil, httpapi.NewHTTPError(400, "bad input")
					case "error":
						return nil, errors.New("business failure")
					case "business":
						bound := m.WithContext(r.Context)
						if err := bound.AddMetric("Orders", powermetrics.Count, 2); err != nil {
							return nil, err
						}
						if err := bound.AddMetadata("custom", "value"); err != nil {
							return nil, err
						}
					}
					status := map[string]int{"created": 201, "redirect": 302, "client": 422, "server": 503}[item.Action]
					if status == 0 {
						status = 200
					}
					return httpapi.Response{StatusCode: status, Body: "ok"}, nil
				}); err != nil {
					t.Fatal(err)
				}
				response, err := app.Resolve(context.Background(), item.Event)
				if err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != item.Status {
					t.Fatalf("status: got %d, want %d", response.StatusCode, item.Status)
				}
				documents := decodeDocuments(t, output.Bytes())
				for _, document := range documents {
					if capture {
						assertRequestCount(t, document)
						delete(document, "request")
						for _, directive := range document["_aws"].(map[string]any)["CloudWatchMetrics"].([]any) {
							entry := directive.(map[string]any)
							filtered := []any{}
							for _, metric := range entry["Metrics"].([]any) {
								if metric.(map[string]any)["Name"] != "request" {
									filtered = append(filtered, metric)
								}
							}
							entry["Metrics"] = filtered
						}
					}
					if latency, ok := document["latency"].(float64); !ok || latency < 0 {
						t.Fatalf("latency: %v", document["latency"])
					}
					delete(document, "latency")
					aws := document["_aws"].(map[string]any)
					delete(aws, "Timestamp")
					for _, directive := range aws["CloudWatchMetrics"].([]any) {
						for _, keys := range directive.(map[string]any)["Dimensions"].([]any) {
							values := keys.([]any)
							sort.Slice(values, func(i, j int) bool { return values[i].(string) < values[j].(string) })
						}
					}
				}
				if !reflect.DeepEqual(documents, item.Documents) {
					t.Fatalf("EMF mismatch\ngot: %#v\nwant: %#v", documents, item.Documents)
				}
			})
		}
	}
}

func assertRequestCount(t *testing.T, document map[string]any) {
	t.Helper()
	if document["request"] != float64(1) {
		t.Fatalf("request count: %v", document["request"])
	}
	definitions := 0
	for _, directive := range document["_aws"].(map[string]any)["CloudWatchMetrics"].([]any) {
		for _, value := range directive.(map[string]any)["Metrics"].([]any) {
			metric := value.(map[string]any)
			if metric["Name"] == "request" {
				definitions++
				if metric["Unit"] != "Count" {
					t.Fatalf("request unit: %v", metric)
				}
			}
		}
	}
	if definitions != 1 {
		t.Fatalf("request definitions: %d", definitions)
	}
}

func newMetrics(t *testing.T, output *bytes.Buffer) *powermetrics.Metrics {
	t.Helper()
	m, err := powermetrics.New(powermetrics.WithNamespace("HTTPTest"), powermetrics.WithServiceName("orders"), powermetrics.WithDefaultDimensions(powermetrics.Dimensions{"environment": "test"}), powermetrics.WithOutput(output), powermetrics.WithDisabled(false))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func decodeDocuments(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	documents := []map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var document map[string]any
		if err := json.Unmarshal(line, &document); err != nil {
			t.Fatal(err)
		}
		documents = append(documents, document)
	}
	return documents
}
