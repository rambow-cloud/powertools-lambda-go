package idempotency

import (
	"context"
	"testing"
	"time"
)

func BenchmarkIdempotencyReplay(b *testing.B) {
	ctx := context.Background()
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	for _, local := range []bool{false, true} {
		name := "Store"
		if local {
			name = "LocalCache"
		}
		b.Run(name, func(b *testing.B) {
			store := newMemory()
			manager, err := New(store, Options{KeyPrefix: "benchmark", UseLocalCache: local, MaxLocalCacheSize: 1, Now: func() time.Time { return now }, Diagnostic: func(string) {}})
			if err != nil {
				b.Fatal(err)
			}
			calls := 0
			handler := func(context.Context) (string, error) { calls++; return "accepted", nil }
			payload := map[string]any{"id": "order-123", "quantity": 2}
			if _, err := Execute(ctx, manager, payload, handler); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				// The shared test double records operations; clear its trace so retained
				// memory stays bounded. This bookkeeping is included in store results.
				store.operations = store.operations[:0]
				value, err := Execute(ctx, manager, payload, handler)
				if err != nil || value != "accepted" {
					b.Fatalf("value=%q, err=%v", value, err)
				}
			}
			b.StopTimer()
			if calls != 1 || len(store.records) != 1 || len(store.operations) > 2 {
				b.Fatal("replay invoked handler or retained unbounded state")
			}
		})
	}
}
