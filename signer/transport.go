package signer

import (
	"fmt"
	"io"
	"net/http"
	"sync"
)

// Signer separates request signing from sending and permits custom algorithms.
type Signer interface {
	Sign(*http.Request) (*http.Request, error)
}

// Transport signs each round trip, including requests followed by http.Client.
// It performs no retries. Signer and Base must support concurrent calls.
type Transport struct {
	Signer Signer
	Base   http.RoundTripper
}

func (t *Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil {
		return nil, &SigningError{fmt.Errorf("request is required")}
	}
	// Signers may retain or wrap the input body. Share idempotent closure across
	// those aliases without comparing application ReadClosers, which may not be comparable.
	if request.Body != nil && request.Body != http.NoBody {
		request.Body = &transportBody{ReadCloser: request.Body}
	}
	handedOff := false
	defer func() {
		if !handedOff && request.Body != nil {
			_ = request.Body.Close()
		}
	}()
	if t.Signer == nil {
		return nil, &ConfigError{"transport signer is required"}
	}
	signed, err := t.Signer.Sign(request)
	if err != nil {
		return nil, err
	}
	if signed == nil {
		return nil, &SigningError{fmt.Errorf("signer returned a nil request")}
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if signed.Body == nil || signed.Body == http.NoBody {
		// Preserve net/http's empty-body framing. There is no upload to wait for.
		if request.Body != nil {
			_ = request.Body.Close()
		}
	} else {
		// A base transport may finish the upload after RoundTrip returns, including
		// error paths. Release both bodies only when that transport closes its body.
		signed.Body = &transportBody{ReadCloser: signed.Body, input: request.Body}
	}
	handedOff = true
	return base.RoundTrip(signed)
}

type transportBody struct {
	io.ReadCloser
	input io.Closer
	once  sync.Once
	err   error
}

func (b *transportBody) Close() error {
	b.once.Do(func() {
		b.err = b.ReadCloser.Close()
		if b.input != nil {
			if err := b.input.Close(); b.err == nil {
				b.err = err
			}
		}
	})
	return b.err
}

// HTTPClient copies a client and installs signing without changing its owner.
// Redirects are disabled by default to avoid signing an untrusted destination.
// Supply CheckRedirect explicitly to permit trusted redirects; each is re-signed.
func HTTPClient(s Signer, client *http.Client) *http.Client {
	copy := http.Client{}
	if client != nil {
		copy = *client
	}
	copy.Transport = &Transport{Signer: s, Base: copy.Transport}
	if copy.CheckRedirect == nil {
		copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	return &copy
}
