package validation

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
)

func TestExternalSchemaSnapshotAndConcurrentReuse(t *testing.T) {
	var calls atomic.Int32
	child := map[string]any{"$id": "child", "$schema": "https://example.test/annotation", "type": "string", "format": "count"}
	document := map[string]any{"$id": "https://example.test/document", "if": child, "then": true, "else": true}
	before, _ := json.Marshal(document)
	options := Options{CompileOptions: CompileOptions{
		ExternalSchemas: []any{document},
		Formats:         map[string]func(any) error{"count": func(any) error { calls.Add(1); return nil }},
	}}
	active, err := Compile(context.Background(), map[string]any{"$ref": "https://example.test/child"}, options)
	if err != nil {
		t.Fatal(err)
	}
	ignored, err := Compile(context.Background(), map[string]any{"$ref": "https://example.test/document"}, options)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(document)
	if string(before) != string(after) || calls.Load() != 0 {
		t.Fatalf("compilation modified caller data or invoked a callback: %s, calls %d", after, calls.Load())
	}
	child["type"] = "number"
	child["format"] = "unregistered"
	options.ExternalSchemas[0] = false
	var group sync.WaitGroup
	for range 32 {
		group.Go(func() {
			if _, err := active.Validate(context.Background(), "value"); err != nil {
				t.Errorf("active resource: %v", err)
			}
			if _, err := ignored.Validate(context.Background(), "value"); err != nil {
				t.Errorf("ignored condition: %v", err)
			}
		})
	}
	group.Wait()
	if calls.Load() != 32 {
		t.Fatalf("active resource must execute exactly once per call; got %d", calls.Load())
	}
}

func TestExternalReferenceMapOrder(t *testing.T) {
	options := Options{CompileOptions: CompileOptions{ExternalRefs: map[string]any{
		"https://example.test/z": json.RawMessage(`{"$defs":{"value":{"$id":"https://example.test/shared","type":"string"}}}`),
		"https://example.test/a": json.RawMessage(`{"$defs":{"value":{"$id":"https://example.test/shared","type":"number"}}}`),
	}}}
	schema, err := Compile(context.Background(), map[string]any{"$ref": "https://example.test/shared"}, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schema.Validate(context.Background(), "value"); err != nil {
		t.Fatalf("lexically last map registration must supply the alias: %v", err)
	}
}
