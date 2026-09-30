package schemas

import (
	"context"
	"encoding/base64"
	"net/url"
	"regexp"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

func field(name string, schema parser.Schema[any]) parser.Field {
	return parser.Field{Name: name, Schema: schema}
}
func optional(name string, schema parser.Schema[any]) parser.Field {
	return parser.Field{Name: name, Schema: schema, Optional: true}
}
func nullish(name string, schema parser.Schema[any]) parser.Field {
	return optional(name, parser.Nullable(schema))
}
func array(schema parser.Schema[any], minimum ...int) parser.Schema[any] {
	return parser.Any(parser.Array(schema, minimum...))
}

var absoluteURL = parser.Pipe(parser.String(), parser.SchemaFunc[any](func(_ context.Context, input any) (any, []parser.Issue, error) {
	value := input.(string)
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() {
		return input, []parser.Issue{{Code: "invalid_format", Message: "Invalid URL", Continuable: true}}, nil
	}
	return value, nil, nil
}))
var positiveNumber = parser.Pipe(parser.Number(), parser.SchemaFunc[any](func(_ context.Context, input any) (any, []parser.Issue, error) {
	if input.(float64) <= 0 {
		return input, []parser.Issue{{Code: "too_small", Message: "Too small: expected number to be >0", Continuable: true}}, nil
	}
	return input, nil, nil
}))
var base64Pattern = regexp.MustCompile(`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`)
var base64String = parser.Pipe(parser.String(), parser.SchemaFunc[any](func(_ context.Context, input any) (any, []parser.Issue, error) {
	if !base64Pattern.MatchString(input.(string)) {
		return input, []parser.Issue{{Code: "invalid_format", Message: "Invalid base64-encoded string", Continuable: true}}, nil
	}
	return input, nil, nil
}))
var base64Text = parser.Transform(base64String, func(_ context.Context, input any) (any, error) {
	value, err := base64.StdEncoding.DecodeString(input.(string))
	return commons.DecodeUTF8(value), err
})

func windowFields(state parser.Schema[any]) []parser.Field {
	return []parser.Field{
		optional("window", parser.Object(field("start", isoDateTime), field("end", isoDateTime))),
		optional("state", parser.Dictionary(state)), optional("shardId", parser.String()), optional("eventSourceARN", parser.String()),
		optional("isFinalInvokeForWindow", parser.Boolean()), optional("isWindowTerminatedEarly", parser.Boolean()),
	}
}
