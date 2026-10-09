package loggerevent_test

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

type event struct {
	Name string `json:"name"`
}

func TestLoggerBeforeAndAfterProjection(t *testing.T) {
	for _, utility := range []string{"parser", "validation"} {
		for _, outside := range []bool{true, false} {
			t.Run(utility+map[bool]string{true: "/outside", false: "/inside"}[outside], func(t *testing.T) {
				for _, name := range []string{"POWERTOOLS_DEV", "POWERTOOLS_LOG_LEVEL", "LOG_LEVEL", "AWS_LAMBDA_LOG_LEVEL", "POWERTOOLS_LOGGER_LOG_EVENT", "POWERTOOLS_LOGGER_SAMPLE_RATE"} {
					t.Setenv(name, "")
				}
				var output bytes.Buffer
				log := logger.New(logger.WithOutput(&output), logger.WithLevel(logger.InfoLevel))
				enabled := true
				options := logger.HandlerOptions{LogEvent: &enabled}
				var retained *logger.Logger
				calls := 0
				business := func(ctx context.Context, input event) (string, error) {
					calls++
					retained = log.WithContext(ctx)
					return input.Name, nil
				}
				if !outside {
					business = logger.WrapHandler(log, business, options)
				}
				var pipeline func(context.Context, jsonv1.RawMessage) (string, error)
				if utility == "parser" {
					schema := parser.Typed[event](parser.Object(parser.Field{Name: "name", Schema: parser.String()}))
					pipeline = parser.WrapHandler[jsonv1.RawMessage](schema, business)
				} else {
					schema, err := validation.Compile(context.Background(), jsonv1.RawMessage(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`), validation.Options{})
					if err != nil {
						t.Fatal(err)
					}
					pipeline = validation.WrapHandler[jsonv1.RawMessage](schema, nil, business)
				}
				if outside {
					pipeline = logger.WrapRawHandler(log, pipeline, options)
				}
				result, err := lambda.NewHandler(pipeline).Invoke(context.Background(), []byte(`{"name":"Alice","age":30}`))
				if err != nil || string(result) != `"Alice"` || calls != 1 {
					t.Fatal(string(result), err, calls)
				}
				var entry struct {
					Event map[string]any `json:"event"`
				}
				if err := jsonv1.Unmarshal(output.Bytes(), &entry); err != nil {
					t.Fatal(err)
				}
				age, present := entry.Event["age"]
				if entry.Event["name"] != "Alice" || present != outside || (outside && age != float64(30)) {
					t.Fatal("wrapper order changed event contents", entry.Event)
				}
				if !errors.Is(retained.Info("late"), logger.ErrInvocationClosed) {
					t.Fatal("composition left invocation open")
				}
			})
		}
	}
}
