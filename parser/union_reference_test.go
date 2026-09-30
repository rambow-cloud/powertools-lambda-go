package parser_test

import (
	"context"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

func TestUnionTypeScriptReference(t *testing.T) {
	runReference(t, "testdata/unions-v2.35.0.json", 1152)
}

var positiveUnionNumber = parser.Refine(parser.Number(), func(value any) bool { return value.(float64) > 0 }, "positive required")
var longUnionText = parser.Refine(parser.String(), func(value any) bool { return len(utf16.Encode([]rune(value.(string)))) > 3 }, "long text required")
var unionSchemas = map[string]parser.Schema[any]{
	"unionTypes":        parser.Union(parser.String(), parser.Number()),
	"unionSole":         parser.Union(longUnionText, parser.Number()),
	"unionTwo":          parser.Union(longUnionText, parser.Refine(parser.String(), func(value any) bool { return strings.HasPrefix(value.(string), "a") }, "prefix required")),
	"unionSingle":       parser.Union(positiveUnionNumber),
	"unionObjects":      parser.Union(parser.Object(parser.Field{Name: "kind", Schema: parser.Literal("a")}, parser.Field{Name: "value", Schema: positiveUnionNumber}), parser.Object(parser.Field{Name: "kind", Schema: parser.Literal("b")}, parser.Field{Name: "value", Schema: parser.String()})),
	"unionArrays":       parser.Union(parser.Any(parser.Array(parser.Number(), 2)), parser.String()),
	"unionNested":       parser.Object(parser.Field{Name: "payload", Schema: parser.Union(parser.Object(parser.Field{Name: "value", Schema: parser.Union(parser.String(), parser.Number())}), parser.Any(parser.Array(parser.Boolean())))}),
	"unionStrict":       parser.Union(parser.Object(parser.Field{Name: "id", Schema: parser.String()}).WithUnknownFields(parser.RejectUnknown), parser.Number()),
	"unionJSON":         parser.Union(parser.JSONStringified[any](parser.Object(parser.Field{Name: "id", Schema: parser.String()})), parser.Number()),
	"unionRefinements":  parser.Refine(positiveUnionNumber, func(value any) bool { return int(value.(float64))%2 == 0 }, "even required"),
	"unionTransform":    parser.Union(parser.Transform(parser.String(), func(_ context.Context, value any) (any, error) { return strings.ToUpper(value.(string)), nil }), parser.Number()),
	"unionArrayRefine":  parser.Refine(parser.Union(parser.Any(parser.Array(parser.Number(), 2)), parser.Boolean()), func(value any) bool { array, ok := value.([]any); return !ok || len(array) > 0 }, "nonempty required"),
	"unionObjectRefine": parser.Refine[any](parser.Object(parser.Field{Name: "value", Schema: positiveUnionNumber}), func(value any) bool { return value.(map[string]any)["value"] != float64(-3) }, "not minus three"),
	"unionNullable":     parser.Nullable(parser.Union(longUnionText, parser.Number())),
	"unionDictionary":   parser.Dictionary(parser.Union(parser.String(), parser.Number())),
	"unionInArray":      parser.Any(parser.Array(parser.Union(parser.String(), parser.Number()))),
}
