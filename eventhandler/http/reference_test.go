package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

type referenceRoute struct {
	Path, Method, Action, Error, Message string
	Regex                                bool
	Middleware                           []string
	Value                                json.RawMessage
	Details                              any
	Validation                           map[string]map[string]string
}

func referenceMiddleware(name string) Middleware {
	return func(ctx *RequestContext, next Next) error {
		value, _ := ctx.Store.Get("order")
		order, _ := value.([]string)
		order = append(order, name+":before")
		ctx.Store.Set("order", order)
		if name == "early" {
			return ctx.Respond(Response{StatusCode: 202, Body: map[string]any{"early": true}})
		}
		if name == "fail" {
			return NewHTTPError(400, "middleware failed")
		}
		ctx.Response.Header.Set("x-"+name, "before")
		if err := next(); err != nil {
			return err
		}
		if name == "twice" {
			if err := next(); err != nil {
				return err
			}
		}
		value, _ = ctx.Store.Get("order")
		order = append(value.([]string), name+":after")
		ctx.Store.Set("order", order)
		ctx.Response.Header.Set("x-"+name, "after")
		ctx.Response.Header.Set("x-order", strings.Join(order, "|"))
		if name == "replace" {
			return ctx.Respond(Response{StatusCode: 203, Body: "replaced"})
		}
		return nil
	}
}
func referenceHandler(route referenceRoute) Handler {
	return func(ctx *RequestContext) (any, error) {
		order := []string{}
		if value, exists := ctx.Store.Get("order"); exists {
			order = append(value.([]string), "handler")
			ctx.Store.Set("order", order)
		}
		switch route.Action {
		case "validated":
			body, err := io.ReadAll(ctx.Request.Body)
			valid := map[string]any{}
			if ctx.Valid.Request != nil {
				valid["req"] = ctx.Valid.Request
			}
			if ctx.Valid.Response != nil {
				valid["res"] = ctx.Valid.Response
			}
			return map[string]any{"valid": valid, "body": string(body)}, err
		case "value":
			return route.Value, nil
		case "proxy", "native":
			var value struct {
				StatusCode        int
				Body              json.RawMessage
				Headers           map[string]string
				MultiValueHeaders map[string][]string
				Cookies           []string
				StatusText        string
			}
			if err := json.Unmarshal(route.Value, &value); err != nil {
				return nil, err
			}
			headers := make(nethttp.Header)
			for name, value := range value.Headers {
				headers.Set(name, value)
			}
			multiHeaders := make(nethttp.Header)
			for name, values := range value.MultiValueHeaders {
				for _, value := range values {
					multiHeaders.Add(name, value)
				}
			}
			var body any
			if len(value.Body) > 0 && !null(value.Body) {
				if text, ok := textValue(value.Body); ok {
					body = text
				} else {
					body = value.Body
				}
			}
			if route.Action == "native" {
				response := &nethttp.Response{StatusCode: value.StatusCode, Status: value.StatusText, Header: headers}
				if body != nil {
					response.Body = ioBody([]byte(body.(string)))
					if headers.Get("Content-Type") == "" {
						headers.Set("Content-Type", "text/plain;charset=UTF-8")
					}
				}
				return response, nil
			}
			return Response{StatusCode: value.StatusCode, Headers: headers, MultiValueHeaders: multiHeaders, Body: body, Cookies: value.Cookies}, nil
		case "bytes":
			var values []byte
			if err := json.Unmarshal(route.Value, &values); err != nil {
				return nil, err
			}
			return values, nil
		case "error":
			message := route.Message
			if message == "" {
				message = "handler failed"
			}
			if route.Error == "Error" {
				return nil, errors.New(message)
			}
			codes := map[string]int{"BadRequestError": 400, "UnauthorizedError": 401, "ForbiddenError": 403, "NotFoundError": 404, "MethodNotAllowedError": 405, "RequestTimeoutError": 408, "RequestEntityTooLargeError": 413, "InternalServerError": 500, "ServiceUnavailableError": 503}
			failure := NewHTTPError(codes[route.Error], message)
			failure.Details = route.Details
			return nil, failure
		default:
			body, err := io.ReadAll(ctx.Request.Body)
			if err != nil {
				return nil, err
			}
			headers := map[string]string{}
			for name, values := range ctx.Request.Header {
				headers[strings.ToLower(name)] = strings.Join(values, ", ")
			}
			return map[string]any{"method": ctx.Request.Method, "url": ctx.Request.URL.String(), "headers": headers, "body": string(body), "params": ctx.Params, "route": ctx.Route, "responseType": ctx.ResponseType, "order": order}, nil
		}
	}
}

