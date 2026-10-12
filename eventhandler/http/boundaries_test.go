package http

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPBoundaryReference(t *testing.T) {
	raw, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string `json:"version"`
		Cases   []struct {
			Name     string         `json:"name"`
			Mode     string         `json:"mode"`
			Input    jsontext.Value `json:"input"`
			Expected struct {
				Value   jsontext.Value `json:"value"`
				Error   string         `json:"error"`
				Message string         `json:"message"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "2.35.0" || len(fixture.Cases) != 88 {
		t.Fatalf("unexpected fixture version/count: %s/%d", fixture.Version, len(fixture.Cases))
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			var actual any
			var err error
			switch item.Mode {
			case "request", "url":
				var request *nethttp.Request
				request, _, err = ProxyEventToWebRequest(context.Background(), item.Input)
				// Go's native URL parser rejects invalid percent escapes. The
				// pinned WHATWG converter retains them; keep that observation.
				if strings.HasSuffix(item.Name, "-malformed-escape") {
					var conversion *RequestConversionError
					var native *url.Error
					if item.Expected.Error != "" || !errors.As(err, &conversion) || !errors.As(err, &native) {
						t.Fatalf("native escaped-URL difference: %v; upstream=%s", err, item.Expected.Value)
					}
					return
				}
				if err == nil {
					defer request.Body.Close()
					actual = map[string]any{"url": request.URL.String(), "headers": boundaryHeaders(request.Header)}
				}
			case "response":
				var response *nethttp.Response
				response, err = HandlerResultToWebResponse(item.Input)
				if err == nil {
					body, readErr := readOwnedBody(response.Body)
					if readErr != nil {
						t.Fatal(readErr)
					}
					cookies := response.Header.Values("Set-Cookie")
					if cookies == nil {
						cookies = []string{}
					}
					actual = map[string]any{"status": response.StatusCode, "headers": boundaryHeaders(response.Header), "cookies": cookies, "body": string(body)}
				}
			case "regex":
				var input struct {
					Pattern string         `json:"pattern"`
					Event   jsontext.Value `json:"event"`
				}
				if err := json.Unmarshal(item.Input, &input); err != nil {
					t.Fatal(err)
				}
				app := New(Options{})
				if err := app.HandleRegex("GET", input.Pattern, func(ctx *RequestContext) (any, error) {
					return map[string]any{"params": ctx.Params}, nil
				}); err != nil {
					t.Fatal(err)
				}
				actual, err = app.Resolve(context.Background(), input.Event)
			default:
				t.Fatalf("unknown fixture mode %s", item.Mode)
			}
			if item.Expected.Error != "" {
				if item.Expected.Error != "TypeError" || err == nil {
					t.Fatalf("error=%v; upstream=%s/%s", err, item.Expected.Error, item.Expected.Message)
				}
				if item.Mode != "response" {
					var conversion *RequestConversionError
					if !errors.As(err, &conversion) || conversion.Cause == nil {
						t.Fatalf("conversion cause lost: %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			var gotValue, wantValue any
			if item.Mode == "regex" {
				gotValue, wantValue = normalizedResponse(got), normalizedResponse(item.Expected.Value)
			} else {
				if err := json.Unmarshal(got, &gotValue); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(item.Expected.Value, &wantValue); err != nil {
					t.Fatal(err)
				}
				// The native Go Header retains every cookie. The pinned Web
				// Headers entries() exposes the final cookie, while getSetCookie()
				// retains both. Only this known projection difference is mapped.
				if item.Name == "response-cookies" {
					want := wantValue.(map[string]any)
					cookies := want["cookies"].([]any)
					if len(cookies) != 2 || want["headers"].(map[string]any)["set-cookie"] != cookies[1] {
						t.Fatalf("pinned cookie projection changed: %s", item.Expected.Value)
					}
					want["headers"].(map[string]any)["set-cookie"] = cookies[0].(string) + ", " + cookies[1].(string)
				}
			}
			if !reflect.DeepEqual(gotValue, wantValue) {
				t.Fatalf("got %s; upstream %s", got, item.Expected.Value)
			}
		})
	}
}

func boundaryHeaders(headers nethttp.Header) map[string]string {
	values := make(map[string]string, len(headers))
	for name, entries := range headers {
		values[strings.ToLower(name)] = strings.Join(entries, ", ")
	}
	return values
}

func TestNativeResponseHeaderBoundaries(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, form := range []string{"native", "typed", "multi", "cookie", "inherited"} {
			t.Run(fmt.Sprintf("stream=%t/%s", streaming, form), func(t *testing.T) {
				body := &observedBody{Reader: strings.NewReader("owned")}
				bad := nethttp.Header{"X-Test": {"bad\r\nvalue"}}
				var value any = Response{StatusCode: 200, Body: body}
				previous := nethttp.Header{}
				switch form {
				case "native":
					value = &nethttp.Response{StatusCode: 200, Header: bad, Body: body}
				case "typed":
					value = Response{StatusCode: 200, Headers: bad, Body: body}
				case "multi":
					value = Response{StatusCode: 200, MultiValueHeaders: bad, Body: body}
				case "cookie":
					value = Response{StatusCode: 200, Cookies: []string{"bad\r\nvalue"}, Body: body}
				case "inherited":
					previous = bad
				}
				response, _, err := handlerResponse(value, previous, 200, streaming)
				if err == nil {
					_ = response.Body.Close()
					t.Fatal("invalid response header accepted")
				}
				if body.closed != 1 {
					t.Fatalf("rejected owned body closed %d times", body.closed)
				}
				if bad["X-Test"][0] != "bad\r\nvalue" {
					t.Fatal("caller header mutated")
				}
			})
		}
	}
}

func TestRespondRetainedNativeBodyFailureOwnership(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			body := &observedBody{Reader: strings.NewReader("owned")}
			ctx := &RequestContext{Response: &nethttp.Response{StatusCode: 200, Header: nethttp.Header{"X-Test": {"bad\r\nvalue"}}, Body: body}, IsHTTPStreaming: streaming}
			if err := ctx.Respond(ctx.Response); err == nil {
				t.Fatal("invalid retained response accepted")
			}
			// The resolver's ordinary deferred cleanup must not close a body
			// already consumed or rejected by native response conversion.
			_ = ctx.Response.Body.Close()
			if body.closed != 1 {
				t.Fatalf("retained body closes=%d", body.closed)
			}
		})
	}
}

type boundaryShortWriter struct {
	calls    int
	metadata bool
	data     bytes.Buffer
}

func (w *boundaryShortWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.metadata || w.calls > 1 {
		n := len(p) - 1
		_, _ = w.data.Write(p[:n])
		return n, nil
	}
	return w.data.Write(p)
}

func TestStreamFramingAndShortWrites(t *testing.T) {
	for _, phase := range []string{"complete", "metadata", "body"} {
		t.Run(phase, func(t *testing.T) {
			body := &observedBody{Reader: bytes.NewReader([]byte{'a', 0, 0xff, 'z'})}
			app := New(Options{})
			if err := app.Get("/items/a", func(*RequestContext) (any, error) {
				return Response{StatusCode: 200, Headers: nethttp.Header{"X-Frame": {"quoted\"value"}}, Body: body}, nil
			}); err != nil {
				t.Fatal(err)
			}
			var complete bytes.Buffer
			short := &boundaryShortWriter{metadata: phase == "metadata"}
			var destination io.Writer = &complete
			if phase != "complete" {
				destination = short
			}
			err := app.ResolveStream(context.Background(), testEvent("/items/a"), destination)
			if body.closed != 1 {
				t.Fatalf("body closes %d", body.closed)
			}
			if phase != "complete" {
				if !errors.Is(err, io.ErrShortWrite) {
					t.Fatalf("short write cause: %v", err)
				}
				if phase == "metadata" && short.calls != 1 {
					t.Fatal("body written after incomplete metadata")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			metadata, wireBody := splitStream(t, complete.Bytes())
			if !bytes.Equal(wireBody, []byte{'a', 0, 0xff, 'z'}) || metadata["headers"].(map[string]any)["x-frame"] != "quoted\"value" {
				t.Fatalf("framing changed bytes/metadata: %v / %x", metadata, wireBody)
			}
			if len(metadata) != 2 {
				t.Fatalf("unexpected metadata fields: %v", metadata)
			}
		})
	}
}

func TestFinalResponseHeaderOwnership(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			body := &observedBody{Reader: strings.NewReader("owned")}
			app := New(Options{})
			app.Use(func(ctx *RequestContext, next Next) error {
				if err := next(); err != nil {
					return err
				}
				ctx.Response.Body = body
				ctx.Response.Header.Set("X-Test", "bad\r\nvalue")
				return nil
			})
			if err := app.Get("/items/a", func(*RequestContext) (any, error) { return "ok", nil }); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			var err error
			if streaming {
				err = app.ResolveStream(context.Background(), testEvent("/items/a"), &output)
			} else {
				_, err = app.Resolve(context.Background(), testEvent("/items/a"))
			}
			if err == nil || output.Len() != 0 || body.closed != 1 {
				t.Fatalf("final invalid headers: err=%v bytes=%d closes=%d", err, output.Len(), body.closed)
			}
		})
	}
	body := &observedBody{Reader: strings.NewReader("owned")}
	_, err := WebResponseToProxyResult(&nethttp.Response{StatusCode: 200, Header: nethttp.Header{"X-\x7f": {}}, Body: body}, APIGatewayV2, false)
	if err == nil || body.closed != 1 {
		t.Fatalf("direct conversion: err=%v closes=%d", err, body.closed)
	}
}

func TestMiddlewareStreamReplacementOwnership(t *testing.T) {
	for _, mode := range []string{"replace", "error", "panic", "retain"} {
		t.Run(mode, func(t *testing.T) {
			original := &observedBody{Reader: strings.NewReader("original")}
			replacement := &observedBody{Reader: strings.NewReader("replacement")}
			failure := errors.New("middleware failed")
			app := New(Options{})
			app.Use(func(ctx *RequestContext, next Next) error {
				if err := next(); err != nil {
					return err
				}
				switch mode {
				case "replace":
					return ctx.Respond(Response{StatusCode: 202, Body: replacement})
				case "error":
					return failure
				case "panic":
					panic(failure)
				default:
					return ctx.Respond(ctx.Response)
				}
			})
			if err := app.Get("/items/a", func(*RequestContext) (any, error) { return original, nil }); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				if err := app.ResolveStream(context.Background(), testEvent("/items/a"), &output); err != nil {
					t.Fatal(err)
				}
			}()
			if original.closed != 1 {
				t.Fatalf("abandoned/retained body closes=%d", original.closed)
			}
			if mode == "panic" {
				if recovered != failure || output.Len() != 0 {
					t.Fatalf("panic identity/framing: %v / %d", recovered, output.Len())
				}
				return
			}
			if recovered != nil {
				t.Fatalf("unexpected panic %v", recovered)
			}
			metadata, body := splitStream(t, output.Bytes())
			switch mode {
			case "replace":
				if replacement.closed != 1 || string(body) != "replacement" || metadata["statusCode"] != float64(202) {
					t.Fatalf("replacement ownership: %v/%q/%d", metadata, body, replacement.closed)
				}
			case "error":
				if metadata["statusCode"] != float64(500) {
					t.Fatalf("error framing: %v", metadata)
				}
			case "retain":
				if string(body) != "original" {
					t.Fatalf("retained body changed: %q", body)
				}
			}
		})
	}
}

func TestStreamifyConcurrentCloseAndSnapshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	event := testEvent("/items/a")
	original := string(event)
	var cleanups atomic.Int32
	var snapshot string
	// Use the existing public RawMessage signature; the producer reads the
	// snapshot after the caller changes its original input and cancels it.
	handler := Streamify(func(ctx context.Context, input jsonv1.RawMessage, writer io.Writer) error {
		defer cleanups.Add(1)
		_, err := writer.Write([]byte("first"))
		snapshot = string(input)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	})
	stream, err := handler(ctx, event)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	event[0] = '['
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			if err := stream.Close(); !errors.Is(err, context.Canceled) {
				t.Errorf("close cause: %v", err)
			}
		})
	}
	group.Wait()
	if snapshot != original || cleanups.Load() != 1 {
		t.Fatalf("snapshot/cleanup: %q / %d", snapshot, cleanups.Load())
	}
	if _, err := stream.Read(make([]byte, 1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("read after close: %v", err)
	}
}
