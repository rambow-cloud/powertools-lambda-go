package jmespath

import (
	"bytes"
	"compress/gzip"
	json "encoding/json/v2"
	"io"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// Built-in envelopes match all thirteen constants in TypeScript v2.35.0.
const (
	APIGatewayREST            = "powertools_json(body)"
	APIGatewayHTTP            = "powertools_json(body)"
	SQS                       = "Records[*].powertools_json(body)"
	SNS                       = "Records[0].Sns.Message | powertools_json(@)"
	EventBridge               = "detail"
	CloudWatchEventsScheduled = "detail"
	KinesisDataStream         = "Records[*].kinesis.powertools_json(powertools_base64(data))"
	CloudWatchLogs            = "awslogs.powertools_base64_gzip(data) | powertools_json(@).logEvents[*]"
	S3SNSSQS                  = "Records[*].powertools_json(body).powertools_json(Message).Records[0]"
	S3SQS                     = "Records[*].powertools_json(body).Records[0]"
	S3SNSKinesisFirehose      = "records[*].powertools_json(powertools_base64(data)).powertools_json(Message).Records[0]"
	S3KinesisFirehose         = "records[*].powertools_json(powertools_base64(data)).Records[0]"
	S3EventBridgeSQS          = "Records[*].powertools_json(body).detail"
)

// WithPowertoolsFunctions enables the reference JSON, Base64 and gzip functions.
// Gzip inputs are fully buffered, matching the reference; bound untrusted inputs
// at the application boundary. Base64 uses Commons validation before decoding.
func WithPowertoolsFunctions() Option {
	functions := []Function{}
	for _, name := range []string{"powertools_json", "powertools_base64", "powertools_base64_gzip"} {
		functions = append(functions, Function{Name: name, Arguments: []Argument{{Types: []Type{String}}}, Handler: func(args []any) (any, error) {
			value := args[0].(string)
			if name == "powertools_json" {
				var result any
				err := json.Unmarshal([]byte(value), &result)
				return result, err
			}
			decoded, err := commons.FromBase64(value, "base64")
			if err != nil {
				return nil, err
			}
			if name == "powertools_base64_gzip" {
				reader, err := gzip.NewReader(bytes.NewReader(decoded))
				if err != nil {
					return nil, err
				}
				defer reader.Close()
				decoded, err = io.ReadAll(reader)
				if err != nil {
					return nil, err
				}
			} else {
				// TextDecoder strips a leading UTF-8 BOM; Buffer.toString does not.
				decoded = bytes.TrimPrefix(decoded, []byte{0xef, 0xbb, 0xbf})
			}
			return commons.DecodeUTF8(decoded), nil
		}})
	}
	return WithFunctions(functions...)
}

// ExtractDataFromEnvelope enables Powertools functions when options are omitted.
// Explicit options replace that default, following the reference helper.
func ExtractDataFromEnvelope(data any, envelope string, options ...Option) (any, error) {
	if len(options) == 0 {
		options = []Option{WithPowertoolsFunctions()}
	}
	return Search(envelope, data, options...)
}
