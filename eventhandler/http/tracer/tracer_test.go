package tracer_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httptracer "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer"
	powertracer "github.com/rambow-cloud/powertools-lambda-go/tracer"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ownedBody struct {
	io.Reader
	readErr, closeErr error
	panicValue        any
	closes            int
}

func (b *ownedBody) Read(p []byte) (int, error) {
	if b.panicValue != nil {
		panic(b.panicValue)
	}
	if b.readErr != nil {
		return 0, b.readErr
	}
	return b.Reader.Read(p)
}
func (b *ownedBody) Close() error { b.closes++; return b.closeErr }

func TestLifecycleAndParentRestoration(t *testing.T) {
	for _, mode := range []string{"success", "error", "panic", "read-error", "read-panic", "close-error", "cancel", "streaming"} {
		t.Run(mode, func(t *testing.T) {
			tr, recorder := newTracer(t)
			parent, end := tr.StartSpan(context.Background(), "parent")
			defer end(nil)
			parent, cancel := context.WithCancel(parent)
			defer cancel()
			req, _ := http.NewRequestWithContext(parent, "GET", "https://api.test/items?secret=omit", nil)
			body := &ownedBody{Reader: strings.NewReader(`{"ok":true}`)}
			failure := errors.New("original failure")
			switch mode {
			case "read-error":
				body.readErr = failure
			case "read-panic":
				body.panicValue = failure
			case "close-error":
				body.closeErr = failure
			}
			r := &httpapi.RequestContext{Context: parent, Request: req, Response: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: body}, ResponseType: httpapi.ALB, Route: "GET /items", IsHTTPStreaming: mode == "streaming"}
			var recovered any
			var err error
			func() {
				defer func() { recovered = recover() }()
				err = httptracer.New(tr)(r, func() error {
					if r.Context != r.Request.Context() {
						t.Fatal("request context not propagated")
					}
					switch mode {
					case "error":
						return failure
					case "panic":
						panic(failure)
					case "cancel":
						cancel()
						return parent.Err()
					}
					return nil
				})
			}()
			if r.Context != parent || r.Request != req {
				t.Fatal("parent context not restored")
			}
			if mode == "streaming" {
				if len(recorder.Ended()) != 0 || body.closes != 0 {
					t.Fatal("streaming middleware should be skipped")
				}
			} else {
				spans := recorder.Ended()
				if len(spans) != 1 || spans[0].Parent().SpanID() != trace.SpanContextFromContext(parent).SpanID() {
					t.Fatal("span parent or closure lost")
				}
				attrs := attributes(spans[0])
				if attrs["url.full"] != "https://api.test/items" || attrs["http.route"] != "/items" {
					t.Fatal("query leaked or route template missing")
				}
				if mode == "success" {
					data, err := io.ReadAll(r.Response.Body)
					if err != nil || string(data) != `{"ok":true}` {
						t.Fatal("response bytes changed")
					}
				} else if spans[0].Status().Code != codes.Error {
					t.Fatal("failure not recorded")
				}
			}
			switch mode {
			case "panic", "read-panic":
				if recovered != failure {
					t.Fatalf("panic identity: %v", recovered)
				}
			case "error", "read-error", "close-error":
				if err != failure {
					t.Fatalf("error identity: %v", err)
				}
			case "cancel":
				if err != context.Canceled {
					t.Fatalf("cancellation: %v", err)
				}
			default:
				if err != nil || recovered != nil {
					t.Fatalf("unexpected failure: %v, %v", err, recovered)
				}
			}
			_ = r.Response.Body.Close()
			if body.closes != 1 {
				t.Fatalf("body closed %d times", body.closes)
			}
		})
	}
}

func TestConcurrentRouteParents(t *testing.T) {
	tr, recorder := newTracer(t)
	middleware := httptracer.New(tr, httptracer.Options{DisableCaptureResponse: true})
	var wait sync.WaitGroup
	for i := range 32 {
		wait.Go(func() {
			ctx, end := tr.StartSpan(context.Background(), fmt.Sprintf("parent-%d", i))
			defer end(nil)
			req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.test/items/%d", i), nil)
			r := &httpapi.RequestContext{Context: ctx, Request: req, ResponseType: httpapi.ALB, Route: "GET /items/:id", Response: &http.Response{StatusCode: 204, Header: http.Header{}}}
			if err := middleware(r, func() error { return tr.PutAnnotation(r.Context, "caller", i) }); err != nil {
				t.Error(err)
			}
		})
	}
	wait.Wait()
	spans := recorder.Ended()
	if len(spans) != 64 {
		t.Fatalf("spans: %d", len(spans))
	}
	parents := map[trace.SpanID]string{}
	for _, span := range spans {
		if strings.HasPrefix(span.Name(), "parent-") {
			parents[span.SpanContext().SpanID()] = span.Name()
		}
	}
	for _, span := range spans {
		if !strings.HasPrefix(span.Name(), "GET ") {
			continue
		}
		id := attributes(span)["caller"].(int64)
		if parents[span.Parent().SpanID()] != fmt.Sprintf("parent-%d", id) || span.Name() != fmt.Sprintf("GET /items/%d", id) {
			t.Fatal("concurrent parent leak")
		}
	}
}

func TestCompressedBodyCaptureDisabled(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprint(nested), func(t *testing.T) {
			tr, recorder := newTracer(t, powertracer.WithCaptureResponse(true))
			app := httpapi.New(httpapi.Options{})
			threshold := float64(0)
			app.Use(httptracer.New(tr, httptracer.Options{DisableCaptureResponse: true}), httpapi.Compress(httpapi.CompressionOptions{Threshold: &threshold}))
			var routeMiddleware []httpapi.Middleware
			if nested {
				routeMiddleware = append(routeMiddleware, httpapi.Compress(httpapi.CompressionOptions{Threshold: &threshold}))
			}
			if err := app.Get("/items", func(*httpapi.RequestContext) (any, error) { return map[string]any{"ok": true}, nil }, routeMiddleware...); err != nil {
				t.Fatal(err)
			}
			event := map[string]any{"httpMethod": "GET", "path": "/items", "headers": map[string]string{}, "requestContext": map[string]any{"elb": map[string]any{}}, "body": nil, "isBase64Encoded": false}
			response, err := app.Resolve(context.Background(), event)
			if err != nil || response.StatusCode != 200 {
				t.Fatalf("response: %v, %d", err, response.StatusCode)
			}
			data, err := base64.StdEncoding.DecodeString(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if len(recorder.Ended()) != 1 || !response.IsBase64Encoded {
				t.Fatal("compressed response or route span missing")
			}
			attrs := attributes(recorder.Ended()[0])
			if nested {
				// The outer compressor measures the inner compressed body, then skips
				// re-encoding because Content-Encoding is already present.
				length, err := strconv.Atoi(response.Headers["content-length"])
				if err != nil || length != len(data) || attrs["http.response.body.size"] != int64(length) {
					t.Fatal("existing wire length was not recorded")
				}
			} else if response.Headers["content-length"] != "" || attrs["http.response.body.size"] != nil {
				t.Fatal("compression removed Content-Length; tracing must not invent it")
			}
		})
	}
}
