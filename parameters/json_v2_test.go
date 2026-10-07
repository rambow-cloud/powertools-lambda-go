package parameters

import (
	"encoding/json/jsontext"
	"errors"
	"testing"
)

func TestJSONTransformRejectsAmbiguousAndInvalidText(t *testing.T) {
	for name, input := range map[string]string{
		"duplicate": `{"config":{"enabled":true,"enabled":false}}`,
		"surrogate": `{"value":"\ud800"}`,
		"utf8":      "{\"value\":\"" + string([]byte{0xff}) + "\"}",
	} {
		for _, bytes := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/string", true: "/bytes"}[bytes], func(t *testing.T) {
				var value any = input
				if bytes {
					value = []byte(input)
				}
				got, err := TransformValue("config.json", value, Auto)
				var transform *TransformParameterError
				var syntax *jsontext.SyntacticError
				if got != nil || !errors.As(err, &transform) || !errors.As(err, &syntax) {
					t.Fatalf("invalid configuration accepted or cause lost: value=%v error=%v", got, err)
				}
			})
		}
	}
}
