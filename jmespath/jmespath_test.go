package jmespath

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestTypeScriptReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name, Expression, Envelope, Error string
			Data, Expected                    any
			Powertools                        bool
			ExpectedUndefined                 bool
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	envelopes := map[string]string{"API_GATEWAY_REST": APIGatewayREST, "API_GATEWAY_HTTP": APIGatewayHTTP, "SQS": SQS, "SNS": SNS, "EVENTBRIDGE": EventBridge, "CLOUDWATCH_EVENTS_SCHEDULED": CloudWatchEventsScheduled, "KINESIS_DATA_STREAM": KinesisDataStream, "CLOUDWATCH_LOGS": CloudWatchLogs, "S3_SNS_SQS": S3SNSSQS, "S3_SQS": S3SQS, "S3_SNS_KINESIS_FIREHOSE": S3SNSKinesisFirehose, "S3_KINESIS_FIREHOSE": S3KinesisFirehose, "S3_EVENTBRIDGE_SQS": S3EventBridgeSQS}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			options := []Option{}
			if item.Powertools {
				options = append(options, WithPowertoolsFunctions())
			}
			if item.Envelope != "" && envelopes[item.Envelope] != item.Expression {
				t.Fatal("envelope differs from reference")
			}
			got, err := Search(item.Expression, item.Data, options...)
			if item.ExpectedUndefined {
				// The pinned interpreter silently swallows decoder failures. Go exposes them.
				var failure *Error
				if !errors.As(err, &failure) || failure.Kind != InvalidValue {
					t.Fatal("decoder error was swallowed", err)
				}
				return
			}
			if item.Error != "" {
				var failure *Error
				if !errors.As(err, &failure) {
					t.Fatalf("expected %s, got %v", item.Error, err)
				}
				kind := InvalidValue
				switch item.Error {
				case "EmptyExpressionError":
					kind = EmptyExpression
				case "IncompleteExpressionError", "LexerError", "ParseError":
					kind = SyntaxError
				case "UnknownFunctionError":
					kind = UnknownFunction
				case "ArityError", "VariadicArityError":
					kind = InvalidArity
				case "JMESPathTypeError":
					kind = InvalidType
				}
				if strings.TrimSpace(item.Expression) == "" {
					kind = EmptyExpression
				}
				if failure.Kind != kind || failure.Expression != item.Expression {
					t.Fatal(failure)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, item.Expected) {
				t.Fatalf("got %#v (%v), want %#v", got, err, item.Expected)
			}
		})
	}
}

func TestCustomFunctionsAndErrors(t *testing.T) {
	want := errors.New("callback failed")
	option := WithFunctions(
		Function{Name: "greet", Arguments: []Argument{{Types: []Type{String}}}, Handler: func(args []any) (any, error) { return "hello " + args[0].(string), nil }},
		Function{Name: "zero", Handler: func([]any) (any, error) { return "zero", nil }},
		Function{Name: "fail", Handler: func([]any) (any, error) { return nil, want }},
		Function{Name: "booleans", Arguments: []Argument{{Types: []Type{Boolean, Null}, Variadic: true}}, Handler: func(args []any) (any, error) { return float64(len(args)), nil }},
	)
	for _, item := range []struct {
		source   string
		expected any
	}{{"greet(name)", "hello Go"}, {"zero()", "zero"}, {"booleans(`true`,`null`)", float64(2)}} {
		result, err := Search(item.source, struct {
			Name string `json:"name"`
		}{"Go"}, option)
		if err != nil || result != item.expected {
			t.Fatal(result, err)
		}
	}
	for source, kind := range map[string]ErrorKind{"zero(`1`)": InvalidArity, "greet()": InvalidArity, "greet(`1`)": InvalidType, "booleans(`true`,`42`)": InvalidType, "booleans()": InvalidArity, "unknown()": UnknownFunction} {
		_, err := Search(source, nil, option)
		var failure *Error
		if !errors.As(err, &failure) || failure.Kind != kind {
			t.Fatal(source, err)
		}
	}
	if _, err := Search("fail()", nil, option); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if _, err := Compile("@", WithFunctions(Function{Name: "bad"})); err == nil {
		t.Fatal("nil handler accepted")
	}
	if _, err := Search("@", make(chan int)); err == nil {
		t.Fatal("non-JSON input accepted")
	}
}

func TestCompiledConcurrencyAndIsolation(t *testing.T) {
	expression := MustCompile("sort_by(items, &n)[*].n")
	data := map[string]any{"items": []any{map[string]any{"n": 2}, map[string]any{"n": 1}}}
	before, _ := json.Marshal(data)
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			got, err := expression.Search(data)
			if err != nil || !reflect.DeepEqual(got, []any{float64(1), float64(2)}) {
				t.Error(got, err)
			}
			PurgeCache()
		})
	}
	wg.Wait()
	after, _ := json.Marshal(data)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
	if _, err := ExtractDataFromEnvelope(map[string]any{"body": "{}"}, APIGatewayREST); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractDataFromEnvelope(map[string]any{"body": "{}"}, APIGatewayREST, WithFunctions()); err == nil {
		t.Fatal("explicit options did not replace envelope defaults")
	}
	// Function definitions are snapshotted before compilation.
	args := []Argument{{Types: []Type{String}}}
	option := WithFunctions(Function{Name: "upper", Arguments: args, Handler: func(args []any) (any, error) { return strings.ToUpper(args[0].(string)), nil }})
	args[0].Types[0] = Number
	if got, err := Search("upper(@)", "ok", option); err != nil || got != "OK" {
		t.Fatal(got, err)
	}
}
