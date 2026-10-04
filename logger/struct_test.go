package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type emptyStringJSON string

func (emptyStringJSON) MarshalJSON() ([]byte, error) { return []byte(`"custom value"`), nil }

type failingEmptyStringJSON string

var emptyStringJSONError = errors.New("custom string marshaling failed")

func (failingEmptyStringJSON) MarshalJSON() ([]byte, error) { return nil, emptyStringJSONError }

type emptyStringText string

func (emptyStringText) MarshalText() ([]byte, error) { return []byte("custom text"), nil }

type failingEmptyStringText string

var emptyStringTextError = errors.New("custom text marshaling failed")

func (failingEmptyStringText) MarshalText() ([]byte, error) { return nil, emptyStringTextError }

type emptyTextOutput string

func (emptyTextOutput) MarshalText() ([]byte, error) { return []byte{}, nil }

type dualEmptyString string

func (dualEmptyString) MarshalJSON() ([]byte, error) { return []byte(`"json value"`), nil }
func (dualEmptyString) MarshalText() ([]byte, error) { return nil, emptyStringTextError }

func TestEmptyStringAttributeMarshalers(t *testing.T) {
	cleanEnv(t)
	for _, tc := range []struct {
		name        string
		value       any
		wantPayload string
		wantErr     error
	}{
		{"value", emptyStringJSON(""), "custom value", nil},
		{"error", failingEmptyStringJSON(""), "", emptyStringJSONError},
		{"text-value", emptyStringText(""), "custom text", nil},
		{"text-error", failingEmptyStringText(""), "", emptyStringTextError},
		{"text-empty", emptyTextOutput(""), "", nil},
		{"dual", dualEmptyString(""), "json value", nil},
	} {
		for _, source := range []string{"extra", "persistent", "temporary", "formatter"} {
			for _, replace := range []bool{false, true} {
				name := tc.name + "/" + source
				if replace {
					name += "/replacer"
				}
				t.Run(name, func(t *testing.T) {
					var output bytes.Buffer
					fields := Fields{"payload": tc.value, "empty": ""}
					options := []Option{WithOutput(&output)}
					calls := 0
					if replace {
						options = append(options, WithReplacer(func(key string, value any) any {
							if key == "payload" {
								calls++
							}
							return value
						}))
					}
					switch source {
					case "persistent":
						options = append(options, WithPersistentKeys(fields))
					case "formatter":
						options = append(options, WithFormatter(func(Fields) (any, error) { return fields, nil }))
					}
					l := New(options...)
					var extra []any
					switch source {
					case "extra":
						extra = append(extra, fields)
					case "temporary":
						l.AppendKeys(fields)
					}
					err := l.Info("record", extra...)
					if tc.wantErr != nil {
						var marshalErr *json.MarshalerError
						if !errors.Is(err, tc.wantErr) || !errors.As(err, &marshalErr) || output.Len() != 0 {
							t.Fatalf("marshaler error lost: output=%s error=%v", &output, err)
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						got := records(t, &output)[0]
						if got["payload"] != tc.wantPayload {
							t.Fatalf("marshaler value lost: %v", got)
						}
						if _, ok := got["empty"]; ok {
							t.Fatal("ordinary empty string was retained")
						}
					}
					if replace && calls != 1 {
						t.Fatalf("payload replacer called %d times, want 1", calls)
					}
					if fields["payload"] != tc.value || fields["empty"] != "" || len(fields) != 2 {
						t.Fatal("caller attributes changed")
					}
				})
			}
		}
	}
}

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
