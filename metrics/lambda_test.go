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
	"sync"
	"testing"
	"time"
)

func TestWrapperReference(t *testing.T) {
	data, err := os.ReadFile("testdata/wrappers-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Now   int64
		Cases []struct {
			Targets                            []int
			Defaults                           Dimensions
			Required, Strict, Disabled, Called bool
			Mode                               string
			Value                              *int
			Error, Stage                       *string
			Warnings                           []string
			Emitted                            []map[string]any
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 876 {
		t.Fatalf("case count: %d", len(fixture.Cases))
	}
	for index, item := range fixture.Cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			for _, key := range []string{"POWERTOOLS_METRICS_NAMESPACE", "POWERTOOLS_SERVICE_NAME", "POWERTOOLS_METRICS_FUNCTION_NAME", "POWERTOOLS_DEV"} {
				t.Setenv(key, "")
			}
			t.Setenv("POWERTOOLS_METRICS_DISABLED", fmt.Sprint(item.Disabled))
			t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "provisioned-concurrency")
			var output bytes.Buffer
			warnings, reported := []string{}, []string{}
			instances := make([]*Metrics, 2)
			for i := range instances {
				instances[i], err = New(WithNamespace(fmt.Sprintf("Wrapper%d", i)), WithServiceName("orders"), WithDefaultDimensions(Dimensions{"environment": "base"}), WithRequireMetrics(item.Required), WithOutput(&output), WithClock(func() time.Time { return time.UnixMilli(fixture.Now) }), WithWarningHandler(func(message string) { warnings = append(warnings, fmt.Sprintf("%d:%s", i, message)) }), WithErrorHandler(func(err error) { reported = append(reported, err.Error()) }))
				if err != nil {
					t.Fatal(err)
				}
			}
			targets := make([]*Metrics, len(item.Targets))
			for i, target := range item.Targets {
				targets[i] = instances[target]
			}
			called := false
			var retained []*Metrics
			handler := WrapHandlers(targets, func(ctx context.Context, _ int) (int, error) {
				for _, target := range targets {
					retained = append(retained, target.WithContext(ctx))
				}
				if item.Mode == "early" {
					return 7, nil
				}
				called = true
				seen := map[int]bool{}
				for _, target := range item.Targets {
					if seen[target] {
						continue
					}
					seen[target] = true
					if item.Mode != "empty" && (item.Mode != "partial" || target == 1) {
						if err := instances[target].WithContext(ctx).AddMetric("Count", Count, float64(target+1)); err != nil {
							return 0, err
						}
					}
				}
				if item.Mode == "error" {
					return 0, errors.New("business failure")
				}
				return 42, nil
			}, HandlerOptions{CaptureColdStart: true, ThrowOnEmptyMetrics: item.Strict, DefaultDimensions: item.Defaults, PropagateErrors: true})
			result, failure := handler(context.Background(), 0)
			var value *int
			var message *string
			if failure == nil {
				value = &result
			} else {
				text := failure.Error()
				message = &text
			}
			if !reflect.DeepEqual(value, item.Value) || !reflect.DeepEqual(message, item.Error) || called != item.Called || !reflect.DeepEqual(warnings, item.Warnings) || !reflect.DeepEqual(documents(t, &output), item.Emitted) {
				t.Fatalf("result=%v error=%v called=%v warnings=%v emitted=%s; want %#v", result, failure, called, warnings, output.String(), item)
			}
			wantReports := []string{}
			if item.Stage != nil && *item.Stage != "handler" {
				wantReports = append(wantReports, *item.Error)
			}
			if !reflect.DeepEqual(reported, wantReports) {
				t.Fatalf("reported=%v want=%v", reported, wantReports)
			}
			for _, bound := range retained {
				if !errors.Is(bound.AddMetric("Late", Count, 1), ErrInvocationClosed) {
					t.Fatal("late write accepted")
				}
			}
			for _, m := range instances {
				if m.HasStoredMetrics() || !reflect.DeepEqual(m.current.defaults, Dimensions{"environment": "base", "service": "orders"}) {
					t.Fatal("wrapper changed parent state")
				}
			}
		})
	}
}

