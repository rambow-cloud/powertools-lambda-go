package metrics

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestStartScopeNestingAndIdempotentFinish(t *testing.T) {
	var output bytes.Buffer
	m, err := New(WithNamespace("Test"), WithOutput(&output), WithDisabled(false), WithRequireMetrics(true))
	if err != nil {
		t.Fatal(err)
	}
	outer, finishOuter := m.StartScope(context.Background())
	parent := m.WithContext(outer)
	if err := parent.SetDefaultDimensions(Dimensions{"tenant": "outer"}); err != nil {
		t.Fatal(err)
	}
	if err := parent.AddMetric("Parent", Count, 1); err != nil {
		t.Fatal(err)
	}
	inner, finishInner := m.StartScope(outer)
	child := m.WithContext(inner)
	if err := child.AddMetric("Child", Count, 1); err != nil {
		t.Fatal(err)
	}
	if err := child.AddDimension("route", "GET /items"); err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for range 16 {
		wait.Go(func() {
			if err := finishInner(); err != nil {
				t.Errorf("finish: %v", err)
			}
		})
	}
	wait.Wait()
	if _, err := child.SingleMetric(); !errors.Is(err, ErrInvocationClosed) {
		t.Fatalf("single metric escaped scope: %v", err)
	}
	if err := finishOuter(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"tenant":"outer"`) || !strings.Contains(lines[0], `"Child":1`) || strings.Contains(lines[0], `"Parent":1`) || strings.Contains(lines[1], `"route"`) {
		t.Fatalf("nested scopes: %s", output.String())
	}
	_, empty := m.StartScope(context.Background())
	if !errors.Is(empty(), ErrEmptyMetrics) || !errors.Is(empty(), ErrEmptyMetrics) {
		t.Fatal("finish did not retain the flush error")
	}
}
