package validation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestUnusedExternalStructure(t *testing.T) {
	// External registration errors retain the Go compilation-error wrapper.
	// Validating structure must not compile unused assertions or call formats.
	for _, raw := range []string{`{"type":"bogus"}`, `{"minLength":-1}`, `{"required":"child"}`} {
		_, err := Compile(context.Background(), true, Options{CompileOptions: CompileOptions{
			ExternalRefs: map[string]any{"https://example.test/unused": json.RawMessage(raw)},
		}})
		var compilation *SchemaCompilationError
		if !errors.As(err, &compilation) {
			t.Fatalf("unused invalid document %s: %v", raw, err)
		}
	}
	calls := 0
	_, err := Compile(context.Background(), true, Options{CompileOptions: CompileOptions{
		Formats:      map[string]func(any) error{"custom": func(any) error { calls++; return nil }},
		ExternalRefs: map[string]any{"https://example.test/unused": json.RawMessage(`{"pattern":"(?i)a","format":"custom"}`)},
	}})
	if err != nil || calls != 0 {
		t.Fatalf("unused external compilation: %v, callback count: %d", err, calls)
	}
}
