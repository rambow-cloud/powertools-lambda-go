package signer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type signingFunc func(*http.Request) (*http.Request, error)

func (f signingFunc) Sign(request *http.Request) (*http.Request, error) { return f(request) }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestCustomTransportAndBodyCloseOnFailure(t *testing.T) {
	want := errors.New("signing failed")
	for _, custom := range []Signer{nil, signingFunc(func(*http.Request) (*http.Request, error) { return nil, want })} {
		original := &trackedBody{Reader: strings.NewReader("payload")}
		req, _ := http.NewRequest("POST", "https://example.com", original)
		transport := &Transport{Signer: custom, Base: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("failed request sent"); return nil, nil })}
		if _, err := transport.RoundTrip(req); err == nil || !original.closed {
			t.Fatal(err, original.closed)
		}
	}
	base := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") == "" {
			t.Fatal("unsigned request")
		}
		_ = request.Body.Close()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header), Request: request}, nil
	})}
	client := HTTPClient(configured(t, Config{}), base)
	response, err := client.Get("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if _, changed := base.Transport.(*Transport); changed {
		t.Fatal("original client mutated")
	}
}

func TestResigningAndRequestValidation(t *testing.T) {
	s := configured(t, Config{})
	req, _ := http.NewRequest("POST", "https://example.com", strings.NewReader("body"))
	req.Header.Set("Content-Length", "999")
	if _, err := s.Sign(req); err == nil {
		t.Fatal("wrong length accepted")
	}
	req.Header.Del("Content-Length")
	first, err := s.Sign(req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Sign(first)
	if err != nil {
		t.Fatal(err)
	}
	if first.Header.Get("Authorization") != second.Header.Get("Authorization") {
		t.Fatal("re-sign changed equivalent request")
	}
	_ = first.Body.Close()
	_ = second.Body.Close()
	for _, address := range []string{"/relative", "ftp://example.com", "https://user:pass@example.com", "https://example.com/?x=%zz"} {
		req, err := http.NewRequest("GET", address, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Sign(req); err == nil {
			t.Fatal(address)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ = http.NewRequestWithContext(ctx, "GET", "https://example.com", nil)
	if _, err := s.Sign(req); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
