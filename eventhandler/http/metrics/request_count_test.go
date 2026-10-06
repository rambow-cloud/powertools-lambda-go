package metrics_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
	powermetrics "github.com/rambow-cloud/powertools-lambda-go/metrics"
)

func TestRequestCountOptions(t *testing.T) {
	for _, item := range []struct {
		name              string
		options           []httpmetrics.Options
		capture, disabled bool
	}{
		{name: "default"},
		{name: "zero", options: []httpmetrics.Options{{}}},
		{name: "explicit-false", options: []httpmetrics.Options{{CaptureRequestCount: false}}},
		{name: "enabled", options: []httpmetrics.Options{{CaptureRequestCount: true}}, capture: true},
		{name: "disabled", options: []httpmetrics.Options{{CaptureRequestCount: true}}, capture: true, disabled: true},
	} {
		t.Run(item.name, func(t *testing.T) {
			var output bytes.Buffer
			m, err := powermetrics.New(powermetrics.WithNamespace("HTTPTest"),
				powermetrics.WithOutput(&output), powermetrics.WithDisabled(item.disabled))
			if err != nil {
				t.Fatal(err)
			}
			app := httpapi.New(httpapi.Options{})
			app.Use(httpmetrics.New(m, item.options...))
			if err := app.Get("/items/:id", func(*httpapi.RequestContext) (any, error) {
				return "ok", nil
			}); err != nil {
				t.Fatal(err)
			}
			response, err := app.Resolve(context.Background(), requestEvent("options"))
			if err != nil || response.StatusCode != 200 {
				t.Fatalf("response: %v, %v", response, err)
			}
			documents := decodeDocuments(t, output.Bytes())
			if item.disabled {
				if len(documents) != 0 {
					t.Fatalf("disabled output: %s", output.String())
				}
				return
			}
			if len(documents) != 1 {
				t.Fatalf("documents: %d", len(documents))
			}
			document := documents[0]
			if item.capture {
				assertRequestCount(t, document)
			} else if _, exists := document["request"]; exists {
				t.Fatalf("unexpected request metric: %v", document)
			}
			definitions := document["_aws"].(map[string]any)["CloudWatchMetrics"].([]any)[0].(map[string]any)["Metrics"].([]any)
			want := 3
			if item.capture {
				want++
			}
			if len(definitions) != want {
				t.Fatalf("metric definitions: %v", definitions)
			}
		})
	}
}

func TestRequestCountAutomaticPublication(t *testing.T) {
	for _, item := range []struct {
		name    string
		pending int
		single  bool
	}{
		{name: "counter-crosses-metric-limit", pending: 97},
		{name: "full-metric-store", pending: 100},
		{name: "single-metric", single: true},
	} {
		t.Run(item.name, func(t *testing.T) {
			var output bytes.Buffer
			m, err := powermetrics.New(powermetrics.WithNamespace("HTTPTest"),
				powermetrics.WithOutput(&output), powermetrics.WithDisabled(false), powermetrics.WithSingleMetric(item.single))
			if err != nil {
				t.Fatal(err)
			}
			app := httpapi.New(httpapi.Options{})
			app.Use(httpmetrics.New(m, httpmetrics.Options{CaptureRequestCount: true}))
			if err := app.Get("/items/:id", func(r *httpapi.RequestContext) (any, error) {
				bound := m.WithContext(r.Context)
				for i := range item.pending {
					if err := bound.AddMetric(fmt.Sprintf("Business%d", i), powermetrics.Count, 1); err != nil {
						return nil, err
					}
				}
				return "ok", nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := app.Resolve(context.Background(), requestEvent("automatic")); err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			for _, document := range decodeDocuments(t, output.Bytes()) {
				for _, name := range []string{"request", "latency", "fault", "error"} {
					if document[name] == nil {
						continue
					}
					counts[name]++
					if document["route"] != "GET /items/:id" {
						t.Fatalf("route lost after automatic publication: %v", document)
					}
					if name == "request" {
						assertRequestCount(t, document)
					}
				}
			}
			for _, name := range []string{"request", "latency", "fault", "error"} {
				if counts[name] != 1 {
					t.Fatalf("%s published %d times", name, counts[name])
				}
			}
		})
	}
}

func TestRequestCountNestedLambdaScope(t *testing.T) {
	var output bytes.Buffer
	m := newMetrics(t, &output)
	if err := m.AddMetric("Parent", powermetrics.Count, 1); err != nil {
		t.Fatal(err)
	}
	app := httpapi.New(httpapi.Options{})
	app.Use(httpmetrics.New(m, httpmetrics.Options{CaptureRequestCount: true}))
	if err := app.Get("/items/:id", func(r *httpapi.RequestContext) (any, error) {
		return "ok", m.WithContext(r.Context).AddMetric("Orders", powermetrics.Count, 2)
	}); err != nil {
		t.Fatal(err)
	}
	handler := powermetrics.WrapHandler(m, func(ctx context.Context, event map[string]any) (any, error) {
		if err := m.WithContext(ctx).AddMetric("Invocations", powermetrics.Count, 1); err != nil {
			return nil, err
		}
		return app.Resolve(ctx, event)
	})
	if _, err := handler(context.Background(), requestEvent("nested")); err != nil {
		t.Fatal(err)
	}
	documents := decodeDocuments(t, output.Bytes())
	if len(documents) != 2 {
		t.Fatalf("documents: %d", len(documents))
	}
	assertRequestCount(t, documents[0])
	if documents[0]["Orders"] != float64(2) || documents[0]["route"] != "GET /items/:id" || documents[0]["Invocations"] != nil || documents[0]["Parent"] != nil {
		t.Fatalf("HTTP scope: %v", documents[0])
	}
	if documents[1]["Invocations"] != float64(1) || documents[1]["request"] != nil || documents[1]["route"] != nil || documents[1]["Orders"] != nil || documents[1]["Parent"] != nil {
		t.Fatalf("Lambda scope: %v", documents[1])
	}
	if err := m.Flush(); err != nil {
		t.Fatal(err)
	}
	documents = decodeDocuments(t, output.Bytes())
	if len(documents) != 3 || documents[2]["Parent"] != float64(1) || documents[2]["request"] != nil {
		t.Fatalf("parent scope: %v", documents)
	}
}
