package appsyncevents

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"unicode/utf16"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type Options struct {
	WarnOnLargePayload bool
	// Diagnostic may be called concurrently. Registration uses context.Background;
	// resolution and handler errors retain the invocation context. Callbacks run unlocked.
	Diagnostic func(ctx context.Context, level, message string, err error)
}

// Router stores separate publish and subscribe registries. Register routes before
// serving requests: successful lookup caches intentionally retain old registrations.
type Router struct {
	options            Options
	publish, subscribe *registry
	largeMu            sync.Mutex
	largeWarnings      map[string]bool
	debug              bool
}

type route struct {
	path        string
	pattern     *regexp.Regexp
	publish     PublishHandler
	subscribe   SubscribeHandler
	aggregate   bool
	specificity int
}

type registry struct {
	mu     sync.Mutex
	routes []*route
	cache  *commons.LRUCache[string, *route]
	warned map[string]bool
	kind   string
	router *Router
}

const lineEnd = "$"

var validPath = regexp.MustCompile(`^/([^/*]+)(/[^/*]+)*(/\*)?` + lineEnd)

func NewRouter(options Options) *Router {
	level, _ := commons.StringEnv("AWS_LAMBDA_LOG_LEVEL", "")
	r := &Router{options: options, largeWarnings: map[string]bool{}, debug: level == "DEBUG"}
	makeRegistry := func(kind string) *registry {
		return &registry{kind: kind, router: r, cache: commons.NewLRUCache[string, *route](100), warned: map[string]bool{}}
	}
	r.publish, r.subscribe = makeRegistry("onPublish"), makeRegistry("onSubscribe")
	return r
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
	} else {
		fmt.Fprintln(output, message, err)
	}
}

func (r *Router) OnPublish(path string, handler PublishHandler, options ...PublishOptions) {
	aggregate := len(options) > 0 && options[0].Aggregate
	r.publish.register(&route{path: path, publish: handler, aggregate: aggregate})
}

func (r *Router) OnSubscribe(path string, handler SubscribeHandler) {
	r.subscribe.register(&route{path: path, subscribe: handler})
}

func (r *registry) register(item *route) {
	ctx := context.Background()
	r.router.diagnostic(ctx, "debug", fmt.Sprintf("Registering %s route handler for path '%s' with aggregate '%t'", r.kind, item.path, item.aggregate), nil)
	if item.path != "/*" && !validPath.MatchString(item.path) {
		r.router.diagnostic(ctx, "warn", fmt.Sprintf("The path '%s' registered for %s is not valid and will be skipped. A path should always have a namespace starting with '/'. A path can have multiple namespaces, all separated by '/'. Wildcards are allowed only at the end of the path.", item.path, r.kind), nil)
		return
	}
	pattern := strings.ReplaceAll(regexp.QuoteMeta(item.path), `\*`, `[^\n\r`+"\u2028\u2029"+`]*`)
	item.pattern = regexp.MustCompile("^" + pattern + lineEnd)
	item.specificity = len(utf16.Encode([]rune(item.path)))
	if strings.HasSuffix(item.path, "*") {
		item.specificity--
	}
	r.mu.Lock()
	duplicate := false
	for i, previous := range r.routes {
		if previous.path == item.path {
			r.routes[i] = item
			duplicate = true
			break
		}
	}
	if !duplicate {
		r.routes = append(r.routes, item)
	}
	r.mu.Unlock()
	if duplicate {
		r.router.diagnostic(ctx, "warn", fmt.Sprintf("A route handler for path '%s' is already registered for %s. The previous handler will be replaced.", item.path, r.kind), nil)
	}
}

func (r *registry) resolve(ctx context.Context, path string) *route {
	r.mu.Lock()
	if item, ok := r.cache.Get(path); ok {
		r.mu.Unlock()
		return item
	}
	r.mu.Unlock()
	r.router.diagnostic(ctx, "debug", fmt.Sprintf("Resolving handler for path '%s'", path), nil)
	r.mu.Lock()
	var selected *route
	for _, item := range r.routes {
		if item.pattern.MatchString(path) && (selected == nil || item.specificity > selected.specificity) {
			selected = item
		}
	}
	if selected != nil {
		r.cache.Add(path, selected)
	}
	warn := selected == nil && !r.warned[path]
	if warn {
		r.warned[path] = true
	}
	r.mu.Unlock()
	if warn {
		r.router.diagnostic(ctx, "warn", fmt.Sprintf("No route handler found for path '%s' registered for %s.", path, r.kind), nil)
	}
	return selected
}
