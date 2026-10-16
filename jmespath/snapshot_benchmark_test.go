package jmespath

import (
	"strings"
	"testing"
)

var snapshotSearchResult any

func BenchmarkSearchSnapshot(b *testing.B) {
	records := make([]any, 2048)
	for i := range records {
		records[i] = map[string]any{"id": float64(i), "message": strings.Repeat("x", 512), "tags": []any{"a", "b", "c"}}
	}
	input := map[string]any{"records": records}
	expression, err := Compile("records[0].id")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		value, err := expression.Search(input)
		if err != nil {
			b.Fatal(err)
		}
		snapshotSearchResult = value
	}
}
