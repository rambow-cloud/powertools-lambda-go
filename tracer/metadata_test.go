package tracer

import (
	"context"
	"testing"
)

func TestMetadataComponentsCannotCollide(t *testing.T) {
	tr, exporter := setup(t)
	ctx, end := tr.StartSpan(context.Background(), "metadata")
	for _, item := range []struct{ namespace, key, value string }{
		{"a.b", "c", "namespace dot"}, {"a", "b.c", "key dot"},
		{"a%2Eb", "c", "literal escape"}, {"orders", "plain", "unchanged"},
	} {
		if err := tr.PutMetadata(ctx, item.key, item.value, item.namespace); err != nil {
			t.Fatal(err)
		}
	}
	end(nil)
	got := attrs(exporter.GetSpans()[0])
	for key, value := range map[string]string{
		"powertools.metadata.a%2Eb.c":      `"namespace dot"`,
		"powertools.metadata.a.b%2Ec":      `"key dot"`,
		"powertools.metadata.a%252Eb.c":    `"literal escape"`,
		"powertools.metadata.orders.plain": `"unchanged"`,
	} {
		if got[key] != value {
			t.Fatalf("%s: %v", key, got[key])
		}
	}
}
