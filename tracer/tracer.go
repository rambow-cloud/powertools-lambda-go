// Package tracer provides Lambda-aware tracing with interchangeable backends.
package tracer

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/smithy-go/middleware"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/commons/awssdk"
	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
	"go.opentelemetry.io/otel/trace"
)

type Kind int

const (
	Internal Kind = iota
	Handler
)

// Span is the common surface supported by OpenTelemetry and the optional X-Ray backend.
type Span interface {
	Annotation(string, any) error
	Metadata(namespace, key string, value any) error
	RecordError(error, bool)
	End()
	TraceID() string
	Sampled() bool
}

// Backend owns instrumentation mechanics. Implementations must be concurrency-safe.
// Shutdown must not close a provider owned by the application unless explicitly agreed.
// ForceFlush must honor context cancellation and deadlines; the wrapper cannot forcibly
// stop a custom implementation that ignores its context.
type Backend interface {
	Start(context.Context, string, Kind) (context.Context, Span, error)
	HTTPTransport(http.RoundTripper) http.RoundTripper
	InstrumentAWS(*[]func(*middleware.Stack) error)
	ForceFlush(context.Context) error
	Shutdown(context.Context) error
}

type config struct {
	service                                string
	enabled, local, response, errors, http bool
	backend                                Backend
	flushTimeout                           time.Duration
	onError                                func(error)
}

type Option func(*config)

func WithServiceName(name string) Option {
	return func(c *config) { c.service = commons.ResolveServiceName(name, "service_undefined") }
}
func WithBackend(backend Backend) Option { return func(c *config) { c.backend = backend } }
func WithEnabled(enabled bool) Option    { return func(c *config) { c.enabled = enabled } }

// WithLocalTracing explicitly permits tracing outside Lambda for development/tests.
func WithLocalTracing(enabled bool) Option    { return func(c *config) { c.local = enabled } }
func WithCaptureResponse(enabled bool) Option { return func(c *config) { c.response = enabled } }
func WithCaptureError(enabled bool) Option    { return func(c *config) { c.errors = enabled } }
func WithCaptureHTTP(enabled bool) Option     { return func(c *config) { c.http = enabled } }
func WithFlushTimeout(timeout time.Duration) Option {
	return func(c *config) {
		if timeout > 0 {
			c.flushTimeout = timeout
		}
	}
}
func WithErrorHandler(handler func(error)) Option {
	return func(c *config) {
		if handler != nil {
			c.onError = handler
		}
	}
}

type Tracer struct {
	cfg     config
	backend Backend
}

// New defaults to OpenTelemetry with an OTLP/HTTP exporter. The exporter uses the
// standard OTEL_EXPORTER_OTLP_* environment settings; no global provider is changed.
// Disabled tracers create no exporter. Initialize once outside the Lambda handler.
func New(options ...Option) (*Tracer, error) {
	c := config{service: commons.ResolveServiceName("", "service_undefined"), enabled: true, response: true, errors: true, http: true,
		flushTimeout: 2 * time.Second, onError: func(error) {}}
	for _, option := range options {
		option(&c)
	}
	// A false environment flag remains a veto, as in the pinned TypeScript Tracer.
	c.enabled = c.enabled && !strings.EqualFold(os.Getenv("POWERTOOLS_TRACE_ENABLED"), "false") && !commons.IsDevMode()
	c.response = c.response && !strings.EqualFold(os.Getenv("POWERTOOLS_TRACER_CAPTURE_RESPONSE"), "false")
	c.errors = c.errors && !strings.EqualFold(os.Getenv("POWERTOOLS_TRACER_CAPTURE_ERROR"), "false")
	c.http = c.http && !strings.EqualFold(os.Getenv("POWERTOOLS_TRACER_CAPTURE_HTTPS_REQUESTS"), "false")
	if !c.local && (os.Getenv("AWS_EXECUTION_ENV") == "" && os.Getenv("AWS_LAMBDA_RUNTIME_API") == "" || os.Getenv("AWS_SAM_LOCAL") != "" || os.Getenv("AWS_EXECUTION_ENV") == "AWS_Lambda_amplify-mock") {
		c.enabled = false
	}
	t := &Tracer{cfg: c, backend: c.backend}
	if c.enabled && t.backend == nil {
		backend, err := newDefaultOTelBackend(context.Background(), c.service)
		if err != nil {
			return nil, err
		}
		t.backend = backend
	}
	return t, nil
}

func (t *Tracer) Enabled() bool    { return t.cfg.enabled }
func (t *Tracer) Backend() Backend { return t.backend }
func (t *Tracer) report(err error) {
	if err != nil {
		t.cfg.onError(err)
	}
}

type spanKey struct{}
type activeSpan struct {
	tracer *Tracer
	span   Span
}

func (t *Tracer) current(ctx context.Context) Span {
	if value, ok := ctx.Value(spanKey{}).(activeSpan); ok && value.tracer == t {
		return value.span
	}
	return nil
}

// StartSpan starts an internal operation and returns an idempotent completion function.
// Pass the returned context to nested work; the caller's context remains unchanged.
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, func(error)) {
	return t.start(ctx, name, Internal)
}

func (t *Tracer) start(ctx context.Context, name string, kind Kind) (context.Context, func(error)) {
	if !t.cfg.enabled || t.backend == nil {
		return ctx, func(error) {}
	}
	next, span, err := t.backend.Start(ctx, name, kind)
	if err != nil {
		t.report(err)
		return ctx, func(error) {}
	}
	if span == nil {
		return ctx, func(error) {}
	}
	next = context.WithValue(next, spanKey{}, activeSpan{t, span})
	var once sync.Once
	return next, func(err error) {
		once.Do(func() {
			if err != nil {
				span.RecordError(err, t.cfg.errors)
			}
			span.End()
		})
	}
}

