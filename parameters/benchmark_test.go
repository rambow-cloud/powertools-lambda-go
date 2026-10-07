package parameters

import (
	"context"
	"testing"
	"time"
)

func BenchmarkParametersCache(b *testing.B) {
	ctx := context.Background()
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	for _, force := range []bool{false, true} {
		name := "Hit"
		if force {
			name = "ForceFetch"
		}
		b.Run(name, func(b *testing.B) {
			cache := NewCache(func() time.Time { return now })
			options := Options{MaxAge: Age(time.Minute), Transform: JSON, ForceFetch: force}
			calls := 0
			fetch := func(context.Context) (any, error) {
				calls++
				return `{"enabled":true,"limit":20}`, nil
			}
			if _, err := cache.Get(ctx, "config", options, fetch); err != nil {
				b.Fatal(err)
			}
			calls = 0
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				value, err := cache.Get(ctx, "config", options, fetch)
				if err != nil || value.(map[string]any)["enabled"] != true {
					b.Fatalf("value=%v, err=%v", value, err)
				}
			}
			b.StopTimer()
			if force && calls != b.N || !force && calls != 0 {
				b.Fatalf("unexpected fetches: %d", calls)
			}
		})
	}
}
