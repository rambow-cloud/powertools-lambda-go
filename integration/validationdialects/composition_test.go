package validationdialects_test

import (
	"context"
	jsonv1 "encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

type request struct {
	Names []string `json:"names"`
}

func TestDraft202012HandlerParserComposition(t *testing.T) {
	testDialectComposition(t, validation.Draft202012Compiler{},
		jsonv1.RawMessage(`{"type":"object","required":["names"],"properties":{"names":{"type":"array","prefixItems":[{"type":"string"}],"items":false}}}`),
		jsonv1.RawMessage(`{"type":"array","prefixItems":[{"type":"string"}],"items":false}`))
}

func TestDraft201909HandlerParserComposition(t *testing.T) {
	testDialectComposition(t, validation.Draft201909Compiler{},
		jsonv1.RawMessage(`{"type":"object","required":["names"],"properties":{"names":{"type":"array","items":[{"type":"string"}],"additionalItems":false}}}`),
		jsonv1.RawMessage(`{"type":"array","items":[{"type":"string"}],"additionalItems":false}`))
}

func testDialectComposition(t *testing.T, compiler validation.Compiler, inputSchema, outputSchema jsonv1.RawMessage) {
	t.Helper()
	ctx := context.Background()
	inbound, err := validation.Compile(ctx, inputSchema, validation.Options{Compiler: compiler})
	if err != nil {
		t.Fatal(err)
	}
	outbound, err := validation.Compile(ctx, outputSchema, validation.Options{Compiler: compiler})
	if err != nil {
		t.Fatal(err)
	}
	for _, parse := range []bool{false, true} {
		t.Run(map[bool]string{false: "handler", true: "parser-handler"}[parse], func(t *testing.T) {
			calls := 0
			business := func(_ context.Context, input request) ([]string, error) { calls++; return input.Names, nil }
			var handler any
			if parse {
				schema := parser.Typed[request](parser.Object(parser.Field{Name: "names", Schema: parser.Any(parser.Array(parser.String()))}))
				handler = parser.WrapHandler[jsonv1.RawMessage](schema, validation.WrapHandler[request](inbound, outbound, business))
			} else {
				handler = validation.WrapHandler[jsonv1.RawMessage](inbound, outbound, business)
			}
			entry := lambda.NewHandler(handler)
			result, err := entry.Invoke(ctx, []byte(`{"names":["Ada"]}`))
			if err != nil || string(result) != `["Ada"]` || calls != 1 {
				t.Fatal(string(result), err, calls)
			}
			result, err = entry.Invoke(ctx, []byte(`{"names":["Ada","extra"]}`))
			var failure *validation.SchemaValidationError
			if !errors.As(err, &failure) || !strings.HasPrefix(failure.Error(), "Inbound") || len(result) != 0 || calls != 1 {
				t.Fatal(string(result), err, calls)
			}
		})
	}
	envelope, err := validation.Compile(ctx, jsonv1.RawMessage(`{"type":"object","dependentRequired":{"name":["id"]}}`), validation.Options{Compiler: compiler, Envelope: "payload"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		input jsonv1.RawMessage
		valid bool
	}{
		{jsonv1.RawMessage(`{"payload":{"name":"Ada","id":1}}`), true},
		{jsonv1.RawMessage(`{"payload":{"name":"Ada"}}`), false},
	} {
		_, err := envelope.Validate(ctx, item.input)
		if item.valid != (err == nil) {
			t.Fatal(string(item.input), err)
		}
	}
	outboundFailure := validation.WrapHandler[request](nil, outbound, func(context.Context, request) ([]string, error) { return []string{"Ada", "extra"}, nil })
	_, err = outboundFailure(ctx, request{})
	var failure *validation.SchemaValidationError
	if !errors.As(err, &failure) || !strings.HasPrefix(failure.Error(), "Outbound") {
		t.Fatal("outbound modern assertions were skipped", err)
	}
}
