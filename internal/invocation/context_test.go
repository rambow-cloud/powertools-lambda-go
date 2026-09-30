package invocation

import (
	"context"
	"testing"
)

func TestRuntimeHeaderAndIdentity(t *testing.T) {
	t.Setenv("_X_AMZN_TRACE_ID", "Root=environment")
	t.Setenv("AWS_LAMBDA_MAX_CONCURRENCY", "10")
	if TraceHeader(context.Background()) != "" {
		t.Fatal("unsafe environment fallback")
	}
	ctx := context.WithValue(context.Background(), "x-amzn-trace-id", "Root=request;Sampled=0")
	if TraceID(ctx) != "request" {
		t.Fatal(TraceID(ctx))
	}
	ctx = Ensure(ctx)
	if Ensure(ctx) != ctx {
		t.Fatal("identity was replaced")
	}
	t.Setenv("AWS_LAMBDA_MAX_CONCURRENCY", "1")
	if TraceHeader(context.WithValue(ctx, "x-amzn-trace-id", "")) != "" {
		t.Fatal("empty runtime header must suppress fallback")
	}
}
