package schemas

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"io"
)

var CloudWatchLogEventSchema = parser.Object(field("id", parser.String()), field("timestamp", parser.Number()), field("message", parser.String()))
var CloudWatchLogsDecodeSchema = parser.Object(
	field("messageType", parser.String()), field("owner", parser.String()), field("logGroup", parser.String()), field("logStream", parser.String()),
	field("subscriptionFilters", array(parser.String())), field("logEvents", array(CloudWatchLogEventSchema, 1)),
)
var cloudwatchData = parser.Pipe(base64String, parser.SchemaFunc[any](func(ctx context.Context, input any) (any, []parser.Issue, error) {
	decoded, err := base64.StdEncoding.DecodeString(input.(string))
	if err == nil {
		reader, readErr := gzip.NewReader(bytes.NewReader(decoded))
		if readErr == nil {
			raw, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			var value any
			if readErr == nil && closeErr == nil && json.Unmarshal(raw, &value) == nil {
				parsed, issues, validationErr := CloudWatchLogsDecodeSchema.Validate(ctx, value)
				if validationErr != nil {
					return nil, nil, validationErr
				}
				if issues == nil {
					return parsed, nil, nil
				}
			}
		}
	}
	return nil, []parser.Issue{{Code: "custom", Message: "Failed to decompress CloudWatch log data"}}, nil
}))
var CloudWatchLogsSchema = parser.Object(field("awslogs", parser.Object(field("data", cloudwatchData))))
