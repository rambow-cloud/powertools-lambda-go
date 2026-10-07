package tracer

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/smithy-go/middleware"
	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// OTelBackend integrates an application-owned OpenTelemetry provider.
// Metadata is represented as JSON attributes; see docs/COMPATIBILITY.md.
type OTelBackend struct {
	provider   trace.TracerProvider
	propagator propagation.TextMapPropagator
	owned      bool
}

// W3C trace context wins when both incoming formats are present and valid.
var defaultPropagator = propagation.NewCompositeTextMapPropagator(xray.Propagator{}, propagation.TraceContext{}, propagation.Baggage{})

// ExtractHTTPContext explicitly reads W3C and X-Ray propagation headers.
// Existing valid OTel context takes precedence; cancellation and deadlines survive.
// Use this in HandlerOptions.ExtractContext for HTTP-triggered Lambda events.
func ExtractHTTPContext(ctx context.Context, headers http.Header) context.Context {
	if trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return defaultPropagator.Extract(ctx, propagation.HeaderCarrier(headers))
}

// ExtractLambdaHTTPContext is an opt-in HandlerOptions.ExtractContext callback
// for API Gateway, Function URL, ALB and AppSync HTTP header envelopes. Existing
// context wins; multiValueHeaders overrides headers. Invalid/non-HTTP events are
// ignored. Applications choose whether incoming propagation headers are trusted.
func ExtractLambdaHTTPContext(ctx context.Context, event any) context.Context {
	if trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	data, err := json.Marshal(event)
	if err != nil {
		return ctx
	}
	type headerEnvelope struct {
		Headers           map[string]string   `json:"headers"`
		MultiValueHeaders map[string][]string `json:"multiValueHeaders"`
	}
	var envelope struct {
		headerEnvelope
		Request headerEnvelope `json:"request"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return ctx
	}
	headers := make(http.Header)
	for key, value := range envelope.Request.Headers {
		headers.Set(key, value)
	}
	for key, value := range envelope.Headers {
		headers.Set(key, value)
	}
	for key, values := range envelope.MultiValueHeaders {
		if len(values) > 0 {
			headers.Set(key, strings.Join(values, ","))
		}
	}
	return ExtractHTTPContext(ctx, headers)
}

// NewOTelBackend does not install globals or take ownership of the provider.
func NewOTelBackend(provider trace.TracerProvider) *OTelBackend {
	return &OTelBackend{provider: provider, propagator: defaultPropagator}
}

// WithPropagator configures propagation before the backend is used by handlers.
func (b *OTelBackend) WithPropagator(p propagation.TextMapPropagator) *OTelBackend {
	copy := *b
	copy.propagator = p
	return &copy
}

func newDefaultOTelBackend(ctx context.Context, service string) (*OTelBackend, error) {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	attrs := []attribute.KeyValue{attribute.String("service.name", service), attribute.String("cloud.provider", "aws"), attribute.String("cloud.platform", "aws_lambda")}
	for key, value := range map[string]string{"cloud.region": os.Getenv("AWS_REGION"), "faas.name": os.Getenv("AWS_LAMBDA_FUNCTION_NAME"), "faas.version": os.Getenv("AWS_LAMBDA_FUNCTION_VERSION")} {
		if value != "" {
			attrs = append(attrs, attribute.String(key, value))
		}
	}
	res, err := resource.New(ctx, resource.WithFromEnv(), resource.WithTelemetrySDK(), resource.WithAttributes(attrs...))
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, err
	}
	// The SDK honors OTEL_TRACES_SAMPLER and OTEL_TRACES_SAMPLER_ARG. Its default
	// remains parent-based always-on when those settings are absent.
	provider := sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithIDGenerator(xray.NewIDGenerator()), sdktrace.WithBatcher(exporter))
	b := NewOTelBackend(provider)
	b.owned = true
	return b, nil
}

func (b *OTelBackend) Start(ctx context.Context, name string, kind Kind) (context.Context, Span, error) {
	if b.provider == nil {
		return ctx, nil, fmt.Errorf("OpenTelemetry provider is nil")
	}
	if kind == Handler && !trace.SpanContextFromContext(ctx).IsValid() {
		if header := invocation.TraceHeader(ctx); header != "" {
			ctx = xray.Propagator{}.Extract(ctx, propagation.HeaderCarrier(http.Header{"X-Amzn-Trace-Id": []string{header}}))
		}
	}
	spanKind := trace.SpanKindInternal
	if kind == Handler {
		spanKind = trace.SpanKindServer
	}
	ctx, span := b.provider.Tracer("github.com/rambow-cloud/powertools-lambda-go/tracer").Start(ctx, name, trace.WithSpanKind(spanKind))
	if kind == Handler {
		if info, ok := invocation.FromContext(ctx); ok {
			span.SetAttributes(attribute.Bool("faas.coldstart", info.ColdStart))
		}
		if lc, ok := lambdacontext.FromContext(ctx); ok {
			span.SetAttributes(attribute.String("faas.invocation_id", lc.AwsRequestID), attribute.String("aws.lambda.invoked_arn", lc.InvokedFunctionArn))
		}
	}
	return ctx, &otelSpan{span: span, indexed: map[string]bool{}}, nil
}

func (b *OTelBackend) HTTPTransport(base http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(base, otelhttp.WithTracerProvider(b.provider), otelhttp.WithPropagators(b.propagator))
}

func (b *OTelBackend) InstrumentAWS(options *[]func(*middleware.Stack) error) {
	otelaws.AppendMiddlewares(options, otelaws.WithTracerProvider(b.provider), otelaws.WithTextMapPropagator(b.propagator))
}

func (b *OTelBackend) ForceFlush(ctx context.Context) error {
	if p, ok := b.provider.(interface{ ForceFlush(context.Context) error }); ok {
		return p.ForceFlush(ctx)
	}
	return nil
}

func (b *OTelBackend) Shutdown(ctx context.Context) error {
	if b.owned {
		if p, ok := b.provider.(interface{ Shutdown(context.Context) error }); ok {
			return p.Shutdown(ctx)
		}
	}
	return nil
}

type otelSpan struct {
	span    trace.Span
	mu      sync.Mutex
	indexed map[string]bool
}

func (s *otelSpan) Annotation(key string, value any) error {
	var attr attribute.KeyValue
	switch v := value.(type) {
	case string:
		attr = attribute.String(key, v)
	case bool:
		attr = attribute.Bool(key, v)
	case int:
		attr = attribute.Int(key, v)
	case int64:
		attr = attribute.Int64(key, v)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("annotation %q must be finite", key)
		}
		attr = attribute.Float64(key, v)
	default:
		return fmt.Errorf("annotation %q requires a string, bool, int, int64, or float64", key)
	}
	if key == "" {
		return fmt.Errorf("annotation key must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.indexed[key] = true
	keys := make([]string, 0, len(s.indexed))
	for key := range s.indexed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	s.span.SetAttributes(attr, attribute.StringSlice("aws.xray.annotations", keys))
	return nil
}

func (s *otelSpan) Metadata(namespace, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.span.SetAttributes(attribute.String(metadataAttributeKey(namespace, key), string(data)))
	return nil
}

// Escape each component independently so dotted names cannot overwrite another pair.
func metadataAttributeKey(namespace, key string) string {
	escape := strings.NewReplacer("%", "%25", ".", "%2E")
	return "powertools.metadata." + escape.Replace(namespace) + "." + escape.Replace(key)
}

func (s *otelSpan) RecordError(err error, capture bool) {
	if capture {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	} else {
		s.span.SetStatus(codes.Error, "operation failed")
	}
}
func (s *otelSpan) End()            { s.span.End() }
func (s *otelSpan) TraceID() string { return s.span.SpanContext().TraceID().String() }
func (s *otelSpan) Sampled() bool   { return s.span.SpanContext().IsSampled() }
