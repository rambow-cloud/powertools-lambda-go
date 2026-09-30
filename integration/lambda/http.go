package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"
	"sync/atomic"

	"github.com/aws/aws-lambda-go/lambdacontext"
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	httpmetrics "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics"
	httptracer "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/metrics"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

func httpJSONCheck(schema *validation.Schema) httpapi.Check {
	return func(ctx context.Context, input any) (any, []httpapi.ValidationIssue, error) {
		value, err := schema.Validate(ctx, input)
		var failure *validation.SchemaValidationError
		if !errors.As(err, &failure) {
			return value, nil, err
		}
		issues := make([]httpapi.ValidationIssue, 0, len(failure.Issues))
		for _, issue := range failure.Issues {
			path := []any{}
			if issue.InstancePath != "" {
				for _, part := range strings.Split(strings.TrimPrefix(issue.InstancePath, "/"), "/") {
					path = append(path, strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~"))
				}
			}
			issues = append(issues, httpapi.ValidationIssue{Message: issue.Message, Path: path})
		}
		return nil, issues, nil
	}
}

func newHTTPProbe(log *logger.Logger, tr *tracer.Tracer, m *metrics.Metrics) func(context.Context) (map[string]any, error) {
	app := httpapi.New(httpapi.Options{})
	app.Shared.Set("service", "orders")
	app.Use(httpmetrics.New(m), httptracer.New(tr, httptracer.Options{DisableCaptureResponse: true}))
	app.Use(func(request *httpapi.RequestContext, next httpapi.Next) error {
		invocation, _ := lambdacontext.FromContext(request.Context)
		if err := m.WithContext(request.Context).AddMetadata("http_invocation_id", invocation.AwsRequestID); err != nil {
			return err
		}
		return next()
	})
	maxAge := float64(600)
	app.Use(httpapi.CORS(httpapi.CORSOptions{Origins: []string{"https://app.example.test"}, Credentials: true, MaxAge: &maxAge}))
	app.Use(httpapi.Compress(httpapi.CompressionOptions{}))
	app.Use(func(request *httpapi.RequestContext, next httpapi.Next) error {
		invocation, _ := lambdacontext.FromContext(request.Context)
		request.Store.Set("request_id", invocation.AwsRequestID)
		if err := next(); err != nil {
			return err
		}
		request.Response.Header.Set("X-Request-ID", invocation.AwsRequestID)
		return nil
	})
	pathSchema, setupErr := validation.Compile(context.Background(), json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","pattern":"^order-"}},"required":["id"]}`), validation.Options{})
	if setupErr != nil {
		return func(context.Context) (map[string]any, error) { return nil, setupErr }
	}
	outputSchema, setupErr := validation.Compile(context.Background(), json.RawMessage(`{"type":"object","properties":{"status":{"const":"ok"}},"required":["status"]}`), validation.Options{})
	if setupErr != nil {
		return func(context.Context) (map[string]any, error) { return nil, setupErr }
	}
	bodySchema := parser.Object(parser.Field{Name: "id", Schema: parser.String()})
	bodyCheck := func(ctx context.Context, input any) (any, []httpapi.ValidationIssue, error) {
		value, failures, err := bodySchema.Validate(ctx, input)
		issues := make([]httpapi.ValidationIssue, 0, len(failures))
		for _, failure := range failures {
			issues = append(issues, httpapi.ValidationIssue{Message: failure.Message, Path: failure.Path})
		}
		return value, issues, err
	}
	var calls atomic.Int64
	setupErr = app.Post("/orders/:id", func(request *httpapi.RequestContext) (any, error) {
		calls.Add(1)
		return tracer.Capture(request.Context, tr, "http-route", func(ctx context.Context) (any, error) {
			identity, _ := request.Store.Get("request_id")
			service, _ := request.Shared.Get("service")
			_ = log.WithContext(ctx).Info("http route", logger.Fields{"http_route": request.Route, "http_request_id": identity})
			return httpapi.Response{StatusCode: 201, Body: map[string]any{"status": "ok", "id": request.Valid.Request["body"].(map[string]any)["id"], "path": request.Params["id"], "request_id": identity, "service": service}}, nil
		})
	}, httpapi.Validate(httpapi.ValidationConfig{Request: &httpapi.RequestChecks{Body: bodyCheck, Path: httpJSONCheck(pathSchema)}, Response: &httpapi.ResponseChecks{Body: httpJSONCheck(outputSchema)}}))
	if setupErr == nil {
		setupErr = app.Get("/binary", func(*httpapi.RequestContext) (any, error) { return []byte{0, 1, 255}, nil })
	}
	for _, encoding := range []string{"gzip", "deflate"} {
		if setupErr != nil {
			break
		}
		threshold := float64(0)
		setupErr = app.Get("/compressed/"+encoding, func(request *httpapi.RequestContext) (any, error) {
			headers := make(nethttp.Header)
			if request.Request.Header.Get("X-No-Transform") == "true" {
				headers.Set("Cache-Control", "no-transform")
			}
			return httpapi.Response{StatusCode: 200, Body: "hello λ 世界", Headers: headers}, nil
		}, httpapi.Compress(httpapi.CompressionOptions{Encoding: encoding, Threshold: &threshold}))
	}
	return func(ctx context.Context) (map[string]any, error) {
		if setupErr != nil {
			return nil, setupErr
		}
		before := calls.Load()
		result := map[string]any{}
		for _, kind := range []string{"v1", "v2", "alb", "url"} {
			response, err := app.Resolve(ctx, httpProbeEvent(kind, "/orders/order-a", "POST", `{"id":"body-a"}`))
			if err != nil {
				return nil, err
			}
			result[kind] = response
			for _, probe := range []struct {
				name, path, method string
				headers            map[string]any
			}{
				{"preflight", "/missing", "OPTIONS", map[string]any{"access-control-request-method": "POST", "access-control-request-headers": "Content-Type"}},
				{"denied_preflight", "/missing", "OPTIONS", map[string]any{"access-control-request-method": "POST", "access-control-request-headers": "X-Not-Allowed"}},
				{"gzip", "/compressed/gzip", "GET", map[string]any{"accept-encoding": "gzip"}},
				{"deflate", "/compressed/deflate", "GET", map[string]any{"accept-encoding": "deflate"}},
				{"identity", "/compressed/gzip", "GET", map[string]any{"accept-encoding": "identity, gzip"}},
				{"no_transform", "/compressed/gzip", "GET", map[string]any{"accept-encoding": "gzip", "x-no-transform": "true"}},
			} {
				event := httpProbeEvent(kind, probe.path, probe.method, "")
				for key, value := range probe.headers {
					event["headers"].(map[string]any)[key] = value
				}
				value, err := app.Resolve(ctx, event)
				if err != nil {
					return nil, fmt.Errorf("%s/%s: %w", kind, probe.name, err)
				}
				result[kind+"_"+probe.name] = value
			}
		}
		for _, test := range []struct{ name, path, method, body string }{
			{"parser_rejected", "/orders/order-a", "POST", `{"id":1}`},
			{"schema_rejected", "/orders/wrong", "POST", `{"id":"body-a"}`},
			{"missing", "/missing", "GET", ""},
			{"unsupported", "/orders/order-a", "TRACE", ""},
			{"binary", "/binary", "GET", ""},
		} {
			response, err := app.Resolve(ctx, httpProbeEvent("v2", test.path, test.method, test.body))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", test.name, err)
			}
			result[test.name] = response
		}
		result["handler_calls"] = calls.Load() - before
		return result, nil
	}
}

func httpProbeEvent(kind, path, method, body string) map[string]any {
	headers := map[string]any{"content-type": "application/json", "origin": "https://app.example.test"}
	event := map[string]any{"headers": headers, "isBase64Encoded": false}
	if kind == "v2" || kind == "url" {
		domain := "api.example.test"
		if kind == "url" {
			domain = "id.lambda-url.ap-east-1.on.aws"
		}
		event["version"], event["routeKey"], event["rawPath"], event["rawQueryString"] = "2.0", "$default", path, ""
		event["requestContext"] = map[string]any{"domainName": domain, "http": map[string]any{"method": method}}
		if body != "" {
			event["body"] = body
		}
	} else {
		event["httpMethod"], event["path"], event["body"] = method, path, body
		if body == "" {
			event["body"] = nil
		}
		event["requestContext"] = map[string]any{"domainName": "api.example.test"}
		if kind == "alb" {
			event["requestContext"] = map[string]any{"elb": map[string]any{}}
		} else {
			event["resource"] = "/{proxy+}"
			for _, key := range []string{"pathParameters", "queryStringParameters", "multiValueQueryStringParameters", "stageVariables"} {
				event[key] = nil
			}
		}
	}
	return event
}
