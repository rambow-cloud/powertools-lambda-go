package main

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
)

func parserErrorsProbe(ctx context.Context, record events.SQSMessage) (map[string]any, error) {
	result := map[string]any{}
	positive := parser.Refine(parser.Number(), func(value any) bool { return value.(float64) > 0 }, "positive required")
	tree := parser.Object(parser.Field{Name: "payload", Schema: parser.Union(parser.Object(parser.Field{Name: "value", Schema: parser.Union(parser.String(), parser.Number())}), parser.Any(parser.Array(parser.Boolean())))})
	first, second := record, record
	first.Body = `{"id":1,"amount":-1}`
	second.Body = `{"id":"b","amount":-1}`
	encoded, err := json.Marshal(events.SQSEvent{Records: []events.SQSMessage{first, second}})
	if err != nil {
		return nil, err
	}
	nested := parser.Object(parser.Field{Name: "event", Schema: parser.JSONStringified(parser.Nullable(parser.Any(envelopes.SQS(parser.JSONStringified(orderInputSchema)))))}, parser.Field{Name: "label", Schema: parser.String()})
	input := map[string]any{"event": string(encoded), "label": 0}
	cases := []struct {
		name   string
		input  any
		schema parser.Schema[any]
	}{
		{"sole", -3, parser.Union(positive, parser.String())},
		{"tree", map[string]any{"payload": map[string]any{"value": false}}, tree},
		{"refinements", -3, parser.Refine(positive, func(value any) bool { return int(value.(float64))%2 == 0 }, "even required")},
		{"array", []any{false}, parser.Any(parser.Array(parser.Number(), 2))},
		{"nested_safe", input, nested},
	}
	for _, item := range cases {
		parsed, err := parser.SafeParse(ctx, item.input, item.schema)
		if err != nil {
			return nil, err
		}
		if parsed.Success || parsed.Error == nil {
			return nil, fmt.Errorf("%s invalid fixture passed", item.name)
		}
		result[item.name] = parsed.Error.Issues
	}
	_, err = parser.Parse(ctx, input, nested)
	var failure *parser.ParseError
	if !errors.As(err, &failure) {
		return nil, errors.New("nested ordinary fixture passed")
	}
	result["nested_ordinary"] = failure.Issues
	return result, nil
}
