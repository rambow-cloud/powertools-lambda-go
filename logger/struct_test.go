package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type privateJSON struct{}

func (privateJSON) MarshalJSON() ([]byte, error) {
	return []byte(`{"password":"secret","large":9007199254740993}`), nil
}

func TestStructAndMarshalerRedaction(t *testing.T) {
	cleanEnv(t)
	for _, value := range []any{
		struct {
			Password string `json:"password"`
			Large    int64  `json:"large"`
			Omitted  string `json:"omitted,omitempty"`
		}{"secret", 9007199254740993, ""},
		privateJSON{}, json.RawMessage(`{"password":"secret","large":9007199254740993}`),
	} {
		var out bytes.Buffer
		calls := 0
		l := New(WithOutput(&out), WithReplacer(func(key string, value any) any {
			if key == "payload" {
				calls++
			}
			if key == "password" {
				return "[REDACTED]"
			}
			return value
		}))
		if err := l.Info("record", Fields{"payload": value}); err != nil {
			t.Fatal(err)
		}
		if calls != 1 || strings.Contains(out.String(), "secret") || strings.Contains(out.String(), "omitted") || !strings.Contains(out.String(), `"large":9007199254740993`) || !strings.Contains(out.String(), "[REDACTED]") {
			t.Fatal(out.String(), calls)
		}
	}
}