func referenceCheck(rule string) Check {
	if rule == "" {
		return nil
	}
	return func(_ context.Context, input any) (any, []ValidationIssue, error) {
		if rule == "reject" {
			return nil, []ValidationIssue{{Message: "rejected value", Path: []any{"value"}}}, nil
		}
		if rule == "order" {
			value, _ := input.(map[string]any)
			id, ok := value["id"].(string)
			if !ok {
				return nil, []ValidationIssue{{Message: "id required", Path: []any{"id"}}}, nil
			}
			result := map[string]any{}
			for key, value := range value {
				result[key] = value
			}
			result["id"] = strings.ToUpper(id)
			return result, nil, nil
		}
		return input, nil, nil
	}
}

func TestHTTPReference(t *testing.T) {
	t.Setenv("POWERTOOLS_DEV", "false")
	raw, err := os.ReadFile("testdata/http-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name, Prefix string
			Event        json.RawMessage
			Routes       []referenceRoute
			Middleware   []string
			Child        *struct {
				Prefix, Mount string
				Middleware    []string
				Routes        []referenceRoute
			}
			ErrorHandler *struct {
				Name  string
				Value any
			}
			Expected struct {
				Response       json.RawMessage
				Error, Message string
				Warnings       []string
			}
		}
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, item := range corpus.Cases {
		t.Run(item.Name, func(t *testing.T) {
			warnings := []string{}
			options := Options{Prefix: item.Prefix, Diagnostic: func(level, message string) {
				if level == "warn" {
					warnings = append(warnings, message)
				}
			}}
			router := New(options)
			for _, name := range item.Middleware {
				router.Use(referenceMiddleware(name))
			}
			register := func(router *Router, routes []referenceRoute) {
				for _, route := range routes {
					if route.Method == "" {
						route.Method = "GET"
					}
					var middleware []Middleware
					for _, name := range route.Middleware {
						middleware = append(middleware, referenceMiddleware(name))
					}
					if route.Validation != nil {
						config := ValidationConfig{}
						if checks, ok := route.Validation["req"]; ok {
							config.Request = &RequestChecks{Body: referenceCheck(checks["body"]), Headers: referenceCheck(checks["headers"]), Path: referenceCheck(checks["path"]), Query: referenceCheck(checks["query"])}
						}
						if checks, ok := route.Validation["res"]; ok {
							config.Response = &ResponseChecks{Body: referenceCheck(checks["body"]), Headers: referenceCheck(checks["headers"])}
						}
						middleware = append(middleware, Validate(config))
					}
					if route.Regex {
						_ = router.HandleRegex(route.Method, route.Path, referenceHandler(route), middleware...)
					} else {
						_ = router.Handle(route.Method, route.Path, referenceHandler(route), middleware...)
					}
				}
			}
			register(router, item.Routes)
			if item.Child != nil {
				options.Prefix = item.Child.Prefix
				child := New(options)
				for _, name := range item.Child.Middleware {
					child.Use(referenceMiddleware(name))
				}
				register(child, item.Child.Routes)
				if err := router.IncludeRouter(child, item.Child.Mount); err != nil {
					t.Fatal(err)
				}
			}
			if item.ErrorHandler != nil {
				router.OnError(item.ErrorHandler.Name, func(error, *RequestContext) (any, error) { return item.ErrorHandler.Value, nil })
			}
			result, err := router.Resolve(context.Background(), item.Event)
			if !reflect.DeepEqual(warnings, item.Expected.Warnings) {
				t.Fatalf("warnings: %#v; want %#v", warnings, item.Expected.Warnings)
			}
			if item.Expected.Error != "" {
				if item.Expected.Error == "TypeError" {
					var conversion *RequestConversionError
					if !errors.As(err, &conversion) || err.Error() != item.Expected.Message {
						t.Fatalf("conversion: %v; want %s", err, item.Expected.Message)
					}
					return
				}
				var invalid *InvalidEventError
				if item.Expected.Error != "InvalidEventError" || !errors.As(err, &invalid) {
					t.Fatalf("error: %v; want %s", err, item.Expected.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			actual, _ := json.Marshal(result)
			got, want := normalizedResponse(actual), normalizedResponse(item.Expected.Response)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("response: %s; want %s", actual, item.Expected.Response)
			}
		})
	}
}

// Only JSON body object-key order is normalized. Text and binary bodies,
// headers, cookies, array order, error fields and status values stay exact.
func normalizedResponse(raw []byte) any {
	var response map[string]any
	if err := json.Unmarshal(raw, &response); err != nil {
		panic(err)
	}
	headers, _ := response["headers"].(map[string]any)
	if media, _ := headers["content-type"].(string); strings.Contains(media, "application/json") && response["isBase64Encoded"] == false {
		if body, ok := response["body"].(string); ok {
			var value any
			if json.Unmarshal([]byte(body), &value) == nil {
				response["body"] = value
			}
		}
	}
	return response
}
