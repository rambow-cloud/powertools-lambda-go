package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

func (c *Consumer) decode(ctx context.Context, input, metadata any, config *FieldConfig) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, absent := input.(Undefined); absent || input == nil {
		return input, nil
	}
	text, ok := input.(string)
	if array, isArray := input.([]any); isArray && len(array) == 0 {
		text, ok = "", true
	}
	if !ok {
		return nil, &TypeError{Message: "Incorrect padding on base64 string."}
	}
	var value any
	var err error
	switch {
	case config == nil || config.Type == JSON:
		value, err = primitive(text)
		if err == nil && config != nil {
			plain := value.(string)
			if parsed, parseErr := parseJSON([]byte(plain)); parseErr == nil {
				value = parsed
			} else {
				c.config.Diagnostic(ctx, "Failed to parse JSON from base64 value: "+text, parseErr)
			}
		}
	case config.Type == Avro || config.Type == Protobuf:
		if config.Schema == nil || config.Schema == "" {
			return nil, &MissingSchemaError{ConsumerError{Message: fmt.Sprintf("Schema string is required for %s deserialization", config.Type)}}
		}
		value, err = config.Decoder(ctx, text, config.Schema, metadata)
	default:
		// A present configuration with no type falls through deserialize upstream.
		value = Undefined{}
	}
	if err != nil {
		return nil, err
	}
	if config != nil && config.Parser != nil {
		parsed, err := config.Parser(ctx, value)
		if err != nil {
			return nil, err
		}
		if parsed.Issues != nil {
			return nil, &ParserError{ConsumerError: ConsumerError{Message: "Schema validation failed"}, Issues: parsed.Issues}
		}
		value = parsed.Value
	}
	return value, nil
}

func primitive(input string) (string, error) {
	length := 0
	for _, r := range input {
		length++
		if r > 0xffff {
			length++
		}
	}
	if length%4 != 0 {
		return "", &TypeError{Message: "Incorrect padding on base64 string."}
	}
	decoded, err := commons.FromBase64(input, "base64")
	if err != nil {
		return "", &TypeError{Message: "Invalid base64 string."}
	}
	// TextDecoder removes one leading BOM; Buffer.toString used for headers does not.
	return strings.TrimPrefix(commons.DecodeUTF8(decoded), "\uFEFF"), nil
}

func parseJSON(raw []byte) (any, error) {
	if !json.Valid(raw) {
		var out any
		err := json.Unmarshal(raw, &out)
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var result any
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	return jsonNumbers(result), nil
}

func jsonNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		// JavaScript numbers round to binary64 and may overflow to infinity.
		f, _ := strconv.ParseFloat(string(v), 64)
		return f
	case map[string]any:
		for key, item := range v {
			v[key] = jsonNumbers(item)
		}
	case []any:
		for i, item := range v {
			v[i] = jsonNumbers(item)
		}
	}
	return value
}

// Headers decodes byte arrays using Buffer-style byte coercion and UTF-8 replacement.
// Null is retained. Missing headers fail only when this method is called.
func (r *Record) Headers(ctx context.Context) ([]map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.headers == nil {
		return nil, nil
	}
	headers, ok := r.headers.([]any)
	if !ok {
		return nil, &TypeError{Message: "headers is not iterable"}
	}
	result := make([]map[string]string, 0, len(headers))
	for _, header := range headers {
		if header == nil {
			return nil, &TypeError{Message: "Cannot convert undefined or null to object"}
		}
		out := map[string]string{}
		for key, value := range properties(header) {
			var data []byte
			switch v := value.(type) {
			case string:
				data = []byte(v)
			case []byte:
				data = v
			case []any:
				data = make([]byte, len(v))
				for i, n := range v {
					data[i] = toByte(n)
				}
			default:
				return nil, &TypeError{Message: "Kafka header values must be byte arrays or strings"}
			}
			out[key] = commons.DecodeUTF8(data)
		}
		result = append(result, out)
	}
	return result, nil
}

func toByte(value any) byte {
	var number float64
	switch v := value.(type) {
	case float64:
		number = v
	case string:
		number = commons.ParseNumber(v)
	case bool:
		if v {
			number = 1
		}
	case nil:
		return 0
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return 0
		}
		if json.Unmarshal(encoded, &number) != nil {
			return 0
		}
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0
	}
	return byte(int64(math.Mod(math.Trunc(number), 256)))
}
