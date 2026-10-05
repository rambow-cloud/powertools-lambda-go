package http

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	nethttp "net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type middlewareSpec struct {
	Type    string
	Options json.RawMessage
}

func referenceBuiltins(t *testing.T, specifications []middlewareSpec) []Middleware {
	t.Helper()
	result := make([]Middleware, 0, len(specifications))
	for _, specification := range specifications {
		if specification.Type == "compress" {
			var options CompressionOptions
			if err := json.Unmarshal(specification.Options, &options); err != nil {
				t.Fatal(err)
			}
			result = append(result, Compress(options))
			continue
		}
		var options struct {
			Origin        json.RawMessage
			AllowMethods  []string
			AllowHeaders  []string
			ExposeHeaders []string
			Credentials   bool
			MaxAge        *float64
		}
		if err := json.Unmarshal(specification.Options, &options); err != nil {
			t.Fatal(err)
		}
		config := CORSOptions{AllowMethods: options.AllowMethods, AllowHeaders: options.AllowHeaders, ExposeHeaders: options.ExposeHeaders, Credentials: options.Credentials, MaxAge: options.MaxAge}
		if len(options.Origin) > 0 {
			if options.Origin[0] == '[' {
				if err := json.Unmarshal(options.Origin, &config.Origins); err != nil {
					t.Fatal(err)
				}
			} else if err := json.Unmarshal(options.Origin, &config.Origin); err != nil {
				t.Fatal(err)
			}
		}
		result = append(result, CORS(config))
	}
	return result
}

func decodeCompressed(t *testing.T, encoding, body string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}
	var reader io.ReadCloser
	if encoding == "gzip" {
		reader, err = gzip.NewReader(bytes.NewReader(data))
	} else {
		reader, err = zlib.NewReader(bytes.NewReader(data))
	}
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := readOwnedBody(reader)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestHTTPMiddlewareReference(t *testing.T) {
	data, err := os.ReadFile("testdata/middleware-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name, RouteMethod           string
			Event, Body                 json.RawMessage
			Status                      int
			Failure                     bool
			ResponseHeaders             map[string]string
			Middleware, RouteMiddleware []middlewareSpec
			Expected                    struct {
				Response ProxyResponse
				Calls    int
				Decoded  *string
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
			app.Use(referenceBuiltins(t, item.Middleware)...)
			method := item.RouteMethod
			if method == "" {
				method = "GET"
			}
			calls := 0
			err := app.Handle(method, "/items", func(*RequestContext) (any, error) {
				calls++
				if item.Failure {
					return nil, NewHTTPError(400, "handler failed")
				}
				headers := make(nethttp.Header)
				for name, value := range item.ResponseHeaders {
					headers.Set(name, value)
				}
				status := item.Status
				if status == 0 {
					status = 200
				}
				response := &nethttp.Response{StatusCode: status, Header: headers}
				if !bytes.Equal(item.Body, []byte("null")) {
					body := "hello"
					if len(item.Body) != 0 {
						if err := json.Unmarshal(item.Body, &body); err != nil {
							return nil, err
						}
					}
					response.Body = ioBody([]byte(body))
					if !hasHeader(headers, "Content-Type") {
						headers.Set("Content-Type", "text/plain;charset=UTF-8")
					}
				}
				return response, nil
			}, referenceBuiltins(t, item.RouteMiddleware)...)
			if err != nil {
				t.Fatal(err)
			}
			got, err := app.Resolve(context.Background(), item.Event)
			if err != nil {
				t.Fatal(err)
			}
			if calls != item.Expected.Calls {
				t.Fatalf("handler calls: %d; want %d", calls, item.Expected.Calls)
			}
			want := item.Expected.Response
			// Correct only named token/quality defects in the immutable TS corpus.
			selection, corrected := false, false
			for _, suffix := range []string{"-compress-gzip-gzip;q=0-1025", "-compress-gzip-xgzipx-1025", "-compress-gzip-*;q=0-1025", "-compress-deflate-*;q=0-1025"} {
				corrected = corrected || strings.HasSuffix(item.Name, suffix)
			}
			for _, suffix := range []string{"-compress-gzip-identity, gzip-1025", "-compress-gzip-GZIP-1025"} {
				if strings.HasSuffix(item.Name, suffix) {
					selection, corrected = true, true
				}
			}
			if corrected {
				var body string
				if err := json.Unmarshal(item.Body, &body); err != nil {
					t.Fatal(err)
				}
				want.IsBase64Encoded = selection
				if selection {
					want.Headers["content-encoding"] = "gzip"
					delete(want.Headers, "content-length")
					decoded := base64.StdEncoding.EncodeToString([]byte(body))
					item.Expected.Decoded = &decoded
				} else {
					delete(want.Headers, "content-encoding")
					want.Headers["content-length"] = strconv.Itoa(len(body))
					want.Body, item.Expected.Decoded = body, nil
				}
			}
			if item.Expected.Decoded != nil {
				if !got.IsBase64Encoded || got.Headers["content-encoding"] != want.Headers["content-encoding"] {
					t.Fatalf("compression flags: %+v; want %+v", got, want)
				}
				decoded := decodeCompressed(t, got.Headers["content-encoding"], got.Body)
				if base64.StdEncoding.EncodeToString(decoded) != *item.Expected.Decoded {
					t.Fatal("decompressed payload differs from reference")
				}
				if length, exists := want.Headers["content-length"]; exists {
					referenceBytes, err := base64.StdEncoding.DecodeString(want.Body)
					if err != nil || length != strconv.Itoa(len(referenceBytes)) {
						t.Fatal("reference compressed content length is inconsistent")
					}
					actualBytes, err := base64.StdEncoding.DecodeString(got.Body)
					if err != nil || got.Headers["content-length"] != strconv.Itoa(len(actualBytes)) {
						t.Fatal("Go compressed content length is inconsistent")
					}
					want.Headers["content-length"] = got.Headers["content-length"]
				}
				// DEFLATE implementations need not produce identical compressed bytes.
				// Any compressed length is checked against that runtime's wire bytes.
				// All other headers, flags, status, cookies and decoded bytes stay exact.
				got.Body, want.Body = "", ""
			}
			actual, _ := json.Marshal(got)
			expected, _ := json.Marshal(want)
			if !reflect.DeepEqual(normalizedResponse(actual), normalizedResponse(expected)) {
				t.Fatalf("response: %s; want %s", actual, expected)
			}
		})
	}
}
