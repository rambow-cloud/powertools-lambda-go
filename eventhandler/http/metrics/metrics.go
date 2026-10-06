package metrics

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	powermetrics "github.com/rambow-cloud/powertools-lambda-go/metrics"
)

// Options controls optional HTTP metrics beyond the TypeScript reference contract.
type Options struct {
	// CaptureRequestCount emits request=1 with unit Count per middleware execution.
	// The default emits only latency, fault and error.
	CaptureRequestCount bool
}

// New emits latency, fault and error metrics with a bounded route dimension.
// Options can enable an additional request counter without changing the defaults.
// Each request owns a metric scope, including metrics added downstream using
// m.WithContext(request.Context). It publishes before streaming body transfer,
// matching the reference middleware. The parent scope remains unchanged.
// Publication failures propagate as errors without discarding business errors.
func New(m *powermetrics.Metrics, options ...Options) httpapi.Middleware {
	if m == nil {
		panic("HTTP metrics middleware requires a Metrics instance")
	}
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	return func(request *httpapi.RequestContext, next httpapi.Next) (err error) {
		started := time.Now()
		previousContext, previousRequest := request.Context, request.Request
		ctx, finish := m.StartScope(previousContext)
		request.Context, request.Request = ctx, previousRequest.WithContext(ctx)
		defer func() {
			request.Context, request.Request = previousContext, previousRequest
		}()
		status := 500
		defer func() {
			var failure *httpapi.HTTPError
			if errors.As(err, &failure) {
				status = failure.StatusCode
			}
			// Finish must run even if a custom metric writer panics.
			defer func() { err = combine(err, finish()) }()
			err = combine(err, record(m.WithContext(ctx), request, status, time.Since(started), opts))
		}()
		err = next()
		if err == nil {
			status = request.Response.StatusCode
		}
		return err
	}
}

func combine(first, second error) error {
	if first == nil {
		return second
	}
	if second == nil {
		return first
	}
	return errors.Join(first, second)
}

func record(m *powermetrics.Metrics, r *httpapi.RequestContext, status int, elapsed time.Duration, options Options) error {
	path := r.Request.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	metadata := map[string]any{"httpMethod": r.Request.Method, "path": path, "statusCode": strconv.Itoa(status)}
	if agent := r.Request.Header.Get("User-Agent"); agent != "" {
		metadata["userAgent"] = agent
	}
	if ip := r.ClientIP(); ip != "" {
		metadata["ipAddress"] = ip
	}
	if r.ResponseType != httpapi.ALB {
		var event struct {
			RequestContext map[string]json.RawMessage `json:"requestContext"`
		}
		if err := json.Unmarshal(r.Event, &event); err != nil {
			return err
		}
		for _, field := range []struct{ source, target string }{{"requestId", "apiGwRequestId"}, {"apiId", "apiGwApiId"}} {
			if value, present := event.RequestContext[field.source]; present {
				metadata[field.target] = value
			}
		}
		if r.ResponseType == httpapi.APIGatewayV1 {
			var extended string
			if json.Unmarshal(event.RequestContext["extendedRequestId"], &extended) == nil && extended != "" {
				metadata["apiGwExtendedRequestId"] = extended
			}
		}
	}
	for key, value := range metadata {
		if err := m.AddMetadata(key, value); err != nil {
			return err
		}
	}
	route := r.Route
	if route == "" {
		route = "NOT_FOUND"
	}
	var dimensionErr error
	if options.CaptureRequestCount {
		// Scope-local defaults survive automatic and single-metric publication.
		dimensionErr = m.SetDefaultDimensions(powermetrics.Dimensions{"route": route})
	} else {
		dimensionErr = m.AddDimension("route", route)
	}
	if dimensionErr != nil {
		return dimensionErr
	}
	fault, clientError := float64(0), float64(0)
	if status >= 500 {
		fault = 1
	} else if status >= 400 {
		clientError = 1
	}
	for _, metric := range []struct {
		name  string
		unit  powermetrics.Unit
		value float64
	}{{"latency", powermetrics.Milliseconds, float64(elapsed) / float64(time.Millisecond)}, {"fault", powermetrics.Count, fault}, {"error", powermetrics.Count, clientError}} {
		if err := m.AddMetric(metric.name, metric.unit, metric.value); err != nil {
			return err
		}
	}
	if options.CaptureRequestCount {
		return m.AddMetric("request", powermetrics.Count, 1)
	}
	return nil
}
