package loggerevent_test

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func TestRawEventCompiledCorrelation(t *testing.T) {
	var output bytes.Buffer
	log := logger.New(logger.WithOutput(&output), logger.WithLevel(logger.InfoLevel))
	enabled := true
	handler := logger.WrapRawHandler(log, func(ctx context.Context, input event) (string, error) {
		if log.WithContext(ctx).GetCorrelationID() != "request-1" {
			t.Fatal("raw-only member was unavailable to the compiled query")
		}
		return input.Name, nil
	}, logger.HandlerOptions{LogEvent: &enabled, CorrelationExtractor: jmespath.MustCompile("requestContext.requestId")})
	result, err := lambda.NewHandler(handler).Invoke(context.Background(), []byte(`{"name":"Alice","requestContext":{"requestId":"request-1"}}`))
	if err != nil || string(result) != `"Alice"` {
		t.Fatal(string(result), err)
	}
	var entry struct {
		CorrelationID string `json:"correlation_id"`
	}
	if err := jsonv1.Unmarshal(output.Bytes(), &entry); err != nil || entry.CorrelationID != "request-1" {
		t.Fatal(output.String(), err)
	}
}
