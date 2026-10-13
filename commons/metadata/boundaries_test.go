package metadata_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/commons/metadata"
)

type boundaryTransport func(*http.Request) (*http.Response, error)

func (f boundaryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type boundaryBody struct {
	io.Reader
	closes int
}

func (b *boundaryBody) Close() error { b.closes++; return nil }

func TestMetadataStrictBodyFailureAndRecovery(t *testing.T) {
	for _, payload := range []string{"null", "[]", `{"x":1,"x":2}`, "{\"x\":\"\xff\"}", `{} trailing`} {
		t.Run(payload, func(t *testing.T) {
			calls := 0
			bodies := []*boundaryBody{}
			transport := boundaryTransport(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Authorization") != "Bearer synthetic" || r.URL.Path != metadata.Path {
					t.Error("request protocol changed")
				}
				calls++
				text := payload
				if calls > 1 {
					text = `{"nested":{"value":1}}`
				}
				body := &boundaryBody{Reader: strings.NewReader(text)}
				bodies = append(bodies, body)
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body}, nil
			})
			client := metadata.New(metadata.Config{Endpoint: "http://metadata.example.test", Token: "synthetic", HTTPClient: &http.Client{Transport: transport}})
			_, err := client.Get(context.Background())
			var typed *metadata.Error
			if !errors.As(err, &typed) || typed.Err == nil {
				t.Fatalf("native body error lost: %v", err)
			}
			value, err := client.Get(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			value["nested"].(map[string]any)["value"] = 99
			next, err := client.Get(context.Background())
			if err != nil || next["nested"].(map[string]any)["value"] != float64(1) || calls != 2 {
				t.Fatalf("failure cached or snapshot aliased: %v/%v/%d", next, err, calls)
			}
			for _, body := range bodies {
				if body.closes != 1 {
					t.Fatalf("body closes=%d", body.closes)
				}
			}
		})
	}
}

func TestMetadataNativeEndpointAndCancellation(t *testing.T) {
	calls := 0
	clientHTTP := &http.Client{Transport: boundaryTransport(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected request") })}
	for _, endpoint := range []string{"ftp://example.test", "http://user:pass@example.test", "http://example.test?query=1", "http://example.test#fragment", "http:///missing", "%"} {
		client := metadata.New(metadata.Config{Endpoint: endpoint, Token: "synthetic", HTTPClient: clientHTTP})
		if _, err := client.Get(context.Background()); err == nil {
			t.Fatalf("endpoint accepted: %s", endpoint)
		}
	}
	client := metadata.New(metadata.Config{Endpoint: "http://metadata.example.test", Token: "synthetic", HTTPClient: clientHTTP})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Get(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel cause: %v", err)
	}
	if calls != 0 {
		t.Fatal("invalid/cancelled request reached transport")
	}
}
