package parser

import (
	"bytes"
	"compress/gzip"
	"context"
	json "encoding/json/v2"
	"io"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

func JSONStringified[T any](schema Schema[T]) Schema[T] {
	return Pipe(jsonText, schema)
}

var jsonText = Pipe(String(), SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) {
	var decoded any
	if err := json.Unmarshal([]byte(input.(string)), &decoded); err != nil {
		return nil, []Issue{{Code: "custom", Message: "Invalid JSON - " + err.Error()}}, nil
	}
	return decoded, nil, nil
}))

// Base64Encoded follows the reference's gzip-JSON, JSON, then UTF-8 text
// fallback. Malformed Base64 errors propagate rather than becoming schema issues.
func Base64Encoded[T any](schema Schema[T]) Schema[T] {
	return Pipe(base64Value, schema)
}

var base64Value = Pipe(String(), SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) {
	data, err := commons.FromBase64(input.(string), "base64")
	if err != nil {
		return nil, nil, err
	}
	var decoded any
	if reader, err := gzip.NewReader(bytes.NewReader(data)); err == nil {
		decompressed, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr == nil && closeErr == nil && json.Unmarshal([]byte(commons.DecodeUTF8(decompressed)), &decoded) == nil {
			return decoded, nil, nil
		}
	}
	// TextDecoder strips one leading BOM; gzip's Buffer.toString above retains it.
	text := strings.TrimPrefix(commons.DecodeUTF8(data), "\ufeff")
	if json.Unmarshal([]byte(text), &decoded) != nil {
		decoded = text
	}
	return decoded, nil, nil
}))
