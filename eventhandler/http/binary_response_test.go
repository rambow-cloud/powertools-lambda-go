package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"testing"
)

func TestBinaryResponseContracts(t *testing.T) {
	payload := []byte{0, 1, 255}
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		for _, form := range []string{"proxy-map", "proxy-struct", "proxy-json", "proxy-false", "proxy-absent", "native-octet", "native-pdf", "native-invalid-utf8", "native-text", "typed-bytes", "typed-reader", "direct-bytes", "direct-reader", "context-false", "context-true"} {
			t.Run(fmt.Sprintf("%s/%s", kind, form), func(t *testing.T) {
				app := New(Options{})
				wantBytes, wantEncoded := payload, true
				var owned *observedBody
				if err := app.Get("/items", func(request *RequestContext) (any, error) {
					switch form {
					case "proxy-map", "proxy-json":
						value := map[string]any{"statusCode": 200, "body": "AAH/", "isBase64Encoded": true}
						if form == "proxy-json" {
							raw, err := json.Marshal(value)
							return json.RawMessage(raw), err
						}
						return value, nil
					case "proxy-struct":
						return ProxyResponse{StatusCode: 200, Headers: map[string]string{}, Body: "AAH/", IsBase64Encoded: true}, nil
					case "proxy-false", "proxy-absent":
						wantBytes = []byte("AAH/")
						value := map[string]any{"statusCode": 200, "body": "AAH/", "headers": map[string]any{"content-type": "image/png"}}
						if form == "proxy-false" {
							value["isBase64Encoded"], wantEncoded = false, false
						}
						return value, nil
					case "native-octet", "native-pdf", "native-invalid-utf8", "native-text":
						media := "text/plain"
						if form == "native-octet" {
							media, wantBytes = "Application/Octet-Stream; charset=binary", []byte("file")
						} else if form == "native-pdf" {
							media, wantBytes = "application/pdf", []byte("%PDF-1.7")
						} else if form == "native-text" {
							wantBytes, wantEncoded = []byte("hello λ 世界"), false
						}
						owned = &observedBody{Reader: bytes.NewReader(wantBytes)}
						return &nethttp.Response{StatusCode: 200, Header: nethttp.Header{"Content-Type": {media}}, Body: owned}, nil
					case "typed-bytes":
						return Response{StatusCode: 200, Headers: nethttp.Header{"Content-Type": {"text/plain"}}, Body: payload}, nil
					case "typed-reader":
						owned = &observedBody{Reader: bytes.NewReader(payload)}
						return Response{StatusCode: 200, Body: owned}, nil
					case "direct-reader":
						owned = &observedBody{Reader: bytes.NewReader(payload)}
						return owned, nil
					case "context-false":
						wantBytes, wantEncoded = []byte("file"), false
						request.Response.Header.Set("Content-Type", "image/png")
						enabled := false
						request.IsBase64Encoded = &enabled
						return wantBytes, nil
					case "context-true":
						wantBytes = []byte("file")
						enabled := true
						request.IsBase64Encoded = &enabled
						return Response{StatusCode: 200, Body: "file"}, nil
					default:
						return payload, nil
					}
				}); err != nil {
					t.Fatal(err)
				}
				result, err := app.Resolve(context.Background(), httpAdapterEvent(t, kind, "GET", "/items"))
				wantBody := string(wantBytes)
				if wantEncoded {
					wantBody = base64.StdEncoding.EncodeToString(wantBytes)
				}
				if err != nil || result.StatusCode != 200 || result.IsBase64Encoded != wantEncoded || result.Body != wantBody {
					t.Fatalf("binary response: %+v / %v; want body=%q encoded=%v", result, err, wantBody, wantEncoded)
				}
				if owned != nil && owned.closed != 1 {
					t.Fatalf("owned body closed %d times", owned.closed)
				}
			})
		}
	}
}

func TestPreencodedProxyConversion(t *testing.T) {
	value := map[string]any{"statusCode": 200, "body": "AAH/", "isBase64Encoded": true}
	response, err := HandlerResultToWebResponse(value)
	if err != nil {
		t.Fatal(err)
	}
	result, err := WebResponseToProxyResult(response, APIGatewayV2, true)
	if err != nil || result.Body != "AAH/" || !result.IsBase64Encoded {
		t.Fatalf("proxy converter encoded twice: %+v / %v", result, err)
	}
	value["body"] = "encoded?"
	if response, err := HandlerResultToWebResponse(value); err == nil {
		_ = response.Body.Close()
		t.Fatal("malformed preencoded proxy accepted")
	}
}

func TestPreencodedProxyCompression(t *testing.T) {
	threshold := float64(0)
	app := New(Options{})
	app.Use(Compress(CompressionOptions{Threshold: &threshold}))
	if err := app.Get("/items", func(*RequestContext) (any, error) {
		return map[string]any{"statusCode": 200, "body": "AAH/", "isBase64Encoded": true}, nil
	}); err != nil {
		t.Fatal(err)
	}
	event := httpAdapterEvent(t, APIGatewayV2, "GET", "/items")
	event["headers"] = map[string]any{"accept-encoding": "gzip"}
	result, err := app.Resolve(context.Background(), event)
	if err != nil || result.Headers["content-encoding"] != "gzip" || !bytes.Equal(decodeCompressed(t, "gzip", result.Body), []byte{0, 1, 255}) {
		t.Fatalf("compressed encoded proxy: %+v / %v", result, err)
	}
}
