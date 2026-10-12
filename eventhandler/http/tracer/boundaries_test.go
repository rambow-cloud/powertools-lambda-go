package tracer_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httptracer "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer"
	powertracer "github.com/rambow-cloud/powertools-lambda-go/tracer"
)

func TestResponseCaptureStorageAndParsingControls(t *testing.T) {
	for _, globalOff := range []bool{false, true} {
		for _, middlewareOff := range []bool{false, true} {
			for _, invalid := range []bool{false, true} {
				t.Run(fmt.Sprintf("global=%v/middleware=%v/invalid=%v", globalOff, middlewareOff, invalid), func(t *testing.T) {
					tr, recorder := newTracer(t, powertracer.WithCaptureResponse(!globalOff))
					ctx := context.Background()
					request, _ := http.NewRequestWithContext(ctx, "GET", "https://api.test/items", nil)
					original := `{"ok":true}`
					if invalid {
						original = `{`
					}
					body := &ownedBody{Reader: strings.NewReader(original)}
					response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: body}
					r := &httpapi.RequestContext{Context: ctx, Request: request, Response: response, ResponseType: httpapi.ALB}
					err := httptracer.New(tr, httptracer.Options{DisableCaptureResponse: middlewareOff})(r, func() error { return nil })
					if (err != nil) != (invalid && !middlewareOff) {
						t.Fatalf("unexpected parsing policy: %v", err)
					}
					if r.Context != ctx || r.Request != request || len(recorder.Ended()) != 1 {
						t.Fatal("span lifecycle changed")
					}
					captured := attributes(recorder.Ended()[0])["powertools.metadata.orders.GET /items response"] != nil
					if captured != (!globalOff && !middlewareOff && !invalid) {
						t.Fatal("metadata capture control ignored")
					}
					if middlewareOff {
						if body.closes != 0 || response.Body != body {
							t.Fatal("disabled middleware touched body")
						}
					} else if body.closes != 1 {
						t.Fatal("captured body not closed exactly once")
					}
					data, readErr := io.ReadAll(response.Body)
					if readErr != nil || string(data) != original {
						t.Fatal("response replay changed bytes")
					}
					_ = response.Body.Close()
					if body.closes != 1 {
						t.Fatal("response body ownership changed")
					}
				})
			}
		}
	}
}
