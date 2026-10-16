package logger

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"math"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestCorrelationSnapshotSemantics(t *testing.T) {
	input := map[string]any{"id": map[string]any{"number": uint64(9007199254740993), "empty": []any(nil)}}
	got, err := ExtractCorrelationID(EventBridge, input)
	if err != nil {
		t.Fatal(err)
	}
	object := got.(map[string]any)
	if object["number"] != float64(9007199254740993) || object["empty"] == nil {
		t.Fatal(object)
	}
	object["number"] = false
	if input["id"].(map[string]any)["number"] != uint64(9007199254740993) {
		t.Fatal("returned caller-owned object")
	}
	for _, event := range []any{
		map[string]any{"id": "valid", "unused": math.Inf(1)},
		map[string]any{"id": "valid", "unused": make(chan int)},
		jsontext.Value(`{"id":"valid","unused":1e1000}`),
		jsontext.Value(`{"id":"valid","unused":{"x":1,"x":2}}`),
	} {
		if _, err := ExtractCorrelationID(EventBridge, event); err == nil {
			t.Fatalf("accepted invalid unrelated member: %T", event)
		}
	}
}

func TestBuiltInCorrelationSources(t *testing.T) {
	for _, source := range []CorrelationSource{APIGatewayREST, APIGatewayHTTP, AppSyncAuthorizer, AppSyncResolver, ApplicationLoadBalancer, EventBridge, LambdaFunctionURL, S3ObjectLambda, VPCLattice} {
		event := map[string]any{"requestContext": map[string]any{"requestId": "request"}, "request": map[string]any{"headers": map[string]any{"x-amzn-trace-id": "request"}}, "headers": map[string]any{"x-amzn-trace-id": "request"}, "id": "request", "xAmzRequestId": "request"}
		if got, err := ExtractCorrelationID(source, event); err != nil || got != "request" {
			t.Fatal(source, got, err)
		}
		if got, err := ExtractCorrelationID(source, map[string]any{}); err != nil || got != nil {
			t.Fatal(source, got, err)
		}
	}
	if _, err := ExtractCorrelationID("unknown", nil); err == nil {
		t.Fatal("unknown source accepted")
	}
}

func TestTypedCorrelationAndInvocationCleanup(t *testing.T) {
	cleanEnv(t)
	var out bytes.Buffer
	l := New(WithOutput(&out))
	h := WrapHandler(l, func(ctx context.Context, _ events.APIGatewayProxyRequest) (int, error) {
		return 0, l.WithContext(ctx).Info("request")
	}, HandlerOptions{CorrelationSource: APIGatewayREST})
	_, _ = h(context.Background(), events.APIGatewayProxyRequest{RequestContext: events.APIGatewayProxyRequestContext{RequestID: "first"}})
	_, _ = h(context.Background(), events.APIGatewayProxyRequest{})
	got := records(t, &out)
	if got[0]["correlation_id"] != "first" || got[1]["correlation_id"] == "first" {
		t.Fatal(got)
	}
}
