package validation

import (
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestReference(t *testing.T) {
	assertReference(t, "testdata/validation-v2.35.0.json")
}

func TestRegexReference(t *testing.T) {
	assertReference(t, "testdata/regex-v2.35.0.json")
}

func TestApplicatorReference(t *testing.T) {
	assertReference(t, "testdata/applicators-v2.35.0.json")
}

func TestSchemaReference(t *testing.T) {
	assertReference(t, "testdata/references-v2.35.0.json")
}

func TestStrictReference(t *testing.T) {
	assertReference(t, "testdata/strict-v2.35.0.json")
}

func TestSetupReference(t *testing.T) {
	assertReference(t, "testdata/setup-v2.35.0.json")
}

func TestKeywordReference(t *testing.T) {
	assertReference(t, "testdata/keywords-v2.35.0.json")
}

func TestShapeReference(t *testing.T) {
	assertReference(t, "testdata/shapes-v2.35.0.json")
}

func TestGraphReference(t *testing.T) {
	assertReference(t, "testdata/graphs-v2.35.0.json")
}

func assertReference(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name    string            `json:"name"`
			Schema  jsonv1.RawMessage `json:"schema"`
			Payload jsonv1.RawMessage `json:"payload"`
			Options struct {
				Envelope     string              `json:"envelope"`
				Formats      bool                `json:"formats"`
				ExternalRefs []jsonv1.RawMessage `json:"externalRefs"`
			} `json:"options"`
			Expected struct {
				Success bool   `json:"success"`
				Value   any    `json:"value"`
				Error   string `json:"error"`
				Message string `json:"message"`
				Issues  any    `json:"issues"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatal("reference corpus has no cases")
	}
	for _, item := range corpus.Cases {
		t.Run(item.Name, func(t *testing.T) {
			options := Options{Envelope: item.Options.Envelope}
			if item.Options.Formats {
				options.Formats = map[string]func(any) error{"startsA": func(value any) error {
					if strings.HasPrefix(value.(string), "a") {
						return nil
					}
					return fmt.Errorf("prefix required")
				}}
				options.NumberFormats = map[string]func(any) error{"even": func(value any) error {
					number, _ := value.(jsonv1.Number).Float64()
					if int(number)%2 == 0 {
						return nil
					}
					return fmt.Errorf("even required")
				}}
			}
			for _, ref := range item.Options.ExternalRefs {
				options.ExternalSchemas = append(options.ExternalSchemas, ref)
			}
			var payload any
			if err := json.Unmarshal(item.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			// Raw JSON retains object declaration order for diagnostics. Envelope
			// extraction takes native values, matching the query package contract.
			var input any = item.Payload
			if options.Envelope != "" {
				input = payload
			}
			result, err := Validate(context.Background(), input, item.Schema, options)
			if item.Expected.Success {
				if raw, ok := result.(jsonv1.RawMessage); ok {
					var decoded any
					if decodeErr := json.Unmarshal(raw, &decoded); decodeErr != nil {
						t.Fatal(decodeErr)
					}
					result = decoded
				}
				if err != nil || !reflect.DeepEqual(result, item.Expected.Value) {
					t.Fatalf("got %#v, %v; want %#v", result, err, item.Expected.Value)
				}
				return
			}
			if err == nil {
				t.Fatal("expected failure")
			}
			if err.Error() != item.Expected.Message {
				t.Fatalf("message %q; want %q", err.Error(), item.Expected.Message)
			}
			if item.Expected.Error == "SchemaCompilationError" {
				var failure *SchemaCompilationError
				if !errors.As(err, &failure) {
					t.Fatalf("error type: %T", err)
				}
				return
			}
			var failure *SchemaValidationError
			if !errors.As(err, &failure) {
				t.Fatalf("error type: %T", err)
			}
			actual, _ := json.Marshal(failure.Issues)
			var actualValue any
			if err := json.Unmarshal(actual, &actualValue); err != nil {
				t.Fatal(err)
			}
			expected, _ := json.Marshal(item.Expected.Issues)
			if !reflect.DeepEqual(actualValue, item.Expected.Issues) {
				t.Fatalf("issues got %s; want %s", actual, expected)
			}
		})
	}
}
