package signer

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type uploadBody struct {
	reader io.Reader
	closes atomic.Int32
}

func (b *uploadBody) Read(p []byte) (int, error) {
	if b.closes.Load() != 0 {
		return 0, errors.New("upload body closed before reading finished")
	}
	return b.reader.Read(p)
}

func (b *uploadBody) Close() error { b.closes.Add(1); return nil }

func TestTransportBodyHandoff(t *testing.T) {
	for _, clone := range []bool{false, true} {
		for _, baseError := range []bool{false, true} {
			t.Run(strings.Join([]string{map[bool]string{false: "identity", true: "clone"}[clone], map[bool]string{false: "success", true: "error"}[baseError]}, "/"), func(t *testing.T) {
				body := &uploadBody{reader: strings.NewReader("payload")}
				request, _ := http.NewRequest("POST", "https://example.com", body)
				start, done := make(chan struct{}), make(chan struct{})
				var uploaded []byte
				var uploadErr error
				failure := errors.New("base transport error")
				transport := &Transport{
					Signer: signingFunc(func(request *http.Request) (*http.Request, error) {
						if clone {
							return request.Clone(request.Context()), nil
						}
						return request, nil
					}),
					Base: roundTripFunc(func(request *http.Request) (*http.Response, error) {
						go func() {
							defer close(done)
							<-start
							uploaded, uploadErr = io.ReadAll(request.Body)
							_ = request.Body.Close()
						}()
						if baseError {
							return nil, failure
						}
						return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
					}),
				}
				_, err := transport.RoundTrip(request)
				close(start)
				<-done
				if (baseError && err != failure) || (!baseError && err != nil) {
					t.Fatalf("base error changed: %v", err)
				}
				if uploadErr != nil || string(uploaded) != "payload" {
					t.Fatalf("deferred upload: %q, %v", uploaded, uploadErr)
				}
				if body.closes.Load() != 1 {
					t.Fatalf("body closed %d times", body.closes.Load())
				}
			})
		}
	}
}

func TestTransportDistinctBodyOwnership(t *testing.T) {
	for _, empty := range []bool{false, true} {
		input := &uploadBody{reader: strings.NewReader("input")}
		output := &uploadBody{reader: strings.NewReader("signed")}
		request, _ := http.NewRequest("POST", "https://example.com", input)
		transport := &Transport{
			Signer: signingFunc(func(request *http.Request) (*http.Request, error) {
				signed := request.Clone(request.Context())
				if empty {
					signed.Body = http.NoBody
				} else {
					signed.Body = output
				}
				return signed, nil
			}),
			Base: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if empty && request.Body != http.NoBody {
					t.Fatal("empty body semantics changed")
				}
				_, _ = io.ReadAll(request.Body)
				_ = request.Body.Close()
				return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
			}),
		}
		if _, err := transport.RoundTrip(request); err != nil {
			t.Fatal(err)
		}
		if input.closes.Load() != 1 || (!empty && output.closes.Load() != 1) {
			t.Fatalf("input/output closes: %d/%d", input.closes.Load(), output.closes.Load())
		}
	}
}

func TestTransportNilSignedRequestClosesInput(t *testing.T) {
	body := &uploadBody{reader: strings.NewReader("input")}
	request, _ := http.NewRequest("POST", "https://example.com", body)
	transport := &Transport{Signer: signingFunc(func(*http.Request) (*http.Request, error) { return nil, nil })}
	if _, err := transport.RoundTrip(request); err == nil {
		t.Fatal("nil signed request accepted")
	}
	if body.closes.Load() != 1 {
		t.Fatalf("input closed %d times", body.closes.Load())
	}
}
