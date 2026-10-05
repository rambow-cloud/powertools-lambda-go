// Package http routes native Lambda HTTP events with isolated invocation state.
package http

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type Handler func(*RequestContext) (any, error)
type Next func() error

// Middleware calls next synchronously or short-circuits using RequestContext.Respond.
type Middleware func(*RequestContext, Next) error
type ErrorHandler func(error, *RequestContext) (any, error)
type errorHandler struct {
	name    string
	handler ErrorHandler
}

type Options struct {
	Prefix     string
	Debug      *bool
	Diagnostic func(level, message string)
}
type Router struct {
	mu         sync.RWMutex
	options    Options
	routes     []*route
	middleware []Middleware
	errors     []errorHandler
	Shared     *Store
}

func New(options Options) *Router {
	if options.Debug != nil {
		enabled := *options.Debug
		options.Debug = &enabled
	}
	return &Router{options: options, Shared: &Store{}}
}
func (r *Router) diagnostic(level, message string) {
	if r.options.Diagnostic != nil {
		r.options.Diagnostic(level, message)
	}
}
func (r *Router) Use(middleware ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, middleware...)
}

// OnError registers a reference error name, "HttpError", or the "Error" fallback.
func (r *Router) OnError(name string, handler ErrorHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, previous := range r.errors {
		if previous.name == name {
			r.errors[i].handler = handler
			return
		}
	}
	r.errors = append(r.errors, errorHandler{name, handler})
}
func (r *Router) NotFound(handler ErrorHandler)         { r.OnError("NotFoundError", handler) }
func (r *Router) MethodNotAllowed(handler ErrorHandler) { r.OnError("MethodNotAllowedError", handler) }

func (r *Router) handleError(err error, request *RequestContext) (any, error) {
	r.mu.RLock()
	handlers := append([]errorHandler(nil), r.errors...)
	r.mu.RUnlock()
	visited := make(map[string]bool)
	for {
		if err := request.Context.Err(); err != nil {
			return nil, err
		}
		var selected string
		var handler ErrorHandler
		name := errorName(err)
		var failure *HTTPError
		isHTTP := errors.As(err, &failure)
		for _, candidate := range handlers {
			if candidate.name == name {
				handler = candidate.handler
				selected = candidate.name
				break
			}
		}
		if handler == nil {
			for _, candidate := range handlers {
				if candidate.name == "Error" || candidate.name == "HttpError" && isHTTP {
					handler = candidate.handler
					selected = candidate.name
					break
				}
			}
		}
		if handler != nil {
			if visited[selected] {
				return r.defaultError(err), nil
			}
			visited[selected] = true
			value, handlerErr := handler(err, request)
			if handlerErr != nil {
				var next *HTTPError
				if errors.As(handlerErr, &next) && next != failure {
					err = handlerErr
					continue
				}
				return r.defaultError(handlerErr), nil
			}
			switch value.(type) {
			case Response, *nethttp.Response, []byte:
				return value, nil
			}
			status := 500
			if data, ok := value.(map[string]any); ok {
				copy := make(map[string]any, len(data)+1)
				for key, value := range data {
					copy[key] = value
				}
				if current, ok := data["statusCode"].(int); ok {
					status = current
				} else if current, ok := data["statusCode"].(float64); ok {
					status = int(current)
				} else if isHTTP && (failure.StatusCode == 404 || failure.StatusCode == 405) {
					status = failure.StatusCode
					copy["statusCode"] = status
				}
				value = copy
			}
			body, err := jsonBytes(value)
			if err != nil {
				return nil, err
			}
			return Response{StatusCode: status, Body: string(body)}, nil
		}
		if isHTTP {
			return Response{StatusCode: failure.StatusCode, Body: failure}, nil
		}
		return r.defaultError(err), nil
	}
}
func (r *Router) defaultError(err error) Response {
	enabled := commons.IsDevMode()
	if r.options.Debug != nil {
		enabled = *r.options.Debug
	}
	body := map[string]any{"statusCode": 500, "error": "Internal Server Error", "message": "Internal Server Error"}
	if enabled {
		body["message"] = err.Error()
		body["stack"] = string(debug.Stack())
		body["details"] = map[string]any{"errorName": errorName(err)}
	}
	return Response{StatusCode: 500, Body: body}
}

