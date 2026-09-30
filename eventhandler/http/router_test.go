package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func testEvent(path string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"version":"2.0","routeKey":"$default","rawPath":%q,"rawQueryString":"","headers":{},"requestContext":{"http":{"method":"GET"},"domainName":"api.example.test"},"isBase64Encoded":false}`, path))
}

func TestConcurrentInvocationIsolation(t *testing.T) {
	app := New(Options{})
	app.Shared.Set("service", "orders")
	var active, cleaned atomic.Int32
	app.Use(func(request *RequestContext, next Next) error {
		if request.Store.Has("id") {
			return fmt.Errorf("request state leaked")
		}
		request.Store.Set("id", request.Params["id"])
		active.Add(1)
		defer cleaned.Add(1)
		return next()
	})
	type identityKey struct{}
	if err := app.Get("/items/:id", func(request *RequestContext) (any, error) {
		identity := request.Context.Value(identityKey{})
		if request.Request.Context().Value(identityKey{}) != identity {
			return nil, fmt.Errorf("context replaced")
		}
		id, _ := request.Store.Get("id")
		service, _ := request.Shared.Get("service")
		return map[string]any{"id": id, "context": identity, "service": service}, nil
	}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for index := range 64 {
		group.Go(func() {
			id := fmt.Sprint(index)
			ctx := context.WithValue(context.Background(), identityKey{}, id)
			result, err := app.Resolve(ctx, testEvent("/items/"+id))
			var value map[string]any
			_ = json.Unmarshal([]byte(result.Body), &value)
			if err != nil || result.StatusCode != 200 || value["id"] != id || value["context"] != id || value["service"] != "orders" {
				t.Errorf("result: %#v, %v", result, err)
			}
		})
	}
	group.Wait()
	if active.Load() != 64 || cleaned.Load() != 64 {
		t.Fatalf("lifecycle: %d/%d", active.Load(), cleaned.Load())
	}
}

func TestCancellationAndPanicCleanup(t *testing.T) {
	for _, mode := range []string{"cancel", "panic"} {
		t.Run(mode, func(t *testing.T) {
			app := New(Options{})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cleaned := false
			app.Use(func(request *RequestContext, next Next) error { defer func() { cleaned = true }(); return next() })
			sentinel := &struct{ Value string }{"panic identity"}
			_ = app.Get("/items/a", func(*RequestContext) (any, error) {
				if mode == "panic" {
					panic(sentinel)
				}
				cancel()
				return "unused", nil
			})
			if mode == "panic" {
				func() {
					defer func() {
						if recovered := recover(); recovered != sentinel {
							t.Errorf("panic identity: %v", recovered)
						}
					}()
					_, _ = app.Resolve(ctx, testEvent("/items/a"))
				}()
			} else if _, err := app.Resolve(ctx, testEvent("/items/a")); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation: %v", err)
			}
			if !cleaned {
				t.Fatal("middleware cleanup did not run")
			}
		})
	}
}

type observedBody struct {
	io.Reader
	closed     int
	closeError error
}

func (b *observedBody) Close() error { b.closed++; return b.closeError }
func TestResponseBodyOwnership(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			body := &observedBody{Reader: strings.NewReader("hello")}
			if fail {
				body.closeError = errors.New("close failed")
			}
			app := New(Options{})
			_ = app.Get("/items/a", func(*RequestContext) (any, error) {
				return &nethttp.Response{StatusCode: 200, Header: nethttp.Header{"Content-Type": []string{"text/plain"}}, Body: body}, nil
			})
			result, err := app.Resolve(context.Background(), testEvent("/items/a"))
			want := 200
			if fail {
				want = 500
			}
			if err != nil || body.closed != 1 || result.StatusCode != want {
				t.Fatalf("response %#v, closes %d, err %v", result, body.closed, err)
			}
		})
	}
}

func TestRequestSnapshotAndMiddlewareReuse(t *testing.T) {
	event := testEvent("/items/a")
	before := string(event)
	app := New(Options{})
	app.Use(func(request *RequestContext, next Next) error {
		request.Event[0] = '['
		request.Params["id"] = "changed"
		request.Request.Header.Set("x-mutated", "yes")
		return next()
	})
	_ = app.Get("/items/:id", func(request *RequestContext) (any, error) { return request.Params["id"], nil })
	for range 2 {
		result, err := app.Resolve(context.Background(), event)
		if err != nil || result.Body != `"changed"` || string(event) != before {
			t.Fatalf("snapshot: %#v, %v", result, err)
		}
	}
}
