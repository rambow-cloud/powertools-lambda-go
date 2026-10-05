package http

import (
	"bytes"
	"context"
	"fmt"
	nethttp "net/http"
	"reflect"
	"strings"
	"testing"
)

func TestBufferedCookieAndHeaderValues(t *testing.T) {
	cookies := []string{"a=1; Expires=Mon, 21 Oct 2030 07:28:00 GMT", "b=2; Path=/; HttpOnly"}
	repeated := []string{"first, comma", "second"}
	date := "Mon, 05 Oct 2026 12:00:00 GMT"
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		for _, count := range []int{1, 2} {
			for _, form := range []string{"native", "typed", "typed-multi", "typed-cookies", "proxy-multi", "proxy-cookies"} {
				t.Run(fmt.Sprintf("%s/%d/%s", kind, count, form), func(t *testing.T) {
					wantCookies := cookies[:count]
					headers := nethttp.Header{"Content-Type": {"text/plain"}, "Set-Cookie": append([]string(nil), wantCookies...), "X-Repeat": append([]string(nil), repeated...), "Date": {date}, "Last-Modified": {date}, "X-Single": {"kept"}}
					original := headers.Clone()
					var owned *observedBody
					app := New(Options{})
					if err := app.Get("/items", func(*RequestContext) (any, error) {
						if form == "native" {
							owned = &observedBody{Reader: strings.NewReader("ok")}
							return &nethttp.Response{StatusCode: 200, Header: headers, Body: owned}, nil
						}
						if form == "typed" {
							return Response{StatusCode: 200, Headers: headers, Body: "ok"}, nil
						}
						single := nethttp.Header{"Content-Type": {"text/plain"}, "Date": {date}, "Last-Modified": {date}, "X-Single": {"kept"}}
						multi := nethttp.Header{"X-Repeat": repeated}
						if form == "typed-multi" || form == "typed-cookies" {
							result := Response{StatusCode: 200, Headers: single, MultiValueHeaders: multi, Body: "ok"}
							if form == "typed-multi" {
								multi["Set-Cookie"] = wantCookies
								result.MultiValueHeaders = multi
							} else {
								result.Cookies = wantCookies
							}
							return result, nil
						}
						result := map[string]any{"statusCode": 200, "body": "ok", "headers": map[string]string{"content-type": "text/plain", "date": date, "last-modified": date, "x-single": "kept"}, "multiValueHeaders": map[string][]string{"x-repeat": repeated}}
						if form == "proxy-multi" {
							result["multiValueHeaders"].(map[string][]string)["set-cookie"] = wantCookies
						} else {
							result["cookies"] = wantCookies
						}
						return result, nil
					}); err != nil {
						t.Fatal(err)
					}
					result, err := app.Resolve(context.Background(), httpAdapterEvent(t, kind, "GET", "/items"))
					if err != nil || result.StatusCode != 200 || result.Body != "ok" || result.Headers["date"] != date || result.Headers["last-modified"] != date || result.Headers["x-single"] != "kept" {
						t.Fatalf("scalar headers or response lost: %+v / %v", result, err)
					}
					if kind == APIGatewayV2 {
						if !reflect.DeepEqual(result.Cookies, wantCookies) || result.Headers["set-cookie"] != "" || result.Headers["x-repeat"] != strings.Join(repeated, ", ") {
							t.Fatalf("v2 header values: %+v; cookies=%#v", result, wantCookies)
						}
					} else {
						if !reflect.DeepEqual(result.MultiValueHeaders["x-repeat"], repeated) || result.Headers["x-repeat"] != "" {
							t.Fatalf("custom repeated values lost: %+v", result)
						}
						if count == 1 {
							if result.Headers["set-cookie"] != wantCookies[0] || len(result.MultiValueHeaders["set-cookie"]) != 0 {
								t.Fatalf("Expires comma split: %+v", result)
							}
						} else if !reflect.DeepEqual(result.MultiValueHeaders["set-cookie"], wantCookies) || result.Headers["set-cookie"] != "" {
							t.Fatalf("cookie values lost: %+v", result)
						}
					}
					if !reflect.DeepEqual(headers, original) || owned != nil && owned.closed != 1 {
						t.Fatalf("caller headers mutated or native body closure wrong: %#v, owned=%+v", headers, owned)
					}
				})
			}
		}
	}
}

func TestResponseMultiValueHeaderValidation(t *testing.T) {
	for _, headers := range []map[string][]string{{"bad name": {"value"}}, {"x-many": {"first", "bad\r\nInjected: yes"}}} {
		value := map[string]any{"statusCode": 200, "body": "ok", "multiValueHeaders": headers}
		if response, err := HandlerResultToWebResponse(value); err == nil {
			_ = response.Body.Close()
			t.Fatalf("invalid response header accepted: %#v", headers)
		}
	}
}

func TestStreamingHeaderProjection(t *testing.T) {
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		t.Run(string(kind), func(t *testing.T) {
			app := New(Options{})
			cookie := "a=1; Expires=Mon, 21 Oct 2030 07:28:00 GMT"
			if err := app.Get("/items", func(*RequestContext) (any, error) {
				return Response{StatusCode: 200, Headers: nethttp.Header{"Content-Type": {"text/plain"}, "Set-Cookie": {cookie}, "X-Many": {"one", "two"}, "Vary": {"Origin", "Accept"}}, Body: "ok"}, nil
			}); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := app.ResolveStream(context.Background(), httpAdapterEvent(t, kind, "GET", "/items"), &output); err != nil {
				t.Fatal(err)
			}
			metadata, body := splitStream(t, output.Bytes())
			headers := metadata["headers"].(map[string]any)
			if headers["set-cookie"] != cookie || headers["x-many"] != "one, two" || headers["vary"] != "Origin, Accept" || string(body) != "ok" {
				t.Fatalf("stream header projection lost values: %#v / %q", metadata, body)
			}
		})
	}
}
