package signer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// ConfigError indicates missing signing configuration or environment credentials.
type ConfigError struct{ Message string }

func (e *ConfigError) Error() string { return e.Message }

// SigningError preserves the underlying request conversion or signing failure.
type SigningError struct{ Err error }

func (e *SigningError) Error() string { return "unable to sign request: " + e.Err.Error() }
func (e *SigningError) Unwrap() error { return e.Err }

// Config controls signing without loading the SDK configuration/provider chain.
// Credentials are retrieved for every request; use aws.CredentialsCache explicitly
// when provider caching is desired. Callbacks must be safe for concurrent use.
type Config struct {
	Service     string
	Region      string
	Credentials aws.CredentialsProvider
	Clock       func() time.Time
	// DisableURIPathEscaping is needed for S3's single-escaped path convention.
	DisableURIPathEscaping bool
}

// SigV4Signer is immutable after construction and safe for concurrent requests.
type SigV4Signer struct {
	config Config
	signer *v4.Signer
}

func New(config Config) (*SigV4Signer, error) {
	if config.Service == "" {
		return nil, &ConfigError{"signing service is required"}
	}
	if config.Region == "" {
		config.Region = os.Getenv("AWS_REGION")
	}
	if config.Region == "" {
		return nil, &ConfigError{"signing region or AWS_REGION is required"}
	}
	if config.Credentials == nil {
		config.Credentials = aws.CredentialsProviderFunc(environmentCredentials)
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	return &SigV4Signer{config: config, signer: v4.NewSigner(func(o *v4.SignerOptions) {
		o.DisableURIPathEscaping = config.DisableURIPathEscaping
	})}, nil
}

func environmentCredentials(ctx context.Context) (aws.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return aws.Credentials{}, err
	}
	key, hasKey := os.LookupEnv("AWS_ACCESS_KEY_ID")
	secret, hasSecret := os.LookupEnv("AWS_SECRET_ACCESS_KEY")
	if !hasKey || !hasSecret {
		return aws.Credentials{}, &ConfigError{"AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required"}
	}
	return aws.Credentials{AccessKeyID: key, SecretAccessKey: secret, SessionToken: os.Getenv("AWS_SESSION_TOKEN"), Source: "Environment"}, nil
}

// Sign clones headers and URL and returns a request with a replayable body.
// With GetBody, the input stream is untouched. Otherwise the input body is read
// and restored; it must not be used concurrently. Callers retain ownership of
// the input body and must close both requests' bodies when signing standalone.
// Provider failures and cancellation remain discoverable through errors.Is/As.
func (s *SigV4Signer) Sign(request *http.Request) (*http.Request, error) {
	if request == nil || request.URL == nil {
		return nil, &SigningError{fmt.Errorf("request URL is required")}
	}
	ctx := request.Context()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.URL.Host == "" || (request.URL.Scheme != "https" && request.URL.Scheme != "http") || request.URL.User != nil {
		return nil, &SigningError{fmt.Errorf("an absolute HTTP(S) URL without user information is required")}
	}
	if _, err := url.ParseQuery(request.URL.RawQuery); err != nil {
		return nil, &SigningError{err}
	}
	credentials, err := s.config.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, err
	}
	body, err := readBody(request)
	if err != nil {
		return nil, &SigningError{err}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	signed := request.Clone(ctx)
	if signed.Header == nil {
		signed.Header = make(http.Header)
	}
	// Normalize repeated headers like the reference web Headers conversion.
	for key, values := range signed.Header {
		signed.Header[key] = []string{strings.Join(values, ", ")}
	}
	for _, key := range []string{"Authorization", "X-Amz-Date", "X-Amz-Security-Token"} {
		signed.Header.Del(key)
	}
	if signed.Method == "" {
		signed.Method = http.MethodGet
	}
	signed.Body = http.NoBody
	signed.GetBody = func() (io.ReadCloser, error) { return http.NoBody, nil }
	if len(body) > 0 {
		signed.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
		signed.Body, _ = signed.GetBody()
	}
	// Smithy signs Content-Length only when explicitly present in web Headers.
	signed.ContentLength = 0
	if value := signed.Header.Get("Content-Length"); value != "" {
		length, err := strconv.ParseInt(value, 10, 64)
		if err != nil || length != int64(len(body)) {
			return nil, &SigningError{fmt.Errorf("Content-Length does not match the buffered payload")}
		}
		signed.ContentLength = length
	}
	signed.TransferEncoding = nil
	hash := signed.Header.Get("X-Amz-Content-Sha256")
	if hash == "" {
		sum := sha256.Sum256(body)
		hash = hex.EncodeToString(sum[:])
		signed.Header.Set("X-Amz-Content-Sha256", hash)
	}
	originalQuery := signed.URL.RawQuery
	if err = s.signer.SignHTTP(ctx, credentials, signed, hash, s.config.Service, s.config.Region, s.config.Clock().UTC()); err != nil {
		_ = signed.Body.Close()
		return nil, &SigningError{err}
	}
	signed.URL.RawQuery = originalQuery
	signed.ContentLength = int64(len(body))
	return signed, nil
}

type replayBody struct {
	io.Reader
	io.Closer
}
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func readBody(request *http.Request) ([]byte, error) {
	if request.Body == nil || request.Body == http.NoBody {
		return nil, nil
	}
	if request.GetBody != nil {
		body, err := request.GetBody()
		if err != nil {
			return nil, err
		}
		if body == nil {
			return nil, fmt.Errorf("GetBody returned nil")
		}
		defer body.Close()
		return io.ReadAll(contextReader{request.Context(), body})
	}
	original := request.Body
	body, err := io.ReadAll(contextReader{request.Context(), original})
	// Keep any unread suffix after an error and preserve the original Close.
	request.Body = replayBody{io.MultiReader(bytes.NewReader(body), original), original}
	return body, err
}
