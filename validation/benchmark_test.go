package validation

import (
	"context"
	"errors"
	"testing"
)

func BenchmarkValidationCompiled(b *testing.B) {
	ctx := context.Background()
	schema, err := Compile(ctx, map[string]any{
		"type": "object", "required": []any{"id", "quantity"},
		"properties": map[string]any{"id": map[string]any{"type": "string"}, "quantity": map[string]any{"type": "integer", "minimum": 1}},
	}, Options{})
	if err != nil {
		b.Fatal(err)
	}
	for _, valid := range []bool{true, false} {
		name, quantity := "Valid", 2
		if !valid {
			name, quantity = "Invalid", 0
		}
		b.Run(name, func(b *testing.B) {
			input := map[string]any{"id": "order-123", "quantity": quantity}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_, err := schema.Validate(ctx, input)
				var failure *SchemaValidationError
				if valid && err != nil || !valid && !errors.As(err, &failure) {
					b.Fatalf("unexpected validation error: %v", err)
				}
			}
		})
	}
}
