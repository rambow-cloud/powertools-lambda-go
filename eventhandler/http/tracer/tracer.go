package tracer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
	powertracer "github.com/rambow-cloud/powertools-lambda-go/tracer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Options struct {
	DisableCaptureResponse bool
}

// New records an internal route span using the supplied OTel-backed Tracer.
// It skips HTTP streaming, matching the reference. Use WrapHandler inside
// Streamify for a span covering the entire streaming invocation instead.
// Register it before Compress to observe headers left by that middleware.
// The Lambda wrapper or application owns flushing and provider shutdown.
func New(t *powertracer.Tracer, options ...Options) httpapi.Middleware {
	if t == nil {
		panic("HTTP tracer middleware requires a Tracer instance")
	}
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	return func(r *httpapi.RequestContext, next httpapi.Next) (err error) {
		if !t.Enabled() || r.IsHTTPStreaming {
			return next()
		}
		previousContext, previousRequest := r.Context, r.Request
		path := r.Request.URL.EscapedPath()
		if path == "" {
			path = "/"
		}
		name := r.Request.Method + " " + path
		ctx, end := t.StartSpan(invocation.Ensure(r.Context), name)
		r.Context, r.Request = ctx, previousRequest.WithContext(ctx)
		defer func() { r.Context, r.Request = previousContext, previousRequest }()
		defer func() {
			failure := recover()
			if failure != nil {
				err = fmt.Errorf("HTTP route panic: %v", failure)
			}
			defer func() {
				end(err)
				if failure != nil {
					panic(failure)
				}
			}()
			annotateHTTP(t, r, path)
		}()
		t.AnnotateInvocation(ctx)
		if err = next(); err != nil {
			return err
		}
		if !opts.DisableCaptureResponse && r.Response.Header.Get("Content-Type") == "application/json" {
			var value any
			value, err = responseJSON(r.Response)
			if err == nil {
				t.AddResponseAsMetadata(ctx, name, value)
			}
		}
		return err
	}
}

// responseJSON replays the original bytes and closes the previous body once.
func responseJSON(response *http.Response) (value any, err error) {
	body := response.Body
	if body == nil {
		body = http.NoBody
	}
	response.Body = http.NoBody
	defer func() {
		if closeErr := body.Close(); err == nil {
			err = closeErr
		}
	}()
	data, err := io.ReadAll(body)
	response.Body = io.NopCloser(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &value)
	return value, err
}

func annotateHTTP(t *powertracer.Tracer, r *httpapi.RequestContext, path string) {
	span := trace.SpanFromContext(r.Context)
	url := r.Request.URL.Scheme + "://" + r.Request.URL.Host + path
	request := map[string]any{"method": r.Request.Method, "url": url}
	response := map[string]any{"status": r.Response.StatusCode}
	attrs := []attribute.KeyValue{
		attribute.String("http.request.method", r.Request.Method),
		attribute.String("url.full", url),
		attribute.String("url.path", path),
		attribute.String("url.scheme", r.Request.URL.Scheme),
		attribute.String("server.address", r.Request.URL.Hostname()),
		attribute.Int("http.response.status_code", r.Response.StatusCode),
	}
	if r.Route != "" {
		_, route, _ := strings.Cut(r.Route, " ")
		attrs = append(attrs, attribute.String("http.route", route))
	}
	if value := r.Request.Header.Get("User-Agent"); value != "" {
		request["user_agent"] = value
		attrs = append(attrs, attribute.String("user_agent.original", value))
	}
	if value := r.ClientIP(); value != "" {
		request["client_ip"] = value
		attrs = append(attrs, attribute.String("client.address", value))
	}
	if _, present := r.Request.Header["X-Forwarded-For"]; present {
		request["x_forwarded_for"] = true
		attrs = append(attrs, attribute.Bool("powertools.http.x_forwarded_for", true))
	}
	if length, ok := contentLength(r.Response.Header.Get("Content-Length")); ok {
		response["content_length"] = length
		if length >= 0 && length < math.Exp2(63) {
			attrs = append(attrs, attribute.Int64("http.response.body.size", int64(length)))
		}
	}
	span.SetAttributes(attrs...)
	if r.Response.StatusCode >= 500 {
		span.SetStatus(codes.Error, "")
		span.SetAttributes(attribute.String("error.type", strconv.Itoa(r.Response.StatusCode)))
	}
	// Preserve reference HTTP data without importing the retired X-Ray SDK.
	_ = t.PutMetadata(r.Context, "http", map[string]any{"request": request, "response": response})
}

// contentLength follows the reference's decimal parseInt prefix behavior.
func contentLength(value string) (float64, bool) {
	value = commons.TrimSpace(value)
	end := 0
	if len(value) > 0 && (value[0] == '+' || value[0] == '-') {
		end++
	}
	start := end
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	number, err := strconv.ParseFloat(value[:end], 64)
	return number, err == nil
}
