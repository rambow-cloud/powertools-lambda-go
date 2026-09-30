package parser_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
)

func kafkaRecordText(value string) string {
	return fmt.Sprintf(`{"topic":"orders","partition":0,"offset":1,"timestamp":1,"timestampType":"CREATE_TIME","value":%q,"headers":[]}`, base64.StdEncoding.EncodeToString([]byte(value)))
}
func kafkaEventText(records string) json.RawMessage {
	return json.RawMessage(`{"eventSource":"aws:kafka","eventSourceArn":"arn:cluster","records":{` + records + `}}`)
}

func TestKafkaOrderingAndReuse(t *testing.T) {
	schema := envelopes.Kafka(parser.String())
	input := kafkaEventText(`"z":[` + kafkaRecordText("old") + `],"10":[` + kafkaRecordText("ten") + `],"2":[` + kafkaRecordText("two") + `],"a":[` + kafkaRecordText("a") + `],"z":[` + kafkaRecordText("last") + `],"01":[` + kafkaRecordText("zero-one") + `]`)
	before := append([]byte(nil), input...)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := parser.Parse(context.Background(), input, schema)
			if err != nil || !reflect.DeepEqual(got, []any{"two", "ten", "last", "a", "zero-one"}) {
				t.Errorf("ordered result=%v, error=%v", got, err)
			}
		}()
	}
	wg.Wait()
	if string(input) != string(before) {
		t.Fatal("input changed")
	}
	var native map[string]any
	if err := json.Unmarshal(input, &native); err != nil {
		t.Fatal(err)
	}
	got, err := parser.Parse(context.Background(), native, schema)
	if err != nil || !reflect.DeepEqual(got, []any{"two", "ten", "zero-one", "a", "last"}) {
		t.Fatalf("native map order=%v, error=%v", got, err)
	}
}

func TestKafkaFailureTraversal(t *testing.T) {
	input := kafkaEventText(`"z":[` + kafkaRecordText("bad") + `,` + kafkaRecordText("good") + `],"a":[` + kafkaRecordText("bad") + `]`)
	calls := 0
	schema := envelopes.Kafka(parser.SchemaFunc[string](func(_ context.Context, input any) (string, []parser.Issue, error) {
		calls++
		if input == "bad" {
			return "", []parser.Issue{{Code: "custom", Message: "bad", Path: []any{"id"}}}, nil
		}
		return input.(string), nil, nil
	}))
	got, err := parser.Parse(context.Background(), input, schema)
	var failure *parser.ParseError
	if !errors.As(err, &failure) || calls != 1 || got != nil || !reflect.DeepEqual(failure.Issues[0].Path, []any{"id"}) {
		t.Fatalf("ordinary result=%v calls=%d error=%v", got, calls, err)
	}
	calls = 0
	safe, err := parser.SafeParse(context.Background(), input, schema)
	if err != nil || safe.Success || safe.Data != nil || safe.Error == nil || calls != 3 {
		t.Fatalf("safe=%+v calls=%d error=%v", safe, calls, err)
	}
	if !reflect.DeepEqual(safe.OriginalEvent, input) || !reflect.DeepEqual(safe.Error.Issues[0].Path, []any{"records", "z", "id"}) || !reflect.DeepEqual(safe.Error.Issues[1].Path, []any{"records", "a", "id"}) {
		t.Fatalf("safe error=%+v", safe.Error)
	}
}

type kafkaSafePayload struct{}

func (kafkaSafePayload) Validate(context.Context, any) (string, []parser.Issue, error) {
	return "ordinary", nil, nil
}
func (kafkaSafePayload) ValidateSafe(context.Context, any) (string, []parser.Issue, error) {
	return "", []parser.Issue{}, nil
}

func TestKafkaOperationalContracts(t *testing.T) {
	input := kafkaEventText(`"orders-0":[` + kafkaRecordText("first") + `,` + kafkaRecordText("second") + `]`)
	safe, err := parser.SafeParse(context.Background(), input, envelopes.Kafka[string](kafkaSafePayload{}))
	if err != nil || safe.Success || safe.Error == nil || safe.Data != nil {
		t.Fatalf("empty validation failure lost: %+v, %v", safe, err)
	}
	sentinel := errors.New("callback failed")
	_, err = parser.SafeParse(context.Background(), input, envelopes.Kafka(parser.SchemaFunc[string](func(context.Context, any) (string, []parser.Issue, error) { return "", nil, sentinel })))
	if err != sentinel {
		t.Fatalf("callback identity lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	_, err = parser.Parse(ctx, input, envelopes.Kafka(parser.SchemaFunc[string](func(context.Context, any) (string, []parser.Issue, error) {
		calls++
		cancel()
		return "first", nil, nil
	})))
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancellation: calls=%d error=%v", calls, err)
	}
	t.Run("panic", func(t *testing.T) {
		defer func() {
			if recover() != sentinel {
				t.Error("panic identity lost")
			}
		}()
		_, _ = parser.Parse(context.Background(), input, envelopes.Kafka(parser.SchemaFunc[string](func(context.Context, any) (string, []parser.Issue, error) { panic(sentinel) })))
	})
	for _, bad := range []any{json.RawMessage(`{broken`), make(chan int)} {
		if _, err := parser.Parse(context.Background(), bad, envelopes.Kafka(parser.String())); err == nil {
			t.Fatalf("invalid JSON accepted: %T", bad)
		}
	}
	if _, err := parser.Parse(context.Background(), input, envelopes.Kafka[string](nil)); err == nil {
		t.Fatal("nil schema accepted")
	}
}
