package appsyncgraphql

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type Options struct {
	// Diagnostic runs outside registry locks and may be called concurrently.
	// Registration uses context.Background; invocation diagnostics retain context.
	Diagnostic func(ctx context.Context, level, message string, err error)
}

type route struct {
	typeName, field string
	handler         Handler
	options         BatchOptions
}
type exception struct {
	name    string
	handler ExceptionHandler
}

// Router owns ordered registries. Configure routes before serving requests.
// Concurrent lookups and registration are safe; inclusion copies registrations.
type Router struct {
	mu            sync.RWMutex
	single, batch []route
	exceptions    []exception
	options       Options
	debug         bool
}

func NewRouter(options Options) *Router {
	level, _ := commons.StringEnv("AWS_LAMBDA_LOG_LEVEL", "")
	return &Router{options: options, debug: level == "DEBUG"}
}

func (r *Router) diagnostic(ctx context.Context, level, message string, err error) {
	if r.options.Diagnostic != nil {
		r.options.Diagnostic(ctx, level, message, err)
		return
	}
	if level == "debug" && !r.debug {
		return
	}
	output := os.Stderr
	if level == "debug" {
		output = os.Stdout
	}
	if err == nil {
		fmt.Fprintln(output, message)
	} else if message == "" {
		fmt.Fprintln(output, err)
	} else {
		fmt.Fprintln(output, message, err)
	}
}

func (r *Router) OnQuery(field string, handler Handler)    { r.OnResolver("Query", field, handler) }
func (r *Router) OnMutation(field string, handler Handler) { r.OnResolver("Mutation", field, handler) }
func (r *Router) OnResolver(typeName, field string, handler Handler) {
	r.register(route{typeName: typeName, field: field, handler: handler}, false, true)
}
func (r *Router) OnBatchQuery(field string, handler Handler, options ...BatchOptions) {
	r.OnBatchResolver("Query", field, handler, options...)
}
func (r *Router) OnBatchMutation(field string, handler Handler, options ...BatchOptions) {
	r.OnBatchResolver("Mutation", field, handler, options...)
}
func (r *Router) OnBatchResolver(typeName, field string, handler Handler, options ...BatchOptions) {
	var opts BatchOptions
	if len(options) > 0 {
		opts = options[0]
	}
	r.register(route{typeName: typeName, field: field, handler: handler, options: opts}, true, true)
}

func (r *Router) register(item route, batch, announce bool) {
	ctx := context.Background()
	if announce {
		r.diagnostic(ctx, "debug", fmt.Sprintf("Adding resolver for field %s.%s", item.typeName, item.field), nil)
	}
	r.mu.Lock()
	items := &r.single
	if batch {
		items = &r.batch
	}
	replaced := false
	for i, current := range *items {
		// The reference concatenates keys, including otherwise ambiguous dots.
		if current.typeName+"."+current.field == item.typeName+"."+item.field {
			(*items)[i] = item
			replaced = true
			break
		}
	}
	if !replaced {
		*items = append(*items, item)
	}
	r.mu.Unlock()
	if replaced {
		r.diagnostic(ctx, "warn", fmt.Sprintf("A resolver for field '%s' is already registered for '%s'. The previous resolver will be replaced.", item.field, item.typeName), nil)
	}
}

// OnException registers exact error names, not inheritance or predicate matching.
func (r *Router) OnException(names []string, handler ExceptionHandler) {
	for _, name := range names {
		r.registerException(exception{name, handler}, true)
	}
}

func (r *Router) registerException(item exception, announce bool) {
	ctx := context.Background()
	if announce {
		r.diagnostic(ctx, "debug", "Adding exception handler for error class "+item.name, nil)
	}
	r.mu.Lock()
	replaced := false
	for i, current := range r.exceptions {
		if current.name == item.name {
			r.exceptions[i] = item
			replaced = true
			break
		}
	}
	if !replaced {
		r.exceptions = append(r.exceptions, item)
	}
	r.mu.Unlock()
	if replaced {
		r.diagnostic(ctx, "warn", fmt.Sprintf("An exception handler for error class '%s' is already registered. The previous handler will be replaced.", item.name), nil)
	}
}

// IncludeRouter merges in order, replacing collisions without sharing registries.
func (r *Router) IncludeRouter(routers ...*Router) {
	ctx := context.Background()
	r.diagnostic(ctx, "debug", "Including router", nil)
	for _, other := range routers {
		other.mu.RLock()
		single, batch := append([]route(nil), other.single...), append([]route(nil), other.batch...)
		exceptions := append([]exception(nil), other.exceptions...)
		other.mu.RUnlock()
		for _, item := range single {
			r.register(item, false, false)
		}
		for _, item := range batch {
			r.register(item, true, false)
		}
		for _, item := range exceptions {
			r.registerException(item, false)
		}
	}
	r.diagnostic(ctx, "debug", "Router included successfully", nil)
}

func (r *Router) lookup(ctx context.Context, typeName, field string, batch bool) (route, bool) {
	r.diagnostic(ctx, "debug", fmt.Sprintf("Looking for resolver for type=%s, field=%s", typeName, field), nil)
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := r.single
	if batch {
		items = r.batch
	}
	for _, item := range items {
		if item.typeName+"."+item.field == typeName+"."+field {
			return item, true
		}
	}
	return route{}, false
}

func (r *Router) lookupException(ctx context.Context, name string) ExceptionHandler {
	r.diagnostic(ctx, "debug", "Looking for exception handler for error: "+name, nil)
	r.mu.RLock()
	var handler ExceptionHandler
	for _, item := range r.exceptions {
		if item.name == name {
			handler = item.handler
			break
		}
	}
	r.mu.RUnlock()
	if handler != nil {
		r.diagnostic(ctx, "debug", "Found exact match for error class: "+name, nil)
	} else {
		r.diagnostic(ctx, "debug", "No exception handler found for error: "+name, nil)
	}
	return handler
}
