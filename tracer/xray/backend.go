// Package xray provides the optional AWS X-Ray SDK compatibility backend.
// Importing the main tracer package does not initialize this SDK.
//
// Deprecated: This adapter is frozen. Use tracer's OpenTelemetry backend with an
// OTLP collector and the awsxray exporter to continue sending traces to X-Ray.
package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/aws/aws-xray-sdk-go/v2/instrumentation/awsv2"
	sdk "github.com/aws/aws-xray-sdk-go/v2/xray"
	"github.com/aws/smithy-go/middleware"
	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

// Backend retains the original SDK integration for existing applications.
// Deprecated: Use tracer.OTelBackend and an OTLP collector with the awsxray exporter.
type Backend struct{ config sdk.Config }

// DeprecationWarning describes the maintained replacement for this frozen adapter.
const DeprecationWarning = "WARNING: powertools tracer/xray is deprecated and frozen. AWS X-Ray SDKs entered maintenance mode on February 25, 2026 (security updates only). Use the OpenTelemetry tracer with an OTLP collector and awsxray exporter. https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-migration.html"

var warnOnce sync.Once

// New configures each trace through context, without changing SDK globals.
// Deprecated: Use tracer.New or tracer.NewOTelBackend. New emits a migration warning
// to stderr once per process, only when the legacy adapter is explicitly constructed.
func New(config sdk.Config) *Backend {
	warnOnce.Do(func() { _, _ = fmt.Fprintln(os.Stderr, DeprecationWarning) })
	return &Backend{config: config}
}

func (b *Backend) Start(ctx context.Context, name string, kind tracer.Kind) (context.Context, tracer.Span, error) {
	var err error
	ctx, err = sdk.ContextWithConfig(ctx, b.config)
	if err != nil {
		return ctx, nil, err
	}
	if sdk.GetSegment(ctx) == nil {
		header := invocation.TraceHeader(ctx)
		if header == "" {
			return ctx, nil, nil
		}
		// This string key is the public integration contract of the AWS SDK.
		ctx = context.WithValue(ctx, sdk.LambdaTraceHeaderKey, header)
	}
	ctx, segment := sdk.BeginSubsegment(ctx, name)
	if segment == nil {
		return ctx, nil, nil
	}
	return ctx, &span{segment}, nil
}

type transport struct{ base, traced http.RoundTripper }

func (t transport) RoundTrip(request *http.Request) (*http.Response, error) {
	if sdk.GetSegment(request.Context()) == nil {
		return t.base.RoundTrip(request)
	}
	// The SDK adds headers; clone to preserve the caller's request object.
	return t.traced.RoundTrip(request.Clone(request.Context()))
}

func (b *Backend) HTTPTransport(base http.RoundTripper) http.RoundTripper {
	return transport{base, sdk.RoundTripper(base)}
}
func (b *Backend) InstrumentAWS(options *[]func(*middleware.Stack) error) {
	awsv2.AWSV2Instrumentor(options)
}
func (b *Backend) ForceFlush(context.Context) error { return nil }
func (b *Backend) Shutdown(context.Context) error   { return nil }

type span struct{ segment *sdk.Segment }

func (s *span) Annotation(key string, value any) error { return s.segment.AddAnnotation(key, value) }
func (s *span) Metadata(namespace, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("serialize trace metadata: %w", err)
	}
	var copy any
	if err = json.Unmarshal(data, &copy); err != nil {
		return err
	}
	return s.segment.AddMetadataToNamespace(namespace, key, copy)
}
func (s *span) RecordError(err error, capture bool) {
	if capture {
		_ = s.segment.AddError(err)
	} else {
		s.segment.Lock()
		s.segment.Error = true
		s.segment.Unlock()
	}
}
func (s *span) End() { s.segment.Close(nil) }
func (s *span) TraceID() string {
	s.segment.RLock()
	defer s.segment.RUnlock()
	return s.segment.TraceID
}
func (s *span) Sampled() bool { s.segment.RLock(); defer s.segment.RUnlock(); return s.segment.Sampled }
