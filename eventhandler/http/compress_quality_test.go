package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"strings"
	"testing"
)

func TestCompressionQualityNegotiation(t *testing.T) {
	for _, encoding := range []string{"gzip", "deflate"} {
		for _, test := range []struct {
			name, header string
			absent, want bool
		}{
			{"absent", "", true, true},
			{"empty", "", false, false},
			{"ordinary", "$coding", false, true},
			{"uppercase", "$UPPER", false, true},
			{"forbidden", "$coding;q=0", false, false},
			{"specific-forbidden", "$coding;q=0, *;q=1", false, false},
			{"wildcard-forbidden", "*;q=0", false, false},
			{"wildcard-positive", "*;q=0.8", false, true},
			{"specific-positive", "*;q=0, $coding;q=0.7", false, true},
			{"identity-tie", "identity, $coding", false, true},
			{"identity-preferred", "$coding;q=0.4, identity;q=0.8", false, false},
			{"coding-preferred", "$coding;q=0.8, identity;q=0.4", false, true},
			{"identity-forbidden", "identity;q=0, $coding", false, true},
			{"identity-only", "identity", false, false},
			{"token-boundary", "x$codingx", false, false},
			{"whitespace", "  $UPPER ; Q=0.5 , identity ; q=0.1  ", false, true},
			{"invalid-weight", "$coding;q=bad", false, false},
			{"negative-weight", "$coding;q=-0.1", false, false},
			{"excess-weight", "$coding;q=2", false, false},
			{"nan-weight", "$coding;q=NaN", false, false},
			{"scientific-weight", "$coding;q=1e0", false, false},
			{"precision", "$coding;q=0.1234", false, false},
			{"duplicate-weight", "$coding;q=0.5;q=1", false, false},
		} {
			t.Run(encoding+"/"+test.name, func(t *testing.T) {
				threshold := float64(0)
				app := New(Options{})
				app.Use(Compress(CompressionOptions{Encoding: encoding, Threshold: &threshold}))
				body := "quality λ 世界"
				if err := app.Get("/items", func(*RequestContext) (any, error) {
					return &nethttp.Response{StatusCode: 200, Header: nethttp.Header{"Content-Type": {"text/plain"}, "X-Correlation": {"kept"}}, Body: ioBody([]byte(body))}, nil
				}); err != nil {
					t.Fatal(err)
				}
				var event map[string]any
				if err := json.Unmarshal(testEvent("/items"), &event); err != nil {
					t.Fatal(err)
				}
				if !test.absent {
					header := strings.NewReplacer("$coding", encoding, "$UPPER", strings.ToUpper(encoding)).Replace(test.header)
					event["headers"] = map[string]any{"accept-encoding": header}
				}
				result, err := app.Resolve(context.Background(), event)
				if err != nil || result.StatusCode != 200 || result.Headers["x-correlation"] != "kept" || result.IsBase64Encoded != test.want {
					t.Fatalf("quality selection or response fields: %+v / %v; compressed=%v", result, err, test.want)
				}
				if test.want {
					if result.Headers["content-encoding"] != encoding || string(decodeCompressed(t, encoding, result.Body)) != body {
						t.Fatalf("compressed payload lost: %+v", result)
					}
				} else if result.Body != body || result.Headers["content-encoding"] != "" {
					t.Fatalf("identity payload lost: %+v", result)
				}
			})
		}
	}
}