func executeMiddleware(request *RequestContext, middleware []Middleware, handler Next) error {
	var dispatch func(int) error
	dispatch = func(index int) error {
		if err := request.Context.Err(); err != nil {
			return err
		}
		if index == len(middleware) {
			return handler()
		}
		if middleware[index] == nil {
			return fmt.Errorf("middleware must not be nil")
		}
		called := false
		return middleware[index](request, func() error {
			if called {
				return fmt.Errorf("next() called multiple times")
			}
			called = true
			return dispatch(index + 1)
		})
	}
	return dispatch(0)
}

// Resolve accepts JSON values or typed Lambda events. Panics retain their identity.
// Conversion errors and context cancellation remain Go errors; handler errors
// become HTTP responses through the registered error policy.
func (r *Router) Resolve(ctx context.Context, event any) (ProxyResponse, error) {
	var result ProxyResponse
	err := r.resolve(ctx, event, false, func(current *RequestContext) error {
		encoded := base64Headers(current.Response.Header)
		if current.IsBase64Encoded != nil {
			encoded = *current.IsBase64Encoded
		}
		response := *current.Response
		current.Response.Body = nethttp.NoBody
		var err error
		result, err = WebResponseToProxyResult(&response, current.ResponseType, encoded)
		return err
	})
	if err != nil {
		return ProxyResponse{}, err
	}
	return result, err
}

func (r *Router) resolve(ctx context.Context, event any, streaming bool, consume func(*RequestContext) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	request, kind, raw, err := eventRequest(ctx, event)
	if err != nil {
		var method *InvalidHTTPMethodError
		if errors.As(err, &method) {
			headers := make(nethttp.Header)
			if streaming {
				headers.Set("Transfer-Encoding", "chunked")
			}
			return consume(&RequestContext{Context: ctx, ResponseType: kind, IsHTTPStreaming: streaming, Response: &nethttp.Response{StatusCode: 405, Header: headers}})
		}
		return conversionError(err)
	}
	defer request.Body.Close()
	response := &nethttp.Response{StatusCode: 500, Header: nethttp.Header{"Content-Type": []string{"text/plain;charset=UTF-8"}}, Body: ioBody(nil)}
	if streaming {
		response.Header.Set("Transfer-Encoding", "chunked")
	}
	current := &RequestContext{Context: ctx, Request: request, Response: response, Event: raw, ResponseType: kind, Params: map[string]string{}, Store: &Store{}, Shared: r.Shared, IsHTTPStreaming: streaming}
	defer func() {
		if current.Response != nil && current.Response.Body != nil {
			_ = current.Response.Body.Close()
		}
	}()
	path := request.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	matched, params, middleware, err := r.resolveRoute(request.Method, path)
	if err == nil {
		current.Params = params
		if matched != nil {
			current.Route = matched.method + " " + matched.path
			middleware = append(middleware, matched.middleware...)
		}
		err = executeMiddleware(current, middleware, func() error {
			var value any
			var err error
			if matched == nil {
				value, err = r.handleError(NewHTTPError(404, fmt.Sprintf("Route %s for method %s not found", path, request.Method)), current)
			} else {
				value, err = matched.handler(current)
			}
			if err != nil {
				return err
			}
			return current.Respond(value)
		})
	}
	if cancel := ctx.Err(); cancel != nil {
		return cancel
	}
	if err != nil {
		r.diagnostic("debug", fmt.Sprintf("There was an error processing the request: %v", err))
		value, handled := r.handleError(err, current)
		if handled != nil {
			return handled
		}
		if err := current.Respond(value); err != nil {
			return err
		}
	}
	err = consume(current)
	if cancelled := ctx.Err(); cancelled != nil {
		return cancelled
	}
	return err
}
