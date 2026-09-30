package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func setup(t *testing.T, opts ...Option) (*Metrics, *bytes.Buffer) {
	t.Helper()
	for _, key := range []string{"POWERTOOLS_METRICS_NAMESPACE", "POWERTOOLS_METRICS_DISABLED", "POWERTOOLS_DEV", "POWERTOOLS_SERVICE_NAME", "POWERTOOLS_METRICS_FUNCTION_NAME"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	out := &bytes.Buffer{}
	options := []Option{WithNamespace("Example"), WithServiceName("orders"), WithDefaultDimensions(Dimensions{"environment": "test"}), WithOutput(out)}
	options = append(options, opts...)
	m, err := New(options...)
	if err != nil {
		t.Fatal(err)
	}
	return m, out
}

func documents(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	result := []map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	for {
		var document map[string]any
		err := decoder.Decode(&document)
		if err == io.EOF {
			return result
		}
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, document)
	}
}

func TestTypeScriptReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		Cases map[string][]map[string]any `json:"cases"`
	}
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	for name, action := range map[string]func(*Metrics){
		"dimensions_and_flush": func(m *Metrics) {
			_ = m.AddDimension("region", "hk")
			_ = m.AddDimensionSet(Dimensions{"stage": "prod"})
			_ = m.AddMetadata("request_id", "request-1")
			_ = m.AddMetric("Requests", Count, 1)
			_ = m.AddMetric("Latency", Milliseconds, 10, High)
			_ = m.AddMetric("Latency", Milliseconds, 20)
			_ = m.Flush()
			_ = m.AddMetric("Next", Count, 2)
			_ = m.Flush()
		},
		"metric_limit": func(m *Metrics) {
			_ = m.AddDimension("request", "first")
			for i := range 101 {
				if err := m.AddMetric(fmt.Sprintf("Metric%d", i), Count, float64(i)); err != nil {
					t.Fatal(err)
				}
			}
			_ = m.Flush()
		},
		"value_limit": func(m *Metrics) {
			_ = m.AddDimension("request", "first")
			for i := range 101 {
				if err := m.AddMetric("Value", Count, float64(i)); err != nil {
					t.Fatal(err)
				}
			}
			_ = m.Flush()
		},
		"single_metric": func(m *Metrics) {
			_ = m.AddDimension("request", "parent")
			_ = m.AddMetric("Parent", Count, 1)
			single, err := m.SingleMetric()
			if err != nil {
				t.Fatal(err)
			}
			_ = single.AddMetric("Child", Count, 2)
			_ = m.Flush()
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, out := setup(t)
			action(m)
			got := documents(t, out)
			for _, document := range got {
				delete(document["_aws"].(map[string]any), "Timestamp")
			}
			if !reflect.DeepEqual(got, reference.Cases[name]) {
				t.Fatalf("Go: %#v\nTypeScript: %#v", got, reference.Cases[name])
			}
		})
	}
}

func TestValidationAndCleanup(t *testing.T) {
	m, out := setup(t, WithRequireMetrics(true))
	if err := m.Flush(); !errors.Is(err, ErrEmptyMetrics) {
		t.Fatal(err)
	}
	for _, name := range []string{"", strings.Repeat("x", 256)} {
		if m.AddMetric(name, Count, 1) == nil {
			t.Fatal("invalid name accepted")
		}
	}
	if m.AddMetric("Invalid", Unit("bad"), 1) == nil || m.AddMetric("Invalid", Count, 1, Resolution(10)) == nil {
		t.Fatal("invalid unit or resolution accepted")
	}
	_ = m.AddMetric("Count", Count, 1)
	if m.AddMetric("Count", Bytes, 2) == nil {
		t.Fatal("unit change accepted")
	}
	_ = m.AddMetadata("Count", "collision")
	if err := m.Flush(); err == nil || out.Len() != 0 {
		t.Fatal("collision emitted invalid EMF", err)
	}
	if err := m.Flush(); !errors.Is(err, ErrEmptyMetrics) {
		t.Fatal("failed flush did not clear state", err)
	}
}

