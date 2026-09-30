// Package appconfigagent reads the local AppConfig Agent or Lambda extension.
// The agent owns caching and polling; this package does not add a second cache.
package appconfigagent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

type Options struct {
	Application, Environment string
	Transform                parameters.Transform
	ThrowOnMissing           bool
	Timeout                  time.Duration
	HTTPClient               *http.Client
	// Endpoint explicitly enables HTTP outside Lambda, for a local agent or tests.
	Endpoint string
}

func GetConfig(ctx context.Context, name string, options Options) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, parameters.GetError(name, err)
	}
	var value any
	if options.Endpoint == "" && !commons.IsRunningInLambda() {
		if local, _ := commons.StringEnv("POWERTOOLS_APPCONFIG_AGENT_RETURN_VALUE", ""); local != "" {
			value = local
		}
	} else {
		application := options.Application
		if application == "" {
			application = commons.ServiceName()
		}
		if strings.TrimSpace(application) == "" {
			return nil, parameters.GetError(name, fmt.Errorf("application name or POWERTOOLS_SERVICE_NAME is required"))
		}
		if options.Environment == "" {
			return nil, parameters.GetError(name, fmt.Errorf("environment is required"))
		}
		endpoint := options.Endpoint
		if endpoint == "" {
			port, _ := commons.StringEnv("AWS_APPCONFIG_EXTENSION_HTTP_PORT", "2772")
			endpoint = "http://localhost:" + port
		}
		target := strings.TrimRight(endpoint, "/") + "/applications/" + url.PathEscape(application) + "/environments/" + url.PathEscape(options.Environment) + "/configurations/" + url.PathEscape(name)
		timeout := options.Timeout
		if timeout == 0 {
			timeout = 3 * time.Second
		}
		requestCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target, nil)
		if err != nil {
			return nil, parameters.GetError(name, err)
		}
		client := options.HTTPClient
		if client == nil {
			client = http.DefaultClient
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, parameters.GetError(name, err)
		}
		defer response.Body.Close()
		if response.StatusCode == http.StatusNotFound {
			value = nil
		} else if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, parameters.GetError(name, fmt.Errorf("AppConfig Agent returned HTTP %d", response.StatusCode))
		} else {
			body, err := io.ReadAll(response.Body)
			if err != nil {
				return nil, parameters.GetError(name, err)
			}
			value = string(body)
		}
	}
	if value == nil && options.ThrowOnMissing {
		return nil, &parameters.ParameterNotFoundError{Name: name}
	}
	return parameters.TransformValue(name, value, options.Transform)
}
