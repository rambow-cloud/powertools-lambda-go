package http

import (
	"context"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"strings"
	"testing"
)

func httpAdapterEvent(t *testing.T, kind ResponseType, method, path string) map[string]any {
	t.Helper()
	if kind == APIGatewayV2 {
		var event map[string]any
		if err := json.Unmarshal(testEvent(path), &event); err != nil {
			t.Fatal(err)
		}
		event["requestContext"].(map[string]any)["http"].(map[string]any)["method"] = method
		return event
	}
	event := map[string]any{"httpMethod": method, "path": path, "headers": map[string]any{}, "body": nil, "isBase64Encoded": false}
	if kind == ALB {
		event["requestContext"] = map[string]any{"elb": map[string]any{}}
	} else {
		event["resource"] = "/{proxy+}"
		event["requestContext"] = map[string]any{"domainName": "api.example.test"}
		for _, name := range []string{"pathParameters", "queryStringParameters", "multiValueQueryStringParameters", "stageVariables"} {
			event[name] = nil
		}
	}
	return event
}

func TestNoContentBufferedBodies(t *testing.T) {
	for _, kind := range []ResponseType{APIGatewayV1, APIGatewayV2, ALB} {
		for _, status := range []int{204, 205, 304} {
			for _, form := range []string{"proxy-omitted", "proxy-empty", "proxy-nonempty", "typed-nil", "typed-empty", "typed-bytes", "typed-nonempty", "native-empty", "native-nonempty"} {
				t.Run(fmt.Sprintf("%s/%d/%s", kind, status, form), func(t *testing.T) {
					app := New(Options{})
					var owned *observedBody
					calls, checks := 0, 0
					app.Use(Validate(ValidationConfig{Response: &ResponseChecks{Body: func(context.Context, any) (any, []ValidationIssue, error) {
						checks++
						return nil, []ValidationIssue{{Message: "no-content response must skip body schema"}}, nil
					}}}))
					if err := app.Get("/items", func(*RequestContext) (any, error) {
						calls++
						payload := ""
						if strings.HasSuffix(form, "nonempty") {
							payload = "forbidden"
						}
						if strings.HasPrefix(form, "proxy-") {
							result := map[string]any{"statusCode": status}
							if form != "proxy-omitted" {
								result["body"] = payload
							}
							return result, nil
						}
						if strings.HasPrefix(form, "native-") {
							owned = &observedBody{Reader: strings.NewReader(payload)}
							return &nethttp.Response{StatusCode: status, Header: make(nethttp.Header), Body: owned}, nil
						}
						var body any = payload
						if form == "typed-nil" {
							body = nil
						} else if form == "typed-bytes" {
							body = []byte{}
						}
						return Response{StatusCode: status, Body: body}, nil
					}); err != nil {
						t.Fatal(err)
					}
					wantStatus := status
					if strings.HasSuffix(form, "nonempty") {
						wantStatus = 500
					}
					result, err := app.Resolve(context.Background(), httpAdapterEvent(t, kind, "GET", "/items"))
					if err != nil || result.StatusCode != wantStatus || calls != 1 || checks != 0 {
						t.Fatalf("no-content flow: %+v / %v, calls=%d checks=%d; want status=%d", result, err, calls, checks, wantStatus)
					}
					if wantStatus != 500 && result.Body != "" {
						t.Fatalf("no-content wire body: %q", result.Body)
					}
					if owned != nil && owned.closed != 1 {
						t.Fatalf("native body closed %d times", owned.closed)
					}
				})
			}
		}
	}
}
