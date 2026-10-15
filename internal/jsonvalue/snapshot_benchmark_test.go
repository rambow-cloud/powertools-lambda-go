package jsonvalue_test

import (
	"strings"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/internal/jsonvalue"
)

var snapshotResult any

func BenchmarkSnapshot(b *testing.B) {
	records := make([]any, 2048)
	for i := range records {
		records[i] = map[string]any{"id": float64(i), "message": strings.Repeat("x", 512), "tags": []any{"a", "b", "c"}}
	}
	input := map[string]any{"records": records}
	for _, name := range []string{"JSONv2RoundTrip", "PrivateCopy"} {
		b.Run(name, func(b *testing.B) {
			operation := roundTrip
			if name == "PrivateCopy" {
				operation = jsonvalue.Snapshot
			}
			b.ReportAllocs()
			for b.Loop() {
				value, err := operation(input)
				if err != nil {
					b.Fatal(err)
				}
				snapshotResult = value
			}
		})
	}
}
