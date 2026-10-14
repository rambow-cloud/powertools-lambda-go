package signer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestResigningRemovesEveryAuthenticationHeaderSpelling(t *testing.T) {
	request, _ := http.NewRequest("POST", "https://example.test/items", strings.NewReader("payload"))
	request.Header = http.Header{
		"authorization": {"old lower"}, "AUTHORIZATION": {"old upper"},
		"x-amz-date": {"old date"}, "X-AMZ-DATE": {"other date"},
		"x-amz-security-token": {"old token"}, "X-AMZ-SECURITY-TOKEN": {"other token"},
	}
	before := request.Header.Clone()
	signed, err := configured(t, Config{}).Sign(request)
	if err != nil {
		t.Fatal(err)
	}
	defer signed.Body.Close()
	defer request.Body.Close()
	for _, name := range []string{"Authorization", "X-Amz-Date", "X-Amz-Security-Token"} {
		count := 0
		for key, values := range signed.Header {
			if strings.EqualFold(key, name) {
				count++
				if len(values) != 1 || strings.Contains(values[0], "old ") || strings.Contains(values[0], "other ") {
					t.Errorf("stale %s survived: %v", key, values)
				}
			}
		}
		if count != 1 {
			t.Errorf("%s has %d differently cased entries", name, count)
		}
	}
	if !reflect.DeepEqual(request.Header, before) {
		t.Fatal("re-signing mutated caller headers")
	}
}

func TestTransportPreservesCallerRequestFields(t *testing.T) {
	body := &uploadBody{reader: strings.NewReader("input")}
	request, _ := http.NewRequest("POST", "https://example.test/original", body)
	request.Header.Set("X-Owner", "original")
	beforeURL, beforeHeaders := request.URL.String(), request.Header.Clone()
	transport := &Transport{
		Signer: signingFunc(func(owned *http.Request) (*http.Request, error) {
			owned.URL.Path = "/signed"
			owned.Header.Set("X-Owner", "signed")
			return owned, nil
		}),
		Base: roundTripFunc(func(signed *http.Request) (*http.Response, error) {
			if signed.URL.Path != "/signed" || signed.Header.Get("X-Owner") != "signed" {
				t.Error("base transport did not receive signed fields")
			}
			data, err := io.ReadAll(signed.Body)
			if err != nil || string(data) != "input" {
				t.Errorf("upload=%s, %v", data, err)
			}
			_ = signed.Body.Close()
			return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
		}),
	}
	if _, err := transport.RoundTrip(request); err != nil {
		t.Fatal(err)
	}
	if request.Body != body || request.URL.String() != beforeURL || !reflect.DeepEqual(request.Header, beforeHeaders) {
		t.Fatalf("transport changed caller request: body=%T, url=%s, headers=%v", request.Body, request.URL, request.Header)
	}
	if body.closes.Load() != 1 {
		t.Fatalf("input body closed %d times", body.closes.Load())
	}
}

func TestInvalidURLsFailBeforeCredentialRetrieval(t *testing.T) {
	calls := 0
	s := configured(t, Config{Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		calls++
		return fixtureCredentials, nil
	})})
	for _, raw := range []string{"/relative", "ftp://example.test/a", "https://user:pass@example.test/a", "https://example.test/?bad=%zz", "https://example.test/?a=1;b=2"} {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		value, err := s.Sign(&http.Request{URL: parsed})
		var failure *SigningError
		if value != nil || !errors.As(err, &failure) {
			t.Errorf("invalid URL %q accepted: %v, %v", raw, value, err)
		}
	}
	if calls != 0 {
		t.Fatalf("invalid URLs invoked credential provider %d times", calls)
	}
}

func TestNativeHeaderNormalizationMatchesCanonicalRequest(t *testing.T) {
	canonical, _ := http.NewRequest("GET", "https://example.test/items", nil)
	canonical.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
	canonical.Header.Add("X-Custom", "first")
	canonical.Header.Add("X-Custom", "second")
	native, _ := http.NewRequest("GET", "https://example.test/items", nil)
	native.Header = http.Header{"x-amz-content-sha256": {"UNSIGNED-PAYLOAD"}, "X-Custom": {"first"}, "x-custom": {"second"}}
	s := configured(t, Config{})
	want, err := s.Sign(canonical)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Sign(native)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Header, want.Header) || got.Header.Get("X-Amz-Content-Sha256") != "UNSIGNED-PAYLOAD" {
		t.Fatalf("native header casing changed signature/hash: got=%v, want=%v", got.Header, want.Header)
	}
}
