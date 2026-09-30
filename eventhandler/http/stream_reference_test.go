package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	nethttp "net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

func splitStream(t *testing.T, data []byte) (map[string]any, []byte) {
	t.Helper()
	metadata, body, found := bytes.Cut(data, make([]byte, 8))
	if !found {
		t.Fatal("missing Lambda metadata delimiter")
	}
	var value map[string]any
	if err := json.Unmarshal(metadata, &value); err != nil {
		t.Fatal(err)
	}
	return value, body
}

func TestHTTPStreamReference(t *testing.T) {
	data, err := os.ReadFile("testdata/stream-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name, Method, Action    string
			Event, Value, Body      json.RawMessage
			Status                  int
			Bytes                   []byte
			Chunks                  []string
			Headers                 map[string]string
			MultiValueHeaders       map[string][]string
			CORS, Compress, Missing bool
			Expected                struct {
				Metadata map[string]any
				Body     string
				Ended    bool
				Error    any
			}
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, item := range corpus.Cases {
		t.Run(item.Name, func(t *testing.T) {
			disabled := false
			app := New(Options{Debug: &disabled})
			if item.CORS {
				app.Use(CORS(CORSOptions{Origins: []string{"https://app.test"}, ExposeHeaders: []string{"X-A", "X-B"}}))
			}
			if item.Compress {
				zero := float64(0)
				app.Use(Compress(CompressionOptions{Threshold: &zero}))
			}
			app.Use(func(request *RequestContext, next Next) error {
				request.Response.Header.Set("X-Before", "yes")
				if err := next(); err != nil {
					return err
				}
				request.Response.Header.Set("X-After", "yes")
				return nil
			})
			if !item.Missing {
				method := item.Method
				if method == "" {
					method = "GET"
				}
				err := app.Handle(method, "/items/:id", func(request *RequestContext) (any, error) {
					switch item.Action {
					case "error":
						return nil, NewHTTPError(400, "bad stream")
					case "inspect":
						return map[string]any{"streaming": request.IsHTTPStreaming, "route": request.Route, "params": request.Params}, nil
					case "json":
						return item.Value, nil
					case "bytes":
						return item.Bytes, nil
					case "reader":
						var parts []io.Reader
						for _, chunk := range item.Chunks {
							data, err := base64.StdEncoding.DecodeString(chunk)
							if err != nil {
								return nil, err
							}
							parts = append(parts, bytes.NewReader(data))
						}
						return io.MultiReader(parts...), nil
					}
					status := item.Status
					if status == 0 {
						status = 200
					}
					headers := make(nethttp.Header)
					for key, value := range item.Headers {
						headers.Set(key, value)
					}
					multiHeaders := make(nethttp.Header)
					for key, values := range item.MultiValueHeaders {
						for _, value := range values {
							multiHeaders.Add(key, value)
						}
					}
					var body any
					if len(item.Body) > 0 && string(item.Body) != "null" {
						if err := json.Unmarshal(item.Body, &body); err != nil {
							return nil, err
						}
					}
					if item.Action == "proxy" {
						return Response{StatusCode: status, Headers: headers, MultiValueHeaders: multiHeaders, Body: body}, nil
					}
					response := &nethttp.Response{StatusCode: status, Header: headers}
					if body != nil {
						response.Body = io.NopCloser(strings.NewReader(body.(string)))
						if !hasHeader(headers, "Content-Type") {
							headers.Set("Content-Type", "text/plain;charset=UTF-8")
						}
					}
					return response, nil
				})
				if err != nil && method != "TRACE" {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			if err := app.ResolveStream(context.Background(), item.Event, &output); err != nil {
				t.Fatal(err)
			}
			metadata, body := splitStream(t, output.Bytes())
			if item.Expected.Error != nil || !item.Expected.Ended {
				t.Fatalf("unexpected reference failure: %+v", item.Expected)
			}
			if !reflect.DeepEqual(metadata, item.Expected.Metadata) {
				t.Fatalf("metadata: %#v; want %#v", metadata, item.Expected.Metadata)
			}
			want, err := base64.StdEncoding.DecodeString(item.Expected.Body)
			if err != nil {
				t.Fatal(err)
			}
			if headers, ok := metadata["headers"].(map[string]any); ok && strings.Contains(fmtString(headers["content-type"]), "application/json") {
				var actualJSON, expectedJSON any
				if json.Unmarshal(body, &actualJSON) == nil && json.Unmarshal(want, &expectedJSON) == nil {
					if !reflect.DeepEqual(actualJSON, expectedJSON) {
						t.Fatalf("JSON: %s; want %s", body, want)
					}
					return
				}
			}
			if !bytes.Equal(body, want) {
				t.Fatalf("body: %x; want %x", body, want)
			}
		})
	}
}

func fmtString(value any) string { result, _ := value.(string); return result }
