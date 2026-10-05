package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"reflect"
	"strings"
	"testing"
)

func TestGetHeadRequestBodyNormalization(t *testing.T) {
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		for _, method := range []string{"GET", "HEAD", "get", "head"} {
			for _, shape := range []string{"omitted", "empty", "plain", "encoded", "malformed-encoded"} {
				t.Run(fmt.Sprintf("%s/%s/%s", kind, method, shape), func(t *testing.T) {
					event := httpAdapterEvent(t, kind, method, "/items")
					event["headers"] = map[string]any{"x-request": "kept"}
					switch shape {
					case "omitted":
						delete(event, "body")
					case "empty":
						event["body"] = ""
					case "plain":
						event["body"] = "hello λ 世界"
					case "encoded":
						event["body"] = base64.StdEncoding.EncodeToString([]byte("hello λ 世界"))
						event["isBase64Encoded"] = true
					default:
						event["body"] = "not base64!!!"
						event["isBase64Encoded"] = true
					}
					original, err := json.Marshal(event)
					if err != nil {
						t.Fatal(err)
					}
					check := func(request *nethttp.Request) {
						t.Helper()
						body, err := io.ReadAll(request.Body)
						if err != nil || len(body) != 0 || request.Body != nethttp.NoBody || request.GetBody != nil || request.ContentLength != 0 || request.Method != strings.ToUpper(method) || request.Header.Get("X-Request") != "kept" || request.Header.Get("Content-Type") != "" {
							t.Fatalf("request body was not normalized: method=%s body=%q length=%d headers=%#v / %v", request.Method, body, request.ContentLength, request.Header, err)
						}
					}
					request, _, err := ProxyEventToWebRequest(context.Background(), event)
					if err != nil {
						t.Fatal(err)
					}
					check(request)
					_ = request.Body.Close()
					app := New(Options{})
					calls := 0
					if err := app.Handle(strings.ToUpper(method), "/items", func(request *RequestContext) (any, error) {
						calls++
						check(request.Request)
						if !reflect.DeepEqual(json.RawMessage(original), request.Event) {
							t.Fatalf("original event snapshot changed: %s; want %s", request.Event, original)
						}
						return Response{StatusCode: 200, Body: "ok"}, nil
					}); err != nil {
						t.Fatal(err)
					}
					result, err := app.Resolve(context.Background(), event)
					if err != nil || result.StatusCode != 200 || calls != 1 {
						t.Fatalf("normalized route: %+v / %v, calls=%d", result, err, calls)
					}
					after, _ := json.Marshal(event)
					if string(after) != string(original) {
						t.Fatal("caller event mutated")
					}
				})
			}
		}
	}
}

func TestPostPatchRequestBodiesRetained(t *testing.T) {
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		for _, method := range []string{"POST", "PATCH"} {
			for _, encoded := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%t", kind, method, encoded), func(t *testing.T) {
					payload := "hello λ 世界"
					event := httpAdapterEvent(t, kind, method, "/items")
					event["body"], event["isBase64Encoded"] = payload, encoded
					if encoded {
						event["body"] = base64.StdEncoding.EncodeToString([]byte(payload))
					}
					app := New(Options{})
					calls := 0
					if err := app.Handle(method, "/items", func(request *RequestContext) (any, error) {
						calls++
						body, err := io.ReadAll(request.Request.Body)
						if err != nil || string(body) != payload || request.Request.ContentLength != int64(len(payload)) || request.Request.GetBody == nil {
							t.Fatalf("retained request body: %q / %v", body, err)
						}
						copy, err := request.Request.GetBody()
						if err != nil {
							t.Fatal(err)
						}
						defer copy.Close()
						replay, err := io.ReadAll(copy)
						if err != nil || string(replay) != payload {
							t.Fatalf("request replay body: %q / %v", replay, err)
						}
						return Response{StatusCode: 200, Body: "ok"}, nil
					}); err != nil {
						t.Fatal(err)
					}
					result, err := app.Resolve(context.Background(), event)
					if err != nil || result.StatusCode != 200 || calls != 1 {
						t.Fatalf("retained body route: %+v / %v, calls=%d", result, err, calls)
					}
				})
			}
		}
	}
}
