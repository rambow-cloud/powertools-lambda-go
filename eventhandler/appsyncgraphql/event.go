package appsyncgraphql

import "context"

// Event retains unknown fields in a JSON-decoded AppSync event.
type Event = map[string]any

// Handler receives arguments and the complete event. Aggregate batch handlers
// receive the complete event slice in both positions. Context is never replaced.
type Handler func(ctx context.Context, input, event any) (any, error)

type ExceptionHandler func(context.Context, error) (any, error)

// BatchOptions defaults to aggregate execution, as in TypeScript. Individual
// selects sequential per-event execution; failures become null unless ThrowOnError.
type BatchOptions struct {
	Individual   bool
	ThrowOnError bool
}

func isEvent(value any) bool {
	e, ok := value.(map[string]any)
	if !ok || e == nil {
		return false
	}
	for _, key := range []string{"identity", "source", "prev"} {
		if _, ok := e[key]; !ok {
			return false
		}
	}
	for _, key := range []string{"arguments", "request", "info", "stash"} {
		if m, ok := e[key].(map[string]any); !ok || m == nil {
			return false
		}
	}
	request := e["request"].(map[string]any)
	if headers, ok := request["headers"].(map[string]any); !ok || headers == nil {
		return false
	}
	if _, ok := request["domainName"]; !ok {
		return false
	}
	info := e["info"].(map[string]any)
	if _, ok := info["fieldName"].(string); !ok {
		return false
	}
	if _, ok := info["parentTypeName"].(string); !ok {
		return false
	}
	variables, ok := info["variables"].(map[string]any)
	return ok && variables != nil
}

func eventRoute(event any) (string, string) {
	info := event.(map[string]any)["info"].(map[string]any)
	return info["parentTypeName"].(string), info["fieldName"].(string)
}
