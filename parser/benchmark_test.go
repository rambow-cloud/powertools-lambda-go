package parser_test

import (
	"context"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

func BenchmarkParserObject(b *testing.B) {
	ctx := context.Background()
	schema := parser.Object(parser.Field{Name: "id", Schema: parser.String()}, parser.Field{Name: "quantity", Schema: parser.Number()})
	for _, valid := range []bool{true, false} {
		name, quantity := "Valid", any(2)
		if !valid {
			name, quantity = "Invalid", "two"
		}
		b.Run(name, func(b *testing.B) {
			input := map[string]any{"id": "order-123", "quantity": quantity}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err := parser.SafeParse(ctx, input, schema)
				if err != nil || result.Success != valid {
					b.Fatalf("success=%v, err=%v", result.Success, err)
				}
			}
		})
	}
}
