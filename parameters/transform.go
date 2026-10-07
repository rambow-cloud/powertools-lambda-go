package parameters

import (
	json "encoding/json/v2"
	"fmt"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type Transform string

const (
	JSON   Transform = "json"
	Binary Transform = "binary"
	Auto   Transform = "auto"
)

type TransformParameterError struct {
	Transform Transform
	Err       error
}

func (e *TransformParameterError) Error() string {
	return fmt.Sprintf("unable to transform value using %q: %v", e.Transform, e.Err)
}
func (e *TransformParameterError) Unwrap() error { return e.Err }

// TransformValue decodes JSON or base64 text. Auto selects by the name suffix.
// Binary returns UTF-8 text; raw byte values stay bytes when no transform is used.
func TransformValue(name string, value any, transform Transform) (any, error) {
	if transform == "" {
		return value, nil
	}
	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case []byte:
		raw = strings.TrimPrefix(string(v), "\ufeff")
	default:
		return value, nil
	}
	mode := strings.ToLower(string(transform))
	if mode == string(Auto) {
		switch {
		case strings.HasSuffix(strings.ToLower(name), ".json"):
			mode = string(JSON)
		case strings.HasSuffix(strings.ToLower(name), ".binary"):
			mode = string(Binary)
		default:
			return value, nil
		}
	}
	var result any
	var err error
	switch mode {
	case string(JSON):
		err = json.Unmarshal([]byte(raw), &result)
	case string(Binary):
		var decoded []byte
		decoded, err = commons.FromBase64(raw, "base64")
		result = strings.TrimPrefix(commons.DecodeUTF8(decoded), "\ufeff")
	default:
		err = fmt.Errorf("unsupported transform %q", transform)
	}
	if err != nil {
		return nil, &TransformParameterError{Transform: transform, Err: err}
	}
	return result, nil
}
