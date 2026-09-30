package http

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type route struct {
	method, path string
	handler      Handler
	middleware   []Middleware
	pattern      *regexp.Regexp
	parameters   int
	regex        bool
}

var parameterName = regexp.MustCompile(`:([a-zA-Z_][a-zA-Z_0-9]*)`)

func prefixed(path, prefix string) string {
	if prefix == "" {
		return path
	}
	return strings.TrimSuffix(strings.TrimRight(prefix, "/")+path, "/")
}
func compileRoute(method, path string, regex bool, handler Handler, middleware []Middleware) (*route, error) {
	if handler == nil {
		return nil, fmt.Errorf("route handler is required")
	}
	var pattern strings.Builder
	seen := map[string]bool{}
	end, count := 0, 0
	for _, match := range parameterName.FindAllStringSubmatchIndex(path, -1) {
		if match[1] != len(path) && path[match[1]] != '/' {
			continue
		}
		name := path[match[2]:match[3]]
		if seen[name] {
			return nil, fmt.Errorf("Duplicate parameter names: %s", name)
		}
		seen[name] = true
		pattern.WriteString(path[end:match[0]])
		pattern.WriteString(`(?P<` + name + `>[-._~()'!*:@,;=+&$%<> \[\]{}|^\w]+)`)
		end, count = match[1], count+1
	}
	if strings.Count(path, ":") != count {
		return nil, fmt.Errorf("Malformed parameter syntax. Use :paramName format.")
	}
	pattern.WriteString(path[end:])
	compiled, err := regexp.Compile("^" + pattern.String() + "$")
	if err != nil {
		return nil, err
	}
	return &route{method: method, path: path, handler: handler, middleware: slices.Clone(middleware), pattern: compiled, parameters: count, regex: regex}, nil
}
func (r *Router) register(method, path string, regex bool, handler Handler, middleware []Middleware) error {
	compiled, err := compileRoute(method, path, regex, handler, middleware)
	if err != nil {
		r.diagnostic("warn", err.Error())
		return err
	}
	r.mu.Lock()
	duplicate := false
	for index, previous := range r.routes {
		if previous.method == method && previous.path == path && previous.regex == regex {
			r.routes[index] = compiled
			duplicate = true
			break
		}
	}
	if !duplicate {
		r.routes = append(r.routes, compiled)
	}
	// Registration is infrequent; preserve lookup precedence without sorting
	// or allocating route buckets on every invocation.
	slices.SortStableFunc(r.routes, func(a, b *route) int {
		category := func(route *route) int {
			if route.regex {
				return 2
			}
			if route.parameters > 0 {
				return 1
			}
			return 0
		}
		if difference := category(a) - category(b); difference != 0 {
			return difference
		}
		if category(a) != 1 {
			return 0
		}
		if a.parameters != b.parameters {
			return a.parameters - b.parameters
		}
		return strings.Count(b.path, "/") - strings.Count(a.path, "/")
	})
	r.mu.Unlock()
	if duplicate {
		r.diagnostic("warn", fmt.Sprintf("Handler for method: %s and path: %s already exists. The previous handler will be replaced.", method, path))
	}
	return nil
}

// Handle registers a string path. Dynamic parameters use :name syntax.
func (r *Router) Handle(method, path string, handler Handler, middleware ...Middleware) error {
	return r.register(method, prefixed(path, r.options.Prefix), false, handler, middleware)
}

// HandleRegex registers an anchored Go regular expression after dynamic routes.
// Use named captures for parameters. JavaScript-only regex forms remain unsupported.
func (r *Router) HandleRegex(method, pattern string, handler Handler, middleware ...Middleware) error {
	return r.register(method, prefixed(pattern, r.options.Prefix), true, handler, middleware)
}
func (r *Router) Get(path string, h Handler, m ...Middleware) error {
	return r.Handle("GET", path, h, m...)
}
func (r *Router) Post(path string, h Handler, m ...Middleware) error {
	return r.Handle("POST", path, h, m...)
}
func (r *Router) Put(path string, h Handler, m ...Middleware) error {
	return r.Handle("PUT", path, h, m...)
}
func (r *Router) Patch(path string, h Handler, m ...Middleware) error {
	return r.Handle("PATCH", path, h, m...)
}
func (r *Router) Delete(path string, h Handler, m ...Middleware) error {
	return r.Handle("DELETE", path, h, m...)
}
func (r *Router) Head(path string, h Handler, m ...Middleware) error {
	return r.Handle("HEAD", path, h, m...)
}
func (r *Router) Options(path string, h Handler, m ...Middleware) error {
	return r.Handle("OPTIONS", path, h, m...)
}

func (r *Router) resolveRoute(method, path string) (*route, map[string]string, []Middleware, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	global := slices.Clone(r.middleware)
	for _, route := range r.routes {
		if route.method != method {
			continue
		}
		if !route.regex && route.parameters == 0 {
			if route.path == path {
				return route, map[string]string{}, global, nil
			}
			continue
		}
		matches := route.pattern.FindStringSubmatch(path)
		if matches == nil {
			continue
		}
		params := map[string]string{}
		for index, name := range route.pattern.SubexpNames() {
			if name == "" {
				continue
			}
			value, err := url.PathUnescape(matches[index])
			if err != nil || !utf8.ValidString(value) {
				return nil, nil, global, fmt.Errorf("URI malformed")
			}
			if commons.TrimSpace(value) == "" {
				return nil, nil, global, &ParameterValidationError{fmt.Sprintf("Parameter validation failed: Parameter '%s' cannot be empty", name)}
			}
			params[name] = value
		}
		return route, params, global, nil
	}
	return nil, map[string]string{}, global, nil
}

// IncludeRouter copies routes, global middleware, error handlers and shared keys.
// Included global middleware applies to the parent router, matching the reference.
func (r *Router) IncludeRouter(child *Router, prefix string) error {
	if child == nil || child == r {
		return fmt.Errorf("a distinct child router is required")
	}
	child.mu.RLock()
	routes := slices.Clone(child.routes)
	middleware := slices.Clone(child.middleware)
	handlers := slices.Clone(child.errors)
	child.mu.RUnlock()
	for _, route := range routes {
		if err := r.register(route.method, prefixed(route.path, prefix), route.regex, route.handler, route.middleware); err != nil {
			return err
		}
	}
	r.Use(middleware...)
	for _, handler := range handlers {
		r.OnError(handler.name, handler.handler)
	}
	for key, value := range child.Shared.Entries() {
		r.Shared.Set(key, value)
	}
	return nil
}
