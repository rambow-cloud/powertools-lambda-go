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
	"testing"
)

func TestBuiltinMiddlewareConcurrentSnapshots(t *testing.T) {
	threshold, maxAge := float64(0), float64(60)
	origins, methods, headers := []string{"https://a.test"}, []string{"GET"}, []string{"X-Token"}
	app := New(Options{})
	app.Use(CORS(CORSOptions{Origins: origins, AllowMethods: methods, AllowHeaders: headers, MaxAge: &maxAge}), Compress(CompressionOptions{Threshold: &threshold}))
	origins[0], methods[0], headers[0], threshold, maxAge = "changed", "changed", "changed", 10000, 0
	if err := app.Get("/items/:id", func(request *RequestContext) (any, error) {
		return Response{StatusCode: 200, Body: request.Params["id"]}, nil
	}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for index := range 64 {
		group.Go(func() {
			id := fmt.Sprint(index)
			event := strings.Replace(string(testEvent("/items/"+id)), `"headers":{}`, `"headers":{"origin":"https://a.test"}`, 1)
			// Raw JSON retains the original per-invocation request snapshot.
			result, err := app.Resolve(context.Background(), json.RawMessage(event))
			if err != nil || result.StatusCode != 200 || result.Headers["access-control-allow-origin"] != "https://a.test" || result.Headers["vary"] != "Origin" || !result.IsBase64Encoded {
				t.Errorf("response: %+v, %v", result, err)
				return
			}
			if got := string(decodeCompressed(t, "gzip", result.Body)); got != id {
				t.Errorf("body: %q; want %q", got, id)
			}
		})
	}
	group.Wait()
	preflight := strings.Replace(string(testEvent("/missing")), `"method":"GET"`, `"method":"OPTIONS"`, 1)
	preflight = strings.Replace(preflight, `"headers":{}`, `"headers":{"origin":"https://a.test","access-control-request-method":"GET","access-control-request-headers":"X-Token"}`, 1)
	result, err := app.Resolve(context.Background(), json.RawMessage(preflight))
	if err != nil || result.StatusCode != 204 || result.Headers["access-control-max-age"] != "60" {
		t.Fatalf("preflight after option mutation: %+v, %v", result, err)
	}
}

type failingCompressionReader struct{ err error }

func (r failingCompressionReader) Read([]byte) (int, error) { return 0, r.err }

func TestCompressionLifecycle(t *testing.T) {
	for _, explicitLength := range []bool{false, true} {
		for _, mode := range []string{"success", "skip", "read-error", "close-error", "cancel", "next-error", "panic"} {
			t.Run(fmt.Sprintf("%t/%s", explicitLength, mode), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				failure := errors.New("body failure")
				body := &observedBody{Reader: strings.NewReader("hello")}
				if mode == "read-error" {
					body.Reader = failingCompressionReader{failure}
				}
				if mode == "close-error" {
					body.closeError = failure
				}
				headers := make(nethttp.Header)
				if explicitLength {
					headers.Set("Content-Length", "5")
				}
				request := &RequestContext{Context: ctx, Request: &nethttp.Request{Method: "GET", Header: make(nethttp.Header)}, Response: &nethttp.Response{StatusCode: 200, Header: headers, Body: body}}
				if mode == "skip" {
					request.Request.Header.Set("Accept-Encoding", "identity")
				}
				threshold := float64(0)
				var got error
				func() {
					defer func() {
						if mode == "panic" && recover() != failure {
							t.Error("panic identity changed")
						}
					}()
					got = Compress(CompressionOptions{Threshold: &threshold})(request, func() error {
						switch mode {
						case "cancel":
							cancel()
						case "next-error":
							return failure
						case "panic":
							panic(failure)
						}
						return nil
					})
				}()
				// The invocation owner closes any body not consumed by middleware.
				_ = request.Response.Body.Close()
				if body.closed != 1 {
					t.Fatalf("body closed %d times", body.closed)
				}
				switch mode {
				case "read-error", "close-error", "next-error":
					if !errors.Is(got, failure) {
						t.Fatalf("error identity: %v", got)
					}
				case "cancel":
					if !errors.Is(got, context.Canceled) {
						t.Fatalf("cancellation: %v", got)
					}
				default:
					if got != nil {
						t.Fatal(got)
					}
				}
			})
		}
	}
}

func TestCompressionWriterFailure(t *testing.T) {
	failure := errors.New("write failed")
	body := &observedBody{Reader: strings.NewReader("hello")}
	writer := &failingCompressionWriter{err: failure}
	if err := compressBody(context.Background(), writer, body); !errors.Is(err, failure) || body.closed != 1 || writer.closed != 1 {
		t.Fatalf("writer error=%v, closes=%d/%d", err, body.closed, writer.closed)
	}
}

type failingCompressionWriter struct {
	err    error
	closed int
}

func (w *failingCompressionWriter) Write([]byte) (int, error) { return 0, w.err }
func (w *failingCompressionWriter) Close() error              { w.closed++; return nil }

var _ io.WriteCloser = (*failingCompressionWriter)(nil)