func (t *Tracer) PutAnnotation(ctx context.Context, key string, value any) error {
	if !t.cfg.enabled {
		return nil
	}
	if span := t.current(ctx); span != nil {
		return span.Annotation(key, value)
	}
	return nil
}

func (t *Tracer) PutMetadata(ctx context.Context, key string, value any, namespace ...string) error {
	if !t.cfg.enabled {
		return nil
	}
	name := t.cfg.service
	if len(namespace) > 0 {
		name = namespace[0]
	}
	if span := t.current(ctx); span != nil {
		return span.Metadata(name, key, value)
	}
	return nil
}

// AnnotateInvocation adds the shared cold-start identity and configured service.
// It does not create a new invocation or change the active span.
func (t *Tracer) AnnotateInvocation(ctx context.Context) {
	if info, ok := invocation.FromContext(ctx); ok {
		t.report(t.PutAnnotation(ctx, "ColdStart", info.ColdStart))
	}
	t.report(t.PutAnnotation(ctx, "Service", t.cfg.service))
}

// AddResponseAsMetadata respects the tracer's response-capture configuration.
func (t *Tracer) AddResponseAsMetadata(ctx context.Context, name string, value any) {
	if t.cfg.response {
		t.report(t.PutMetadata(ctx, name+" response", value))
	}
}

func (t *Tracer) TraceID(ctx context.Context) string {
	if span := t.current(ctx); span != nil {
		return span.TraceID()
	}
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		return span.TraceID().String()
	}
	return invocation.TraceID(ctx)
}

// XRayTraceID returns the current trace ID in the X-Ray display/wire format.
// It supports OTel contexts without requiring the retired X-Ray SDK adapter.
func (t *Tracer) XRayTraceID(ctx context.Context) string {
	id := t.TraceID(ctx)
	if formatted := commons.FormatXRayTraceID(id); formatted != "" {
		return formatted
	}
	return id
}

func (t *Tracer) IsTraceSampled(ctx context.Context) bool {
	if span := t.current(ctx); span != nil {
		return span.Sampled()
	}
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		return span.IsSampled()
	}
	return commons.IsRequestXRaySampled(ctx)
}

// HTTPClient copies the client and instruments its transport without global patching.
func (t *Tracer) HTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	copy := *client
	if copy.Transport == nil {
		copy.Transport = http.DefaultTransport
	}
	if t.cfg.enabled && t.cfg.http && t.backend != nil {
		copy.Transport = t.backend.HTTPTransport(copy.Transport)
	}
	return &copy
}

// InstrumentAWS modifies SDK v2 APIOptions during client construction only.
func (t *Tracer) InstrumentAWS(options *[]func(*middleware.Stack) error) {
	if t.cfg.enabled && t.backend != nil {
		t.backend.InstrumentAWS(options)
		*options = append(*options, awssdk.UserAgent("tracer"))
	}
}

func (t *Tracer) Flush(ctx context.Context) error {
	if !t.cfg.enabled || t.backend == nil {
		return nil
	}
	return t.backend.ForceFlush(ctx)
}

func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.backend == nil {
		return nil
	}
	return t.backend.Shutdown(ctx)
}

func (t *Tracer) flushInvocation(ctx context.Context) {
	limit := time.Now().Add(t.cfg.flushTimeout)
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(limit) {
		limit = deadline
	}
	flushCtx, cancel := context.WithDeadline(context.WithoutCancel(ctx), limit)
	defer cancel()
	t.report(t.Flush(flushCtx))
}

// HandlerOptions allows response suppression and custom event context extraction.
// Extraction must preserve the parent context's deadline and invocation metadata.
type HandlerOptions struct {
	Name                   string
	DisableCaptureResponse bool
	ExtractContext         func(context.Context, any) context.Context
}

// WrapHandler captures one handler operation, preserves business errors/panics,
// and attempts a bounded flush after ending the span, including error paths.
func WrapHandler[T, R any](t *Tracer, handler func(context.Context, T) (R, error), options ...HandlerOptions) func(context.Context, T) (R, error) {
	var opts HandlerOptions
	if len(options) > 0 {
		opts = options[0]
	}
	return func(ctx context.Context, event T) (result R, err error) {
		ctx = invocation.Ensure(ctx)
		if opts.ExtractContext != nil {
			ctx = opts.ExtractContext(ctx, event)
		}
		name := opts.Name
		if name == "" {
			name = os.Getenv("_HANDLER")
		}
		if name == "" {
			name = "bootstrap"
		}
		ctx, end := t.start(ctx, "## "+name, Handler)
		defer func() {
			failure := recover()
			if failure != nil {
				end(fmt.Errorf("handler panic: %v", failure))
			} else {
				if err == nil && !opts.DisableCaptureResponse {
					t.AddResponseAsMetadata(ctx, name, result)
				}
				end(err)
			}
			t.flushInvocation(ctx)
			if failure != nil {
				panic(failure)
			}
		}()
		t.AnnotateInvocation(ctx)
		return handler(ctx, event)
	}
}

// Capture wraps an arbitrary operation, preserving its typed result and error.
func Capture[R any](ctx context.Context, t *Tracer, name string, operation func(context.Context) (R, error)) (result R, err error) {
	ctx, end := t.StartSpan(ctx, "### "+name)
	defer func() {
		if failure := recover(); failure != nil {
			end(fmt.Errorf("operation panic: %v", failure))
			panic(failure)
		}
		if err == nil {
			t.AddResponseAsMetadata(ctx, name, result)
		}
		end(err)
	}()
	return operation(ctx)
}
