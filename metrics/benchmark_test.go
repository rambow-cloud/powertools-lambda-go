package metrics

import (
	"testing"
	"time"
)

type benchmarkWriter struct{ bytes int64 }

func (w *benchmarkWriter) Write(p []byte) (int, error) {
	w.bytes += int64(len(p))
	return len(p), nil
}

func BenchmarkMetricsPublish(b *testing.B) {
	for _, key := range []string{"POWERTOOLS_DEV", "POWERTOOLS_METRICS_DISABLED", "POWERTOOLS_METRICS_NAMESPACE", "POWERTOOLS_SERVICE_NAME", "POWERTOOLS_METRICS_FUNCTION_NAME"} {
		value := ""
		if key == "POWERTOOLS_DEV" || key == "POWERTOOLS_METRICS_DISABLED" {
			value = "false"
		}
		b.Setenv(key, value)
	}
	var out benchmarkWriter
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	m, err := New(WithNamespace("Benchmark"), WithServiceName("orders"), WithOutput(&out), WithClock(func() time.Time { return now }))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := m.AddMetric("Requests", Count, 1); err != nil {
			b.Fatal(err)
		}
		if err := m.Flush(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if out.bytes == 0 || m.HasStoredMetrics() {
		b.Fatal("publication failed or retained metric state")
	}
	b.ReportMetric(float64(out.bytes)/float64(b.N), "output-B/op")
}
