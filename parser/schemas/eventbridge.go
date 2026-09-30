package schemas

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

var utcDateTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?Z$`)

var isoDateTime = parser.SchemaFunc[any](func(ctx context.Context, input any) (any, []parser.Issue, error) {
	value, issues, err := parser.String().Validate(ctx, input)
	if issues != nil || err != nil {
		return value, issues, err
	}
	text := value.(string)
	normalized := text
	if len(text) == 17 {
		normalized = strings.TrimSuffix(text, "Z") + ":00Z"
	}
	_, parseErr := time.Parse(time.RFC3339Nano, normalized)
	if !utcDateTime.MatchString(text) || parseErr != nil {
		return input, []parser.Issue{{Code: "invalid_format", Message: "Invalid ISO datetime", Continuable: true}}, nil
	}
	return text, nil, nil
})

var EventBridgeSchema = parser.Object(
	parser.Field{Name: "version", Schema: parser.String()},
	parser.Field{Name: "id", Schema: parser.String()},
	parser.Field{Name: "source", Schema: parser.String()},
	parser.Field{Name: "account", Schema: parser.String()},
	parser.Field{Name: "time", Schema: isoDateTime},
	parser.Field{Name: "region", Schema: parser.String()},
	parser.Field{Name: "resources", Schema: parser.Any(parser.Array(parser.String()))},
	parser.Field{Name: "detail-type", Schema: parser.String()},
	parser.Field{Name: "detail", Schema: parser.Unknown()},
	parser.Field{Name: "replay-name", Schema: parser.String(), Optional: true},
)
