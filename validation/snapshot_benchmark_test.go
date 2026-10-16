package validation

import (
	"context"
	"strings"
	"testing"
)

func BenchmarkValidationLargeSnapshot(b *testing.B) {
	ctx := context.Background()
	schema, err := Compile(ctx, map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}}, Options{})
	if err != nil {
		b.Fatal(err)
	}
	input := map[string]any{"id": "valid", "body": strings.Repeat("x", 2*1024*1024)}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := schema.Validate(ctx, input); err != nil {
			b.Fatal(err)
		}
	}
}
