package metrics_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
	powermetrics "github.com/rambow-cloud/powertools-lambda-go/metrics"
)

func requestEvent(id string) map[string]any {
	return map[string]any{"version": "2.0", "routeKey": "$default", "rawPath": "/items/" + id, "rawQueryString": "", "headers": map[string]string{}, "isBase64Encoded": false,
		"requestContext": map[string]any{"requestId": id, "apiId": "api", "domainName": "api.test", "http": map[string]string{"method": "GET", "sourceIp": "192.0.2.1"}}}
}

func TestConcurrentScopes(t *testing.T) {
	var output bytes.Buffer
	m := newMetrics(t, &output)
	if err := m.AddMetric("Parent", powermetrics.Count, 1); err != nil {
		t.Fatal(err)
	}
	app := httpapi.New(httpapi.Options{})
	app.Use(httpmetrics.New(m, httpmetrics.Options{CaptureRequestCount: true}))
	late := make(chan *powermetrics.Metrics, 64)
	if err := app.Get("/items/:id", func(r *httpapi.RequestContext) (any, error) {
		if r.Request.Context() != r.Context {
			return nil, errors.New("request context was not bound")
		}
		bound := m.WithContext(r.Context)
		late <- bound
		return "ok", bound.AddMetadata("id", r.Params["id"])
	}); err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for i := range 64 {
		wait.Go(func() {
			response, err := app.Resolve(context.Background(), requestEvent(fmt.Sprint(i)))
			if err != nil || response.StatusCode != 200 {
				t.Errorf("resolve: %v, %d", err, response.StatusCode)
			}
		})
	}
	wait.Wait()
	close(late)
	for bound := range late {
		if err := bound.AddMetric("Late", powermetrics.Count, 1); !errors.Is(err, powermetrics.ErrInvocationClosed) {
			t.Fatalf("late write: %v", err)
		}
	}
	documents := decodeDocuments(t, output.Bytes())
	if len(documents) != 64 {
		t.Fatalf("documents: %d", len(documents))
	}
	seen := map[string]bool{}
	for _, document := range documents {
		assertRequestCount(t, document)
		id := document["id"].(string)
		if seen[id] || document["apiGwRequestId"] != id || document["route"] != "GET /items/:id" || document["Parent"] != nil {
			t.Fatalf("scope leak: %v", document)
		}
		seen[id] = true
	}
	if err := m.Flush(); err != nil {
		t.Fatal(err)
	}
	documents = decodeDocuments(t, output.Bytes())
	if len(documents) != 65 || documents[64]["Parent"] != float64(1) || documents[64]["route"] != nil {
		t.Fatal("parent state was changed")
	}
}

type failedWriter struct {
	err    error
	output *bytes.Buffer
}

func (w failedWriter) Write(data []byte) (int, error) {
	_, _ = w.output.Write(data)
	return 0, w.err
}

func TestFailureAndContextRestoration(t *testing.T) {
	writeFailure := errors.New("output unavailable")
	for _, mode := range []string{"success", "error", "panic", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			var attempted bytes.Buffer
			m, err := powermetrics.New(powermetrics.WithNamespace("Test"), powermetrics.WithOutput(failedWriter{writeFailure, &attempted}), powermetrics.WithDisabled(false))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.test/items", nil)
			r := &httpapi.RequestContext{Context: ctx, Request: req, Response: &http.Response{StatusCode: 200}, ResponseType: httpapi.ALB}
			business := errors.New("business failure")
			var bound *powermetrics.Metrics
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				err = httpmetrics.New(m, httpmetrics.Options{CaptureRequestCount: true})(r, func() error {
					bound = m.WithContext(r.Context)
					switch mode {
					case "error":
						return business
					case "panic":
						panic(business)
					case "cancel":
						cancel()
						return ctx.Err()
					}
					return nil
				})
			}()
			if r.Context != ctx || r.Request != req {
				t.Fatal("context not restored")
			}
			documents := decodeDocuments(t, attempted.Bytes())
			if len(documents) != 1 {
				t.Fatalf("publication attempts: %d", len(documents))
			}
			assertRequestCount(t, documents[0])
			if mode == "panic" {
				if recovered != business {
					t.Fatalf("panic: %v", recovered)
				}
			} else {
				if recovered != nil || !errors.Is(err, writeFailure) {
					t.Fatalf("publication error: %v, panic: %v", err, recovered)
				}
				if mode == "error" && !errors.Is(err, business) {
					t.Fatal("business error lost")
				}
				if mode == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatal("cancellation lost")
				}
			}
			if err := bound.AddMetric("Late", powermetrics.Count, 1); !errors.Is(err, powermetrics.ErrInvocationClosed) {
				t.Fatalf("scope not closed: %v", err)
			}
		})
	}
}

type inspectedReader struct {
	io.Reader
	beforeRead func()
	closes     int
}

func (r *inspectedReader) Read(p []byte) (int, error) { r.beforeRead(); return r.Reader.Read(p) }
func (r *inspectedReader) Close() error               { r.closes++; return nil }

func TestStreamingPublishesBeforeBodyTransfer(t *testing.T) {
	var output, wire bytes.Buffer
	m := newMetrics(t, &output)
	app := httpapi.New(httpapi.Options{})
	app.Use(httpmetrics.New(m, httpmetrics.Options{CaptureRequestCount: true}))
	var bound *powermetrics.Metrics
	body := &inspectedReader{Reader: strings.NewReader("streamed body")}
	body.beforeRead = func() {
		documents := decodeDocuments(t, output.Bytes())
		if len(documents) != 1 {
			t.Fatal("metrics were not published before body reads")
		}
		assertRequestCount(t, documents[0])
		if err := bound.AddMetric("Late", powermetrics.Count, 1); !errors.Is(err, powermetrics.ErrInvocationClosed) {
			t.Fatalf("stream scope: %v", err)
		}
	}
	if err := app.Get("/items/:id", func(r *httpapi.RequestContext) (any, error) { bound = m.WithContext(r.Context); return body, nil }); err != nil {
		t.Fatal(err)
	}
	if err := app.ResolveStream(context.Background(), requestEvent("stream"), &wire); err != nil {
		t.Fatal(err)
	}
	if body.closes != 1 || !bytes.HasSuffix(wire.Bytes(), []byte("streamed body")) {
		t.Fatalf("stream ownership: %d, %q", body.closes, wire.Bytes())
	}
	if len(decodeDocuments(t, output.Bytes())) != 1 {
		t.Fatal("stream transfer republished request metrics")
	}
}
