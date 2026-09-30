package parser_test

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

type modeSchema struct{}

func (modeSchema) Validate(context.Context, any) (any, []parser.Issue, error) {
	return nil, []parser.Issue{{Message: "first"}}, nil
}
func (modeSchema) ValidateSafe(context.Context, any) (any, []parser.Issue, error) {
	return nil, []parser.Issue{{Message: "first"}, {Message: "second"}}, nil
}

func TestPipelinePreservesModesAndFailures(t *testing.T) {
	ctx := context.Background()
	calls := 0
	refined := parser.Refine[any](modeSchema{}, func(any) bool { calls++; return true }, "refinement")
	piped := parser.Pipe(refined, parser.SchemaFunc[string](func(context.Context, any) (string, []parser.Issue, error) { calls++; return "converted", nil, nil }))
	safe, err := parser.SafeParse(ctx, 1, piped)
	if err != nil || safe.Error == nil || len(safe.Error.Issues) != 2 || calls != 0 {
		t.Fatalf("safe mode lost: %+v %v calls=%d", safe, err, calls)
	}
	_, err = parser.Parse(ctx, 1, piped)
	var failure *parser.ParseError
	if !errors.As(err, &failure) || len(failure.Issues) != 1 {
		t.Fatalf("ordinary mode changed: %v", err)
	}
	value, err := parser.Parse(ctx, nil, parser.Refine(parser.Unknown(), func(value any) bool { return value == nil }, "must be null"))
	if err != nil || value != nil {
		t.Fatalf("nil output: %v/%v", value, err)
	}
	if _, err = parser.Parse(ctx, 1, parser.Pipe[any, any](nil, parser.Unknown())); err == nil {
		t.Fatal("nil pipe accepted")
	}
}

func TestDynamoDBHelperPrecisionAndExtension(t *testing.T) {
	ctx := context.Background()
	input := map[string]any{"large": map[string]any{"N": "9007199254740993"}, "id": map[string]any{"S": "a"}}
	parsed, err := parser.Parse(ctx, input, parser.DynamoDBMarshalled(parser.Unknown()))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.(map[string]any)["large"].(*big.Int).String() != "9007199254740993" {
		t.Fatal("large integer precision lost")
	}
	schema := schemas.DynamoDBStreamChangeRecordBase.Extend(parser.Field{Name: "NewImage", Schema: parser.DynamoDBMarshalled(parser.Unknown()), Optional: true}).Omit("SequenceNumber", "StreamViewType")
	source := map[string]any{"Keys": map[string]any{"id": map[string]any{"S": "a"}}, "NewImage": input, "SizeBytes": 1}
	result, err := parser.Parse(ctx, source, schema)
	if err != nil {
		t.Fatal(err)
	}
	mapped := result.(map[string]any)
	if _, ok := mapped["Keys"].(map[string]any)["id"].(map[string]any); !ok {
		t.Fatal("base schema unexpectedly decoded keys")
	}
	if mapped["NewImage"].(map[string]any)["id"] != "a" {
		t.Fatal("extended image was not decoded")
	}
	if !reflect.DeepEqual(source["NewImage"], input) {
		t.Fatal("source image mutated")
	}
	if _, err := parser.Parse(ctx, map[string]any{}, schemas.DynamoDBStreamChangeRecordBase); err == nil {
		t.Fatal("omission mutated the base schema")
	}
}

func TestConcurrentStreamEnvelopeAndCancellation(t *testing.T) {
	notification := map[string]any{"TopicArn": "arn:topic", "UnsubscribeUrl": "https://example.test/unsubscribe", "Type": "Notification", "Message": "payload", "MessageId": "1", "Timestamp": "2026-09-15T00:00:00Z"}
	event := map[string]any{"Records": []any{map[string]any{"EventSource": "aws:sns", "EventVersion": "1", "EventSubscriptionArn": "arn:subscription", "Sns": notification}}}
	envelope := envelopes.SNS(parser.String())
	var workers sync.WaitGroup
	for range 100 {
		workers.Go(func() {
			result, err := parser.Parse(context.Background(), event, envelope)
			if err != nil || !reflect.DeepEqual(result, []any{"payload"}) {
				t.Errorf("result=%v err=%v", result, err)
			}
		})
	}
	workers.Wait()
	sentinel := errors.New("callback failed")
	failing := envelopes.SNS(parser.SchemaFunc[string](func(context.Context, any) (string, []parser.Issue, error) { return "", nil, sentinel }))
	if _, err := parser.SafeParse(context.Background(), event, failing); err != sentinel {
		t.Fatalf("callback error changed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := parser.Parse(ctx, event, envelope); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
