package validation

import (
	"context"
	"encoding/json"
	"testing"
)

func TestConditionalCompilationDoesNotExecuteBranches(t *testing.T) {
	thenCalls, elseCalls := 0, 0
	options := Options{CompileOptions: CompileOptions{Formats: map[string]func(any) error{
		"then": func(any) error { thenCalls++; return nil },
		"else": func(any) error { elseCalls++; return nil },
	}}}
	schema, err := Compile(context.Background(), json.RawMessage(`{"if":true,"then":{"format":"then"},"else":{"format":"else"}}`), options)
	if err != nil || thenCalls != 0 || elseCalls != 0 {
		t.Fatalf("compilation: %v, callbacks: %d/%d", err, thenCalls, elseCalls)
	}
	if _, err := schema.Validate(context.Background(), "value"); err != nil || thenCalls != 1 || elseCalls != 0 {
		t.Fatalf("selected branch: %v, callbacks: %d/%d", err, thenCalls, elseCalls)
	}
	schema, err = Compile(context.Background(), json.RawMessage(`{"if":{"format":"else"},"then":true,"else":{"deprecated":true}}`), options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schema.Validate(context.Background(), "value"); err != nil || thenCalls != 1 || elseCalls != 0 {
		t.Fatalf("ignored condition: %v, callbacks: %d/%d", err, thenCalls, elseCalls)
	}
}