func TestWrapperGroupSnapshotConcurrencyAndNestedOwnership(t *testing.T) {
	a, outA := setup(t, WithWarningHandler(func(string) {}))
	b, outB := setup(t, WithWarningHandler(func(string) {}))
	defaults := Dimensions{"region": "original"}
	targets := []*Metrics{a, b}
	inner := WrapHandlers(targets, func(ctx context.Context, n int) (int, error) {
		for _, m := range []*Metrics{a, b} {
			bound := m.WithContext(ctx)
			if err := bound.AddDimension("request", fmt.Sprint(n)); err != nil {
				return 0, err
			}
			if err := bound.AddMetric("Count", Count, float64(n)); err != nil {
				return 0, err
			}
		}
		return n, nil
	}, HandlerOptions{DefaultDimensions: defaults, ThrowOnEmptyMetrics: true})
	defaults["region"], targets[0] = "mutated", nil
	outer := WrapHandler(a, inner, HandlerOptions{DefaultDimensions: Dimensions{"region": "outer"}})
	var group sync.WaitGroup
	for i := range 64 {
		group.Go(func() {
			if result, err := outer(context.Background(), i); err != nil || result != i {
				t.Errorf("%d: %v / %d", i, err, result)
			}
		})
	}
	group.Wait()
	for i, output := range []*bytes.Buffer{outA, outB} {
		got := documents(t, output)
		if len(got) != 64 {
			t.Fatalf("document count: %d", len(got))
		}
		seen := map[string]bool{}
		region := "outer"
		if i == 1 {
			region = "original"
		}
		for _, doc := range got {
			id := doc["request"].(string)
			if seen[id] || id != fmt.Sprint(doc["Count"]) || doc["region"] != region {
				t.Fatalf("document: %v", doc)
			}
			seen[id] = true
		}
	}
}

func TestWrapperGroupFailureCleanupAndPanicPrecedence(t *testing.T) {
	for _, propagate := range []bool{false, true} {
		for _, panicHandler := range []bool{false, true} {
			var retained []*Metrics
			reports := 0
			a, _ := setup(t, WithOutput(failingWriter{}), WithErrorHandler(func(err error) {
				reports++
				if !errors.Is(err, io.ErrClosedPipe) {
					t.Error(err)
				}
				for _, m := range retained {
					if !errors.Is(m.AddMetric("Late", Count, 1), ErrInvocationClosed) {
						t.Error("callback observed an open scope")
					}
				}
			}))
			b, output := setup(t)
			business := errors.New("business")
			handler := WrapHandlers([]*Metrics{a, b}, func(ctx context.Context, _ int) (int, error) {
				for _, m := range []*Metrics{a, b} {
					bound := m.WithContext(ctx)
					retained = append(retained, bound)
					if err := bound.AddMetric("Count", Count, 1); err != nil {
						return 0, err
					}
				}
				if panicHandler {
					panic(business)
				}
				return 42, business
			}, HandlerOptions{PropagateErrors: propagate})
			var recovered any
			var result int
			var err error
			func() { defer func() { recovered = recover() }(); result, err = handler(context.Background(), 0) }()
			if propagate {
				if result != 0 || !errors.Is(err, io.ErrClosedPipe) || recovered != nil {
					t.Fatal(result, err, recovered)
				}
			} else if panicHandler {
				if recovered != business {
					t.Fatal(recovered)
				}
			} else if result != 42 || err != business {
				t.Fatal(result, err)
			}
			if reports != 1 || output.Len() != 0 {
				t.Fatal("publication continued after the first failure")
			}
			for _, m := range retained {
				if m.HasStoredMetrics() {
					t.Fatal("unpublished scope retained metrics")
				}
			}
		}
	}
}

type wrapperWriterFunc func([]byte) (int, error)

func (f wrapperWriterFunc) Write(data []byte) (int, error) { return f(data) }

func TestWrapperGroupClosesAfterCallbackPanic(t *testing.T) {
	for _, source := range []string{"writer", "warning", "error"} {
		t.Run(source, func(t *testing.T) {
			failure := &struct{}{}
			options := []Option{}
			switch source {
			case "writer":
				options = append(options, WithOutput(wrapperWriterFunc(func([]byte) (int, error) { panic(failure) })))
			case "warning":
				options = append(options, WithWarningHandler(func(string) { panic(failure) }))
			case "error":
				options = append(options, WithOutput(failingWriter{}), WithErrorHandler(func(error) { panic(failure) }))
			}
			a, _ := setup(t, options...)
			b, output := setup(t)
			var retained []*Metrics
			handler := WrapHandlers([]*Metrics{a, b}, func(ctx context.Context, _ int) (int, error) {
				for i, target := range []*Metrics{a, b} {
					bound := target.WithContext(ctx)
					retained = append(retained, bound)
					if source != "warning" || i > 0 {
						if err := bound.AddMetric("Count", Count, 1); err != nil {
							return 0, err
						}
					}
				}
				return 42, nil
			})
			var recovered any
			func() { defer func() { recovered = recover() }(); _, _ = handler(context.Background(), 0) }()
			if recovered != failure || output.Len() != 0 {
				t.Fatal("panic identity or publication order changed")
			}
			for _, m := range retained {
				if m.HasStoredMetrics() || !errors.Is(m.AddMetric("Late", Count, 1), ErrInvocationClosed) {
					t.Fatal("panic leaked an invocation scope")
				}
			}
		})
	}
}
