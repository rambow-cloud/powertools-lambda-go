package appconfig

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/appconfigdata"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/commons/awssdk"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

type Client interface {
	StartConfigurationSession(context.Context, *sdk.StartConfigurationSessionInput, ...func(*sdk.Options)) (*sdk.StartConfigurationSessionOutput, error)
	GetLatestConfiguration(context.Context, *sdk.GetLatestConfigurationInput, ...func(*sdk.Options)) (*sdk.GetLatestConfigurationOutput, error)
}
type Config struct {
	Application, Environment string
	// Clock defaults to time.Now and controls both cache and token expiry.
	Clock func() time.Time
}
type GetOptions struct {
	parameters.Options
	SDKOptions *sdk.StartConfigurationSessionInput
}
type session struct {
	gate    chan struct{}
	token   string
	expires time.Time
	value   []byte
}
type Provider struct {
	client   Client
	cache    *parameters.Cache
	config   Config
	mu       sync.Mutex
	sessions map[string]*session
}

func New(client Client, config Config) (*Provider, error) {
	if config.Application == "" {
		config.Application = commons.ServiceName()
	}
	if strings.TrimSpace(config.Application) == "" {
		return nil, fmt.Errorf("application name or POWERTOOLS_SERVICE_NAME is required")
	}
	if config.Environment == "" {
		return nil, fmt.Errorf("environment is required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	return &Provider{client: client, cache: parameters.NewCache(config.Clock), config: config, sessions: make(map[string]*session)}, nil
}

// ClearCache clears transformed values. Session tokens and last known configuration
// remain available because an unchanged AppConfig response has an empty body.
func (p *Provider) ClearCache() { p.cache.ClearCache() }

func userAgent(o *sdk.Options) { o.APIOptions = append(o.APIOptions, awssdk.UserAgent("parameters")) }

func (p *Provider) Get(ctx context.Context, name string, options GetOptions) (any, error) {
	return p.cache.Get(ctx, name, options.Options, func(ctx context.Context) (any, error) {
		p.mu.Lock()
		state := p.sessions[name]
		if state == nil {
			state = &session{gate: make(chan struct{}, 1)}
			p.sessions[name] = state
		}
		p.mu.Unlock()
		// AppConfig tokens are single-use. Serialize each profile and allow cancelled waiters to leave.
		select {
		case state.gate <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		defer func() { <-state.gate }()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if state.token == "" || !p.config.Clock().Before(state.expires) {
			input := sdk.StartConfigurationSessionInput{}
			if options.SDKOptions != nil {
				input = *options.SDKOptions
			}
			input.ApplicationIdentifier = aws.String(p.config.Application)
			input.EnvironmentIdentifier = aws.String(p.config.Environment)
			input.ConfigurationProfileIdentifier = aws.String(name)
			out, err := p.client.StartConfigurationSession(ctx, &input, userAgent)
			if err != nil {
				return nil, err
			}
			if out == nil || aws.ToString(out.InitialConfigurationToken) == "" {
				return nil, fmt.Errorf("unable to retrieve configuration token")
			}
			state.token = *out.InitialConfigurationToken
			state.expires = p.config.Clock().Add(23*time.Hour + 45*time.Minute)
		}
		token := state.token
		// An unsuccessful request may already have consumed its token. Start a new session next time.
		state.token = ""
		out, err := p.client.GetLatestConfiguration(ctx, &sdk.GetLatestConfigurationInput{ConfigurationToken: aws.String(token)}, userAgent)
		if err != nil {
			return nil, err
		}
		if out == nil {
			return nil, fmt.Errorf("empty SDK response")
		}
		state.token = aws.ToString(out.NextPollConfigurationToken)
		state.expires = p.config.Clock().Add(23*time.Hour + 45*time.Minute)
		if len(out.Configuration) > 0 {
			state.value = append([]byte{}, out.Configuration...)
		}
		if state.value == nil {
			return nil, nil
		}
		return append([]byte{}, state.value...), nil
	})
}
