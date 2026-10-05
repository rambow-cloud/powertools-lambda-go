package http

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	nethttp "net/http"
	"testing"
)

func TestExplicitTypedResponseEncoding(t *testing.T) {
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		for _, encoded := range []bool{false, true} {
			for _, form := range []string{"string", "bytes", "reader", "json"} {
				t.Run(fmt.Sprintf("%s/%t/%s", kind, encoded, form), func(t *testing.T) {
					payload := []byte("file")
					if encoded {
						payload = []byte{0, 1, 255}
					}
					var body any = payload
					var owned *observedBody
					if form == "string" {
						body = string(payload)
						if encoded {
							body = base64.StdEncoding.EncodeToString(payload)
						}
					} else if form == "reader" {
						owned = &observedBody{Reader: bytes.NewReader(payload)}
						body = owned
					} else if form == "json" {
						payload = []byte(`{"id":1}`)
						body = map[string]any{"id": 1}
					}
					app := New(Options{})
					if err := app.Get("/items", func(*RequestContext) (any, error) {
						return Response{StatusCode: 200, Headers: nethttp.Header{"Content-Type": {"image/png"}}, Body: body, IsBase64Encoded: &encoded}, nil
					}); err != nil {
						t.Fatal(err)
					}
					result, err := app.Resolve(context.Background(), httpAdapterEvent(t, kind, "GET", "/items"))
					want := string(payload)
					if encoded {
						want = base64.StdEncoding.EncodeToString(payload)
					}
					if err != nil || result.StatusCode != 200 || result.IsBase64Encoded != encoded || result.Body != want {
						t.Fatalf("explicit encoding: %+v / %v; want body=%q encoded=%t", result, err, want, encoded)
					}
					if owned != nil && owned.closed != 1 {
						t.Fatalf("owned body closed %d times", owned.closed)
					}
				})
			}
		}
	}
}

func TestPreencodedProxyValidationAndStreaming(t *testing.T) {
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", kind, streaming), func(t *testing.T) {
				payload := []byte(`{"id":1}`)
				checks := 0
				app := New(Options{})
				app.Use(Validate(ValidationConfig{Response: &ResponseChecks{Body: func(_ context.Context, value any) (any, []ValidationIssue, error) {
					checks++
					object, ok := value.(map[string]any)
					if !ok || object["id"] != float64(1) {
						t.Errorf("preencoded JSON validator input: %#v", value)
					}
					return value, nil, nil
				}}}))
				if err := app.Get("/items", func(*RequestContext) (any, error) {
					return map[string]any{"statusCode": 200, "body": base64.StdEncoding.EncodeToString(payload), "isBase64Encoded": true}, nil
				}); err != nil {
					t.Fatal(err)
				}
				event := httpAdapterEvent(t, kind, "GET", "/items")
				if streaming {
					var output bytes.Buffer
					if err := app.ResolveStream(context.Background(), event, &output); err != nil {
						t.Fatal(err)
					}
					separator := bytes.Index(output.Bytes(), make([]byte, 8))
					if separator < 0 || !bytes.Equal(output.Bytes()[separator+8:], payload) {
						t.Fatalf("stream did not contain raw decoded body: %q", output.Bytes())
					}
				} else {
					result, err := app.Resolve(context.Background(), event)
					if err != nil || result.StatusCode != 200 || !result.IsBase64Encoded || result.Body != base64.StdEncoding.EncodeToString(payload) {
						t.Fatalf("validated binary response: %+v / %v", result, err)
					}
				}
				if checks != 1 {
					t.Fatalf("body schema called %d times", checks)
				}
			})
		}
	}
}

func TestAlreadyCompressedProxyAndMalformedBase64(t *testing.T) {
	payload := []byte{0, 1, 255}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		for _, malformed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/malformed=%t", kind, malformed), func(t *testing.T) {
				threshold := float64(0)
				app := New(Options{})
				app.Use(Compress(CompressionOptions{Threshold: &threshold}))
				wire := base64.StdEncoding.EncodeToString(compressed.Bytes())
				if malformed {
					wire = "encoded?"
				}
				if err := app.Get("/items", func(*RequestContext) (any, error) {
					return map[string]any{"statusCode": 200, "body": wire, "isBase64Encoded": true, "headers": map[string]string{"content-encoding": "gzip"}}, nil
				}); err != nil {
					t.Fatal(err)
				}
				event := httpAdapterEvent(t, kind, "GET", "/items")
				event["headers"] = map[string]any{"accept-encoding": "gzip"}
				result, err := app.Resolve(context.Background(), event)
				if err != nil {
					t.Fatal(err)
				}
				if malformed {
					if result.StatusCode != 500 {
						t.Fatalf("malformed base64 did not fail: %+v", result)
					}
				} else if result.StatusCode != 200 || !result.IsBase64Encoded || result.Body != wire || !bytes.Equal(decodeCompressed(t, "gzip", result.Body), payload) {
					t.Fatalf("already compressed proxy changed: %+v", result)
				}
			})
		}
	}
}

func TestResponseEncodingFlagSnapshot(t *testing.T) {
	encoded := true
	request := &RequestContext{Response: &nethttp.Response{Header: make(nethttp.Header), Body: nethttp.NoBody}}
	if err := request.Respond(Response{StatusCode: 200, Body: "AAH/", IsBase64Encoded: &encoded}); err != nil {
		t.Fatal(err)
	}
	encoded = false
	if request.IsBase64Encoded == nil || !*request.IsBase64Encoded {
		t.Fatal("caller encoding flag was retained by reference")
	}
	result, err := WebResponseToProxyResult(request.Response, APIGatewayV2, *request.IsBase64Encoded)
	raw, decodeErr := base64.StdEncoding.DecodeString(result.Body)
	if err != nil || decodeErr != nil || !bytes.Equal(raw, []byte{0, 1, 255}) {
		t.Fatalf("encoding snapshot: %+v / %v / %v", result, err, decodeErr)
	}
}

func TestPreencodedProxyBinaryStreaming(t *testing.T) {
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		t.Run(string(kind), func(t *testing.T) {
			app := New(Options{})
			if err := app.Get("/items", func(*RequestContext) (any, error) {
				return map[string]any{"statusCode": 200, "body": "AAH/", "isBase64Encoded": true, "headers": map[string]string{"content-type": "application/octet-stream"}}, nil
			}); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := app.ResolveStream(context.Background(), httpAdapterEvent(t, kind, "GET", "/items"), &output); err != nil {
				t.Fatal(err)
			}
			separator := bytes.Index(output.Bytes(), make([]byte, 8))
			if separator < 0 || !bytes.Equal(output.Bytes()[separator+8:], []byte{0, 1, 255}) {
				t.Fatalf("stream lost raw binary bytes: %q", output.Bytes())
			}
		})
	}
}
