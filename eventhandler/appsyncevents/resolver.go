package appsyncevents

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

const MaxEventSize = 240 * 1024

type Resolver struct{ *Router }

func New(options Options) *Resolver { return &Resolver{Router: NewRouter(options)} }

// Resolve accepts a JSON-decoded Lambda event. The context is passed unchanged to
// handlers and diagnostics. Per-item handlers run concurrently; output order is stable.
// Invalid events and successful subscriptions return nil (JSON null in Go Lambda).
func (r *Resolver) Resolve(ctx context.Context, input any) (any, error) {
	event, ok := record(input)
	if !ok || !validEvent(event) {
		r.diagnostic(ctx, "warn", "Received an event that is not compatible with this resolver", nil)
		return nil, nil
	}
	path := event["info"].(map[string]any)["channel"].(map[string]any)["path"].(string)
	items, publish := publishEvents(event)
	if !publish {
		route := r.subscribe.resolve(ctx, path)
		if route == nil {
			return event["events"], nil
		}
		_, err := call(func() (any, error) { return nil, route.subscribe(ctx, event) })
		return r.handleError(ctx, path, err)
	}
	route := r.publish.resolve(ctx, path)
	if route == nil {
		return map[string]any{"events": event["events"]}, nil
	}
	if route.aggregate {
		result, err := call(func() (any, error) {
			value, err := route.publish(ctx, items, event)
			if err == nil && r.options.WarnOnLargePayload && value != nil {
				array := reflect.ValueOf(value)
				if array.Kind() == reflect.Slice || array.Kind() == reflect.Array {
					for i := 0; i < array.Len(); i++ {
						if err := r.warnLarge(ctx, path, array.Index(i).Interface()); err != nil {
							return nil, err
						}
					}
				}
			}
			return value, err
		})
		if err != nil {
			return r.handleError(ctx, path, err)
		}
		response := map[string]any{}
		if _, omitted := result.(Undefined); !omitted {
			response["events"] = result
		}
		return response, nil
	}
	results := make([]any, len(items))
	failures := make([]error, len(items))
	panics := make([]any, len(items))
	var group sync.WaitGroup
	for i, item := range items {
		group.Go(func() {
			defer func() { panics[i] = recover() }()
			message := item.(map[string]any)
			result, err := call(func() (any, error) {
				value, err := route.publish(ctx, message["payload"], event)
				if err != nil {
					return nil, err
				}
				response := map[string]any{"id": message["id"]}
				if _, omitted := value.(Undefined); !omitted {
					response["payload"] = value
				}
				if r.options.WarnOnLargePayload {
					if err := r.warnLarge(ctx, path, response); err != nil {
						return nil, err
					}
				}
				return response, nil
			})
			if err != nil {
				result, failures[i] = r.handleError(ctx, path, err)
				if failures[i] != nil {
					return
				}
				result.(map[string]any)["id"] = message["id"]
			}
			results[i] = result
		})
	}
	group.Wait()
	for _, failure := range panics {
		if failure != nil {
			panic(failure)
		}
	}
	for _, failure := range failures {
		if failure != nil {
			return nil, failure
		}
	}
	return map[string]any{"events": results}, nil
}

func (r *Resolver) handleError(ctx context.Context, path string, err error) (any, error) {
	if err == nil {
		return nil, nil
	}
	r.diagnostic(ctx, "error", "An error occurred in handler "+path, err)
	var unauthorized *UnauthorizedError
	if errors.As(err, &unauthorized) {
		return nil, err
	}
	return errorResponse(err), nil
}

func (r *Router) warnLarge(ctx context.Context, path string, value any) error {
	r.largeMu.Lock()
	warned := r.largeWarnings[path]
	r.largeMu.Unlock()
	if warned {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return &NamedError{Name: "TypeError", Message: err.Error(), Cause: err}
	}
	size := len(data)
	if size <= MaxEventSize {
		return nil
	}
	r.largeMu.Lock()
	warned = r.largeWarnings[path]
	r.largeWarnings[path] = true
	r.largeMu.Unlock()
	if !warned {
		r.diagnostic(ctx, "warn", fmt.Sprintf("One or more events published to channel '%s' exceed the AWS AppSync Events per-event size limit of %d bytes (got %d bytes). Events larger than this limit are silently dropped by AppSync and will not be delivered to subscribers.", path, MaxEventSize, size), nil)
	}
	return nil
}
