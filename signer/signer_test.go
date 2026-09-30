package signer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

var fixtureCredentials = aws.Credentials{AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", SessionToken: "fixture-session"}
var fixtureTime = time.Date(2026, 9, 14, 12, 34, 56, 0, time.UTC)

func configured(t *testing.T, config Config) *SigV4Signer {
	t.Helper()
	if config.Service == "" {
		config.Service = "execute-api"
	}
	if config.Region == "" {
		config.Region = "ap-east-1"
	}
	if config.Clock == nil {
		config.Clock = func() time.Time { return fixtureTime }
	}
	if config.Credentials == nil {
		config.Credentials = aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) { return fixtureCredentials, nil })
	}
	s, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTypeScriptSignatures(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name, URL, Method, BodyBase64, Service, SignedBodyBase64 string
			Headers, SignedHeaders                                   map[string]string
			Tokenless, IntentionalDifference                         bool
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			body, _ := base64.StdEncoding.DecodeString(item.BodyBase64)
			req, _ := http.NewRequest(item.Method, item.URL, bytes.NewReader(body))
			for key, value := range item.Headers {
				req.Header.Set(key, value)
			}
			credentials := fixtureCredentials
			if item.Tokenless {
				credentials.SessionToken = ""
			}
			s := configured(t, Config{Service: item.Service, Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) { return credentials, nil })})
			signed, err := s.Sign(req)
			if err != nil {
				t.Fatal(err)
			}
			defer signed.Body.Close()
			for key, want := range item.SignedHeaders {
				if key == "host" {
					if signed.URL.Host != want {
						t.Fatal("host changed")
					}
					continue
				}
				if key == "authorization" && item.IntentionalDifference {
					if signed.Header.Get(key) == want {
						t.Fatal("duplicate query values were discarded")
					}
					continue
				}
				if got := signed.Header.Get(key); got != want {
					t.Errorf("%s: got %s, want %s", key, got, want)
				}
			}
			if signed.URL.String() != item.URL || req.Header.Get("Authorization") != "" {
				t.Fatal("request mutation")
			}
			got, _ := io.ReadAll(signed.Body)
			if base64.StdEncoding.EncodeToString(got) != item.SignedBodyBase64 {
				t.Fatal("payload changed")
			}
			original, _ := io.ReadAll(req.Body)
			if !bytes.Equal(original, body) {
				t.Fatal("original body consumed")
			}
		})
	}
}

func TestEnvironmentRefreshAndFailures(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "first")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_SESSION_TOKEN", "token")
	s, err := New(Config{Service: "lambda"})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	first, _ := s.Sign(req)
	t.Setenv("AWS_ACCESS_KEY_ID", "second")
	t.Setenv("AWS_SESSION_TOKEN", "")
	second, _ := s.Sign(req)
	if !strings.Contains(first.Header.Get("Authorization"), "Credential=first/") || !strings.Contains(second.Header.Get("Authorization"), "Credential=second/") || second.Header.Get("X-Amz-Security-Token") != "" {
		t.Fatal("credentials were cached")
	}
	t.Setenv("AWS_REGION", "")
	var configError *ConfigError
	if _, err = New(Config{Service: "lambda"}); !errors.As(err, &configError) {
		t.Fatal(err)
	}
	if _, err = New(Config{Region: "ap-east-1"}); !errors.As(err, &configError) {
		t.Fatal(err)
	}
	if err = os.Unsetenv("AWS_ACCESS_KEY_ID"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Sign(req); !errors.As(err, &configError) {
		t.Fatal(err)
	}
	want := errors.New("provider failed")
	s = configured(t, Config{Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) { return aws.Credentials{}, want })})
	if _, err = s.Sign(req); err != want {
		t.Fatal("provider error replaced", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.Sign(req.WithContext(ctx)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestStreamReplayAndFailure(t *testing.T) {
	s := configured(t, Config{})
	original := &trackedBody{Reader: strings.NewReader("finite stream")}
	req, _ := http.NewRequest("POST", "https://example.com", original)
	signed, err := s.Sign(req)
	if err != nil {
		t.Fatal(err)
	}
	if original.closed {
		t.Fatal("standalone signing closed the input")
	}
	for _, body := range []io.ReadCloser{req.Body, signed.Body} {
		data, _ := io.ReadAll(body)
		if string(data) != "finite stream" {
			t.Fatal(string(data))
		}
		_ = body.Close()
	}
	if !original.closed {
		t.Fatal("original Close lost")
	}
	want := errors.New("read failed")
	req, _ = http.NewRequest("POST", "https://example.com", io.MultiReader(strings.NewReader("prefix"), failingReader{want}))
	_, err = s.Sign(req)
	var signingError *SigningError
	if !errors.As(err, &signingError) || !errors.Is(err, want) {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(req.Body)
	if string(data) != "prefix" {
		t.Fatal("partial stream not restored")
	}
	for _, bad := range []*http.Request{nil, {URL: nil}} {
		if _, err := s.Sign(bad); !errors.As(err, &signingError) {
			t.Fatal(err)
		}
	}
}

func TestTransportRedirectsAndConcurrentRefresh(t *testing.T) {
	var calls atomic.Int32
	s := configured(t, Config{Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) { calls.Add(1); return fixtureCredentials, nil })})
	var mu sync.Mutex
	authorizations := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authorizations[r.URL.Path] = r.Header.Get("Authorization")
		mu.Unlock()
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/destination", http.StatusTemporaryRedirect)
			return
		}
		_, _ = io.Copy(w, r.Body)
	}))
	defer server.Close()
	client := HTTPClient(s, server.Client())
	response, err := client.Get(server.URL + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 307 || calls.Load() != 1 {
		t.Fatal("redirect followed without opt-in")
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	response, err = client.Post(server.URL+"/redirect", "text/plain", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(body) != "payload" || authorizations["/redirect"] == authorizations["/destination"] {
		t.Fatal("redirect body/signature lost")
	}
	var wg sync.WaitGroup
	for range 30 {
		wg.Go(func() {
			response, err := client.Get(server.URL + "/parallel")
			if err != nil {
				t.Error(err)
				return
			}
			_ = response.Body.Close()
		})
	}
	wg.Wait()
	if calls.Load() != 33 {
		t.Fatal(calls.Load())
	}
}
