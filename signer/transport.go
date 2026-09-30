package signer

import (
	"fmt"
	"net/http"
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
	// A RoundTripper owns its input body, including error paths. Sign may restore it.
	defer func() {
		if request.Body != nil {
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
	return base.RoundTrip(signed)
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
