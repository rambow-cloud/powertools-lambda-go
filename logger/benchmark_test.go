package logger

import (
	"errors"
	"testing"
	"time"
)

type benchmarkWriter struct{ bytes int64 }

func (w *benchmarkWriter) Write(p []byte) (int, error) {
	w.bytes += int64(len(p))
	return len(p), nil
}

func BenchmarkErrorStack(b *testing.B) {
	for _, mode := range []string{"disabled", "logging", "attached", "capture-and-log", "filtered"} {
		b.Run(mode, func(b *testing.B) {
			b.Setenv("POWERTOOLS_DEV", "false")
			var out benchmarkWriter
			l := New(WithOutput(&out), WithLevel(InfoLevel), WithStackTrace(mode != "disabled"))
			err := errors.New("payment declined")
			if mode == "attached" {
				err = WithStack(err)
			}
			level := ErrorLevel
			if mode == "filtered" {
				level = DebugLevel
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				item := err
				if mode == "capture-and-log" {
					item = WithStack(err)
				}
				if e := l.Log(level, "Payment failed", item); e != nil {
					b.Fatal(e)
				}
			}
		})
	}
}

func BenchmarkLogger(b *testing.B) {
	for _, key := range []string{"POWERTOOLS_DEV", "POWERTOOLS_LOG_LEVEL", "POWERTOOLS_LOGGER_SAMPLE_RATE", "AWS_LAMBDA_LOG_LEVEL", "POWERTOOLS_LOGGER_LOG_EVENT", "POWERTOOLS_LOGGER_TIMEZONE"} {
		b.Setenv(key, "")
	}
	fields := Fields{"order_id": "order-123", "quantity": 2, "customer": Fields{"id": "customer-456", "region": "hk"}}
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	for _, emitted := range []bool{true, false} {
		name, level := "Emitted", InfoLevel
		if !emitted {
			name, level = "Filtered", WarnLevel
		}
		b.Run(name, func(b *testing.B) {
			var out benchmarkWriter
			log := New(WithOutput(&out), WithLevel(level), WithSampleRate(0), WithServiceName("benchmark"), WithClock(func() time.Time { return now }))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := log.Info("Processing order", fields); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			if emitted && out.bytes == 0 || !emitted && out.bytes != 0 {
				b.Fatalf("unexpected output bytes: %d", out.bytes)
			}
			b.ReportMetric(float64(out.bytes)/float64(b.N), "output-B/op")
		})
	}
}
