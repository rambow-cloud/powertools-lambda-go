package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"testing"
)

func TestValidationJSONMediaTypes(t *testing.T) {
	for _, side := range []string{"request", "response"} {
		for _, media := range []struct {
			name, value string
			json        bool
		}{
			{"ordinary", "application/json", true},
			{"problem", "application/problem+json", true},
			{"vendor", "application/vnd.api+json", true},
			{"case-parameters", `Application/Problem+JSON; charset="utf-8"`, true},
			{"json-parameters", "APPLICATION/JSON; charset=utf-8", true},
			{"other-top-level", "text/example+json", true},
			{"text", "text/plain", false},
			{"absent", "", false},
			{"token-boundary", "application/jsonp", false},
			{"suffix-boundary", "application/problem+json-extra", false},
			{"empty-suffix-base", "application/+json", false},
			{"malformed-parameter", "application/json; charset", false},
			{"multiple-types", "application/json, text/plain", false},
		} {
			for _, malformed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/malformed=%t", side, media.name, malformed), func(t *testing.T) {
					payload := ` {"id":1} `
					if malformed {
						payload = ` {"id": `
					}
					wantStatus, wantHandler, wantCheck := 200, 1, 1
					if media.json && malformed {
						wantCheck = 0
						if side == "request" {
							wantStatus, wantHandler = 422, 0
						} else {
							wantStatus = 500
						}
					}
					calls, checks := 0, 0
					check := func(_ context.Context, value any) (any, []ValidationIssue, error) {
						checks++
						if media.json {
							object, ok := value.(map[string]any)
							if !ok || object["id"] != float64(1) {
								t.Errorf("JSON validator input: %#v", value)
							}
						} else if value != payload {
							t.Errorf("text validator input: %#v; want %q", value, payload)
						}
						return "transformed", nil, nil
					}
					config := ValidationConfig{}
					if side == "request" {
						config.Request = &RequestChecks{Body: check}
					} else {
						config.Response = &ResponseChecks{Body: check}
					}
					app := New(Options{})
					app.Use(func(request *RequestContext, next Next) error {
						err := next()
						if err == nil && side == "response" && request.Valid.Response["body"] != "transformed" {
							t.Error("response transformed value missing")
						}
						return err
					}, Validate(config))
					if err := app.Post("/items", func(request *RequestContext) (any, error) {
						calls++
						if side == "request" {
							raw, err := io.ReadAll(request.Request.Body)
							if err != nil || string(raw) != payload || request.Valid.Request["body"] != "transformed" {
								t.Errorf("request rewritten or transformed value lost: %q / %v / %#v", raw, err, request.Valid.Request)
							}
						}
						return &nethttp.Response{StatusCode: 200, Header: nethttp.Header{"Content-Type": {media.value}}, Body: ioBody([]byte(payload))}, nil
					}); err != nil {
						t.Fatal(err)
					}
					var event map[string]any
					if err := json.Unmarshal(testEvent("/items"), &event); err != nil {
						t.Fatal(err)
					}
					event["requestContext"].(map[string]any)["http"].(map[string]any)["method"] = "POST"
					event["headers"] = map[string]any{"content-type": media.value}
					event["body"] = payload
					result, err := app.Resolve(context.Background(), event)
					if err != nil || result.StatusCode != wantStatus || calls != wantHandler || checks != wantCheck {
						t.Fatalf("validation flow: %+v / %v, handler=%d check=%d; want status=%d handler=%d check=%d", result, err, calls, checks, wantStatus, wantHandler, wantCheck)
					}
					if wantStatus == 200 {
						if result.Body != payload {
							t.Fatalf("response wire payload rewritten: %q", result.Body)
						}
					} else {
						name := "ResponseValidationError"
						if side == "request" {
							name = "RequestValidationError"
						}
						if !strings.Contains(result.Body, name) {
							t.Fatalf("validation error type: %s", result.Body)
						}
					}
				})
			}
		}
	}
}