func TestDimensionLimitAndAtomicUpdates(t *testing.T) {
	m, _ := setup(t)
	values := Dimensions{}
	for i := range 27 {
		values[fmt.Sprintf("key%d", i)] = "value"
	}
	if err := m.SetDefaultDimensions(values); err != nil {
		t.Fatal(err)
	}
	if err := m.AddDimension("overflow", "value"); !errors.Is(err, ErrDimensionLimit) {
		t.Fatal(err)
	}
	if err := m.AddDimensionSet(Dimensions{"overflow": "value"}); !errors.Is(err, ErrDimensionLimit) {
		t.Fatal(err)
	}
	_ = m.AddMetric("Count", Count, 1)
	data, err := m.Serialize()
	if err != nil || bytes.Contains(data, []byte("overflow")) {
		t.Fatal(string(data), err)
	}
}

func TestMetadataSnapshotAndTimestamp(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	m, out := setup(t, WithClock(func() time.Time { return now }))
	metadata := map[string]string{"value": "original"}
	_ = m.AddMetadata("context", metadata)
	metadata["value"] = "changed"
	if err := m.SetTimestamp(now.Add(-15 * 24 * time.Hour)); err != nil {
		t.Fatal("reference stores old timestamps with a warning", err)
	}
	if err := m.SetTimestamp(now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	_ = m.AddMetric("Count", Count, 1)
	_ = m.Flush()
	got := documents(t, out)[0]
	if got["context"].(map[string]any)["value"] != "original" || got["_aws"].(map[string]any)["Timestamp"] != float64(now.Add(-time.Hour).UnixMilli()) {
		t.Fatal(got)
	}
}

func TestDisabledEnvironmentPrecedence(t *testing.T) {
	for _, tc := range []struct {
		value         string
		present, want bool
	}{{"", false, true}, {"false", true, false}, {"true", true, true}, {"off", true, false}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			t.Setenv("POWERTOOLS_DEV", "true")
			t.Setenv("POWERTOOLS_METRICS_DISABLED", tc.value)
			if !tc.present {
				_ = os.Unsetenv("POWERTOOLS_METRICS_DISABLED")
			}
			m, err := New()
			if err != nil || m.Disabled() != tc.want {
				t.Fatal(err, m.Disabled())
			}
		})
	}
	m, out := setup(t, WithDisabled(true), WithRequireMetrics(true))
	_ = m.AddMetric("Count", Count, 1)
	if err := m.Flush(); err != nil || out.Len() != 0 {
		t.Fatal(err, out.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWrapperPreservesResultsAndClosesState(t *testing.T) {
	var reported error
	m, _ := setup(t, WithOutput(failingWriter{}), WithErrorHandler(func(err error) { reported = err }))
	business := errors.New("business failure")
	var retained, single *Metrics
	h := WrapHandler(m, func(ctx context.Context, _ int) (int, error) {
		retained = m.WithContext(ctx)
		var err error
		single, err = retained.SingleMetric()
		if err != nil {
			return 0, err
		}
		_ = retained.AddMetric("Count", Count, 1)
		return 42, business
	})
	if result, err := h(context.Background(), 0); result != 42 || err != business || !errors.Is(reported, io.ErrClosedPipe) {
		t.Fatal(result, err, reported)
	}
	if retained.AddMetric("Late", Count, 1) != ErrInvocationClosed || single.AddMetric("Late", Count, 1) != ErrInvocationClosed {
		t.Fatal("late write accepted")
	}
	panicValue := &struct{}{}
	panicHandler := WrapHandler(m, func(ctx context.Context, _ int) (int, error) {
		_ = m.WithContext(ctx).AddMetric("Panic", Count, 1)
		panic(panicValue)
	})
	func() {
		defer func() {
			if recover() != panicValue {
				t.Fatal("panic identity changed")
			}
		}()
		_, _ = panicHandler(context.Background(), 0)
	}()
}

func TestConcurrentInvocationIsolation(t *testing.T) {
	m, out := setup(t)
	h := WrapHandler(m, func(ctx context.Context, index int) (int, error) {
		bound := m.WithContext(ctx)
		if err := bound.AddDimension("request", fmt.Sprint(index)); err != nil {
			return 0, err
		}
		return index, bound.AddMetric("Count", Count, float64(index))
	})
	var group sync.WaitGroup
	for i := range 100 {
		group.Add(1)
		go func() {
			defer group.Done()
			if result, err := h(context.Background(), i); err != nil || result != i {
				t.Errorf("%d: %v", i, err)
			}
		}()
	}
	group.Wait()
	got := documents(t, out)
	if len(got) != 100 {
		t.Fatal(len(got))
	}
	seen := map[string]bool{}
	for _, document := range got {
		id := document["request"].(string)
		if id != fmt.Sprint(document["Count"]) || seen[id] {
			t.Fatal(document)
		}
		seen[id] = true
	}
}
