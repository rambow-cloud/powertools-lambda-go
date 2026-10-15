package logger

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

type benchmarkWriter struct{ bytes int64 }

func (w *benchmarkWriter) Write(p []byte) (int, error) {
	w.bytes += int64(len(p))
	return len(p), nil
}

// Compare identical invocation fields and output sinks. DirectJSON shares record
// enrichment but omits Logger normalization and ordered-root encoding.
func BenchmarkLoggerInvocation(b *testing.B) {
	for _, mode := range []string{"DirectJSON", "BoundINFO", "BindAndINFO", "WrappedINFO", "Filtered"} {
		b.Run(mode, func(b *testing.B) {
			for _, key := range []string{"POWERTOOLS_DEV", "POWERTOOLS_LOG_LEVEL", "POWERTOOLS_LOGGER_SAMPLE_RATE", "AWS_LAMBDA_LOG_LEVEL", "POWERTOOLS_LOGGER_LOG_EVENT"} {
				b.Setenv(key, "")
			}
			b.Setenv("AWS_LAMBDA_FUNCTION_NAME", "benchmark")
			b.Setenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", "256")
			ctx := lambdacontext.NewContext(context.Background(), &lambdacontext.LambdaContext{
				AwsRequestID: "request-123", InvokedFunctionArn: "arn:aws:lambda:ap-east-1:123456789012:function:benchmark",
			})
			ctx = context.WithValue(ctx, "x-amzn-trace-id", "Root=1-12345678-123456789012345678901234;Sampled=0")
			var out benchmarkWriter
			level := InfoLevel
			if mode == "Filtered" {
				level = WarnLevel
			}
			log := New(WithOutput(&out), WithLevel(level), WithSampleRate(0), WithServiceName("benchmark"), WithClock(func() time.Time {
				return time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
			}))
			// All modes compare warm invocation records, including cold_start=false.
			warmup := WrapHandler(log, func(context.Context, int) (int, error) { return 0, nil })
			if _, err := warmup(ctx, 0); err != nil {
				b.Fatal(err)
			}
			fields := Fields{"case_id": "case-123", "records": 3, "checksum": 210}
			write := func(bound *Logger) error {
				if mode == "DirectJSON" {
					data, err := json.Marshal(bound.record(InfoLevel, "benchmark", fields))
					if err != nil {
						return err
					}
					_, err = out.Write(append(data, '\n'))
					return err
				}
				return bound.Info("benchmark", fields)
			}
			b.ReportAllocs()
			if mode == "WrappedINFO" {
				handler := WrapHandler(log, func(ctx context.Context, _ int) (int, error) {
					return 0, write(log.WithContext(ctx))
				})
				b.ResetTimer()
				for b.Loop() {
					if _, err := handler(ctx, 0); err != nil {
						b.Fatal(err)
					}
				}
			} else {
				handler := WrapHandler(log, func(ctx context.Context, _ int) (int, error) {
					bound := log.WithContext(ctx)
					b.ResetTimer()
					for b.Loop() {
						current := bound
						if mode == "BindAndINFO" {
							current = log.WithContext(ctx)
						}
						if err := write(current); err != nil {
							b.Fatal(err)
						}
					}
					b.StopTimer()
					return 0, nil
				})
				if _, err := handler(ctx, 0); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(out.bytes)/float64(b.N), "output-B/op")
		})
	}
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
