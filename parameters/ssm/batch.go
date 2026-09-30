package ssm

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

type ByNameOptions struct {
	parameters.Options
	Decrypt *bool
	// ThrowOnError defaults to true; false returns failed names under _errors.
	ThrowOnError *bool
}

// GetParametersByName supports per-name overrides, groups decryption requirements,
// and fetches at most ten names per batch. Input maps are never modified.
func (p *Provider) GetParametersByName(ctx context.Context, names map[string]GetOptions, options ByNameOptions) (map[string]any, error) {
	strict := options.ThrowOnError == nil || *options.ThrowOnError
	if err := ctx.Err(); err != nil {
		return nil, parameters.GetError("batch", err)
	}
	if _, ok := names["_errors"]; ok && !strict {
		return nil, parameters.GetError("_errors", fmt.Errorf("reserved name in graceful error mode"))
	}
	configs := make(map[string]GetOptions, len(names))
	defaultDecrypt, err := decrypt(options.Decrypt, nil)
	if err != nil {
		return nil, parameters.GetError("batch", err)
	}
	var encrypted, plain []string
	for name, cfg := range names {
		if cfg.MaxAge == nil {
			cfg.MaxAge = options.MaxAge
			if cfg.MaxAge == nil {
				cfg.MaxAge = parameters.Age(5 * time.Second)
			}
		}
		if cfg.Transform == "" {
			cfg.Transform = options.Transform
		}
		if cfg.Decrypt == nil {
			cfg.Decrypt = defaultDecrypt
		}
		// The pinned batch implementation does not honor ForceFetch.
		cfg.ForceFetch = false
		configs[name] = cfg
		if *cfg.Decrypt {
			encrypted = append(encrypted, name)
		} else {
			plain = append(plain, name)
		}
	}
	sort.Strings(encrypted)
	sort.Strings(plain)
	result := make(map[string]any, len(names))
	failures := []string{}
	// The reference uses individual calls for encrypted names in a mixed batch.
	if len(plain) > 0 {
		for _, name := range encrypted {
			value, err := p.get(ctx, name, configs[name])
			if err != nil {
				if strict || ctx.Err() != nil {
					return nil, err
				}
				failures = append(failures, name)
			} else {
				result[name] = value
			}
		}
	} else {
		plain = encrypted
	}
	var pending []string
	for _, name := range plain {
		if value, ok := p.cache.Lookup(name, configs[name].Options); ok {
			result[name] = value
		} else {
			pending = append(pending, name)
		}
	}
	for start := 0; start < len(pending); start += 10 {
		end := min(start+10, len(pending))
		chunk := pending[start:end]
		out, err := p.client.GetParameters(ctx, &sdk.GetParametersInput{Names: chunk, WithDecryption: aws.Bool(len(encrypted) == len(names))}, userAgent)
		if err != nil {
			return nil, parameters.GetError("batch", err)
		}
		if err := ctx.Err(); err != nil {
			return nil, parameters.GetError("batch", err)
		}
		if out == nil {
			return nil, parameters.GetError("batch", fmt.Errorf("empty SDK response"))
		}
		if len(out.InvalidParameters) > 0 && strict {
			return nil, parameters.GetError("batch", fmt.Errorf("failed to fetch parameters: %v", out.InvalidParameters))
		}
		failures = append(failures, out.InvalidParameters...)
		for _, item := range out.Parameters {
			if item.Name == nil {
				continue
			}
			name := *item.Name
			cfg, ok := configs[name]
			if !ok {
				continue
			}
			var value any
			var err error
			if item.Value != nil && *item.Value != "" {
				value, err = parameters.TransformValue(name, *item.Value, cfg.Transform)
			}
			if err != nil && strict {
				return nil, err
			}
			// Preserve the reference's truthiness guard and zero-age fallback for batches.
			if truthy(value) {
				cacheOptions := cfg.Options
				if cacheOptions.MaxAge != nil && *cacheOptions.MaxAge == 0 {
					cacheOptions.MaxAge = parameters.Age(5 * time.Second)
				}
				p.cache.Store(name, value, cacheOptions)
			}
			result[name] = value
		}
	}
	if !strict {
		result["_errors"] = failures
	}
	return result, nil
}

func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return v != ""
	default:
		return true
	}
}
