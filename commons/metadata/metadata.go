package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

const Path = "/2026-01-15/metadata/execution-environment"

type Config struct {
	// Endpoint explicitly enables local HTTP access outside Lambda.
	Endpoint   string
	Token      string
	HTTPClient *http.Client
}
type Options struct{ Timeout time.Duration }
type Error struct {
	StatusCode int
	Err        error
}

func (e *Error) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("failed to fetch execution environment metadata: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("failed to fetch execution environment metadata: %v", e.Err)
}
func (e *Error) Unwrap() error { return e.Err }

type Client struct {
	config     Config
	mu         sync.Mutex
	cache      map[string]any
	inflight   chan struct{}
	generation uint64
}

func New(config Config) *Client { return &Client{config: config} }

var defaultClient = New(Config{})

func GetMetadata(ctx context.Context, options ...Options) (map[string]any, error) {
	return defaultClient.Get(ctx, options...)
}
func ClearMetadataCache()     { defaultClient.ClearCache() }
func (c *Client) ClearCache() { c.mu.Lock(); defer c.mu.Unlock(); c.cache = nil; c.generation++ }

func (c *Client) Get(ctx context.Context, options ...Options) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, &Error{Err: err}
	}
	if c.config.Endpoint == "" && !commons.IsRunningInLambda() {
		return map[string]any{}, nil
	}
	timeout := time.Second
	if len(options) > 0 && options[0].Timeout != 0 {
		timeout = options[0].Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return nil, &Error{Err: err}
		}
		c.mu.Lock()
		if len(c.cache) > 0 {
			value := commons.CloneValue(c.cache).(map[string]any)
			c.mu.Unlock()
			return value, nil
		}
		if pending := c.inflight; pending != nil {
			c.mu.Unlock()
			select {
			case <-pending:
				continue
			case <-ctx.Done():
				return nil, &Error{Err: ctx.Err()}
			}
		}
		pending := make(chan struct{})
		c.inflight = pending
		generation := c.generation
		c.mu.Unlock()
		value, err := c.fetch(ctx)
		c.mu.Lock()
		if err == nil && generation == c.generation && len(value) > 0 {
			c.cache = commons.CloneValue(value).(map[string]any)
		}
		c.inflight = nil
		close(pending)
		c.mu.Unlock()
		return value, err
	}
}

func (c *Client) fetch(ctx context.Context) (map[string]any, error) {
	endpoint, token := c.config.Endpoint, c.config.Token
	var err error
	if endpoint == "" {
		endpoint, err = commons.StringEnv("AWS_LAMBDA_METADATA_API")
		if err != nil {
			return nil, &Error{Err: err}
		}
		endpoint = "http://" + endpoint
	}
	if token == "" {
		token, err = commons.StringEnv("AWS_LAMBDA_METADATA_TOKEN")
		if err != nil {
			return nil, &Error{Err: err}
		}
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, &Error{Err: fmt.Errorf("invalid metadata endpoint")}
	}
	u.Path = strings.TrimRight(u.Path, "/") + Path
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, &Error{Err: err}
	}
	request.Header.Set("Authorization", "Bearer "+token)
	client := http.DefaultClient
	if c.config.HTTPClient != nil {
		client = c.config.HTTPClient
	}
	copy := *client
	// Do not forward the metadata bearer token through redirects.
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := copy.Do(request)
	if err != nil {
		return nil, &Error{Err: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &Error{StatusCode: response.StatusCode}
	}
	var result map[string]any
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, &Error{Err: err}
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &Error{Err: err}
	}
	if result == nil {
		return nil, &Error{Err: fmt.Errorf("metadata response must be an object")}
	}
	return result, nil
}
