package logger

import (
	json "encoding/json/v2"
	"strings"
	"testing"
	"time"
)

func largeSnapshotEvent() map[string]any {
	records := make([]any, 2048)
	for i := range records {
		records[i] = map[string]any{"id": float64(i), "message": strings.Repeat("x", 512), "tags": []any{"a", "b", "c"}}
	}
	return map[string]any{"id": "correlation-id", "records": records}
}
func BenchmarkLoggerLargeEvent(b *testing.B) {
	input := largeSnapshotEvent()
	var out benchmarkWriter
	l := New(WithOutput(&out), WithClock(func() time.Time { return time.Unix(0, 0).UTC() }), WithServiceName("benchmark"))
	fields := Fields{"event": input}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := l.Info("event", fields); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(out.bytes)/float64(b.N), "output-B/op")
}

var correlationResult any

func BenchmarkCorrelationSnapshot(b *testing.B) {
	input := largeSnapshotEvent()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		value, err := ExtractCorrelationID(EventBridge, input)
		if err != nil {
			b.Fatal(err)
		}
		correlationResult = value
	}
}

func TestLargeEventSnapshotRetainsCallerOwnership(t *testing.T) {
	input := largeSnapshotEvent()
	before, err := json.Marshal(input, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractCorrelationID(EventBridge, input); err != nil {
		t.Fatal(err)
	}
	var out benchmarkWriter
	if err := New(WithOutput(&out)).Info("event", Fields{"event": input}); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(input, json.Deterministic(true))
	if err != nil || string(after) != string(before) {
		t.Fatal("modified caller-owned event", err)
	}
}
