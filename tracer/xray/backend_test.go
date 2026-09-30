package xray

import (
	"context"
	"net"
	"sync"
	"testing"

	sdk "github.com/aws/aws-xray-sdk-go/v2/xray"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

type emitter struct {
	mu       sync.Mutex
	segments []*sdk.Segment
}

func (e *emitter) Emit(s *sdk.Segment) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.segments = append(e.segments, s)
}
func (e *emitter) RefreshEmitterWithAddress(*net.UDPAddr) {}
func TestLambdaSubsegmentAndMetadata(t *testing.T) {
	t.Setenv("POWERTOOLS_TRACE_ENABLED", "")
	t.Setenv("POWERTOOLS_DEV", "")
	t.Setenv("POWERTOOLS_TRACER_CAPTURE_RESPONSE", "")
	e := &emitter{}
	b := New(sdk.Config{Emitter: e})
	tr, err := tracer.New(tracer.WithBackend(b), tracer.WithLocalTracing(true), tracer.WithServiceName("orders"))
	if err != nil {
		t.Fatal(err)
	}
	header := "Root=1-12345678-123456789012345678901234;Parent=1234567890123456;Sampled=1"
	ctx := context.WithValue(context.Background(), "x-amzn-trace-id", header)
	var child *sdk.Segment
	h := tracer.WrapHandler(tr, func(ctx context.Context, n int) (int, error) {
		if tr.TraceID(ctx) != "1-12345678-123456789012345678901234" {
			t.Error(tr.TraceID(ctx))
		}
		return tracer.Capture(ctx, tr, "work", func(ctx context.Context) (int, error) {
			child = sdk.GetSegment(ctx)
			return n, tr.PutAnnotation(ctx, "Order", n)
		})
	}, tracer.HandlerOptions{Name: "handler"})
	if got, err := h(ctx, 3); got != 3 || err != nil {
		t.Fatal(got, err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.segments) != 1 {
		t.Fatalf("emitted %d segments", len(e.segments))
	}
	s := e.segments[0]
	if s.Name != "## handler" || s.ParentID != "1234567890123456" || s.Metadata["orders"]["handler response"] != float64(3) {
		t.Fatalf("unexpected segment: %+v", s)
	}
	// The default emitter packs private child storage into JSON Subsegments.
	// A test emitter receives the model before packing, so inspect the captured child.
	if child == nil || child.Name != "### work" || child.ParentSegment != s.ParentSegment || child.ID == s.ID || child.InProgress || child.EndTime == 0 {
		t.Fatalf("child lifecycle or parent mismatch: %+v", child)
	}
}
