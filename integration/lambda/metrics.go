package main

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"errors"
	"math"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/metrics"
)

func metricsStoreProbe(ctx context.Context, m *metrics.Metrics) (result map[string]any, err error) {
	ctx, finish := m.StartScope(ctx)
	defer func() {
		if failure := finish(); err == nil {
			err = failure
		}
	}()
	bound := m.WithContext(ctx)
	if err := bound.CaptureColdStartMetric("manual-after-wrapper"); err != nil {
		return nil, err
	}
	if err := bound.CaptureColdStartMetric("manual-again"); err != nil {
		return nil, err
	}
	timestamp := time.Now().Add(-time.Hour)
	for _, operation := range []func() error{
		func() error { return bound.SetThrowOnEmptyMetrics(true) },
		func() error { return bound.AddDimension("request", "discarded") },
		func() error { return bound.AddDimensionSet(metrics.Dimensions{"stage": "discarded"}) },
		func() error { return bound.AddMetadata("context", "discarded") },
		func() error { return bound.SetTimestamp(timestamp) },
		func() error { return bound.AddMetric("Discarded", metrics.Count, 1) },
		bound.ClearDimensions,
		bound.ClearMetadata,
	} {
		if err := operation(); err != nil {
			return nil, err
		}
	}
	result = map[string]any{"before_clear": bound.HasStoredMetrics()}
	if err := bound.ClearMetrics(); err != nil {
		return nil, err
	}
	result["after_clear"] = bound.HasStoredMetrics()
	_, empty := bound.Serialize()
	result["empty_error"] = errors.Is(empty, metrics.ErrEmptyMetrics)
	if err := bound.SetThrowOnEmptyMetrics(false); err != nil {
		return nil, err
	}
	serialized, err := bound.Serialize()
	if err != nil {
		return nil, err
	}
	result["serialized"] = jsonv1.RawMessage(serialized)
	result["old_timestamp"] = timestamp.UnixMilli()
	result["diagnostics"], err = metricsDiagnosticProbe(ctx)
	if err != nil {
		return nil, err
	}
	result["manual_capture_ok"] = true
	result["cold_start"], err = metricsColdStartProbe(ctx)
	if err != nil {
		return nil, err
	}
	result["configuration"], err = metricsConfigurationProbe(ctx)
	if err != nil {
		return nil, err
	}
	result["values"], err = metricsValueProbe(ctx)
	if err != nil {
		return nil, err
	}
	result["timestamps"], err = metricsTimestampProbe(ctx)
	if err != nil {
		return nil, err
	}
	result["wrappers"], err = metricsWrapperProbe(ctx)
	return result, err
}

func metricsWrapperProbe(ctx context.Context) (map[string]any, error) {
	var output bytes.Buffer
	targets := make([]*metrics.Metrics, 2)
	for i, namespace := range []string{"WrapperA", "WrapperB"} {
		m, err := metrics.New(metrics.WithNamespace(namespace), metrics.WithServiceName("orders"), metrics.WithOutput(&output), metrics.WithWarningHandler(func(string) {}))
		if err != nil {
			return nil, err
		}
		targets[i] = m
	}
	var retained []*metrics.Metrics
	handler := metrics.WrapHandlers(targets, func(ctx context.Context, _ int) (int, error) {
		for _, target := range targets {
			bound := target.WithContext(ctx)
			retained = append(retained, bound)
			if err := bound.AddMetric("Count", metrics.Count, 1); err != nil {
				return 0, err
			}
		}
		return 42, nil
	}, metrics.HandlerOptions{CaptureColdStart: true, ThrowOnEmptyMetrics: true, DefaultDimensions: metrics.Dimensions{"stage": "wrapper"}, PropagateErrors: true})
	value, err := handler(ctx, 0)
	if err != nil {
		return nil, err
	}
	documents := []jsonv1.RawMessage{}
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'}) {
		if len(line) > 0 {
			documents = append(documents, append(jsonv1.RawMessage(nil), line...))
		}
	}
	closed := true
	for _, bound := range retained {
		closed = closed && errors.Is(bound.AddMetric("Late", metrics.Count, 1), metrics.ErrInvocationClosed)
	}
	output.Reset()
	// The first empty strict instance stops the second instance's publication.
	reports := 0
	strict, err := metrics.New(metrics.WithNamespace("Strict"), metrics.WithServiceName("orders"), metrics.WithOutput(&output), metrics.WithRequireMetrics(true), metrics.WithErrorHandler(func(error) { reports++ }))
	if err != nil {
		return nil, err
	}
	var skipped *metrics.Metrics
	failed := metrics.WrapHandlers([]*metrics.Metrics{strict, targets[1]}, func(ctx context.Context, _ int) (int, error) {
		skipped = targets[1].WithContext(ctx)
		return 99, skipped.AddMetric("Skipped", metrics.Count, 1)
	}, metrics.HandlerOptions{PropagateErrors: true})
	failedValue, failure := failed(ctx, 0)
	return map[string]any{
		"value": value, "documents": documents, "closed": closed,
		"strict_error": errors.Is(failure, metrics.ErrEmptyMetrics) && failedValue == 0,
		"reports":      reports, "stopped": output.Len() == 0,
		"skipped_closed": !skipped.HasStoredMetrics() && errors.Is(skipped.AddMetric("Late", metrics.Count, 1), metrics.ErrInvocationClosed),
	}, nil
}

func metricsTimestampProbe(ctx context.Context) (result map[string]any, err error) {
	const now = 1789992000000
	var output bytes.Buffer
	clockCalls, warnings := 0, 0
	m, err := metrics.New(metrics.WithNamespace("TimestampProbe"), metrics.WithServiceName("orders"), metrics.WithOutput(&output), metrics.WithClock(func() time.Time {
		clockCalls++
		return time.UnixMilli(now)
	}), metrics.WithWarningHandler(func(string) { warnings++ }))
	if err != nil {
		return nil, err
	}
	ctx, finish := m.StartScope(ctx)
	defer func() {
		if failure := finish(); err == nil {
			err = failure
		}
	}()
	bound := m.WithContext(ctx)
	if err := bound.AddMetric("Count", metrics.Count, 1); err != nil {
		return nil, err
	}
	documents := []jsonv1.RawMessage{}
	for _, set := range []func() error{
		func() error { return bound.SetTimestampMillis(0.5) },
		func() error { return bound.SetTimestampMillis(math.NaN()) },
		func() error { return bound.SetTimestamp(time.UnixMilli(8640000000000001)) },
		func() error { return bound.SetTimestampMillis(now - 14*24*60*60*1000) },
		func() error { return bound.SetTimestampMillis(now + 2*60*60*1000) },
	} {
		if err := set(); err != nil {
			return nil, err
		}
		document, err := bound.Serialize()
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	result = map[string]any{"documents": documents, "explicit_clock_calls": clockCalls}
	if err := bound.ClearMetrics(); err != nil {
		return nil, err
	}
	if err := bound.AddMetric("Count", metrics.Count, 1); err != nil {
		return nil, err
	}
	reset, err := bound.Serialize()
	if err != nil {
		return nil, err
	}
	result["reset"], result["reset_clock_calls"] = jsonv1.RawMessage(reset), clockCalls
	if err := finish(); err != nil {
		return nil, err
	}
	result["warnings"], result["final_clock_calls"] = warnings, clockCalls
	result["closed"] = errors.Is(bound.SetTimestampMillis(math.Inf(1)), metrics.ErrInvocationClosed) && errors.Is(bound.SetTimestamp(time.UnixMilli(now)), metrics.ErrInvocationClosed)
	return result, nil
}

func metricsValueProbe(ctx context.Context) (map[string]any, error) {
	var output bytes.Buffer
	m, err := metrics.New(metrics.WithNamespace("ValuesProbe"), metrics.WithServiceName("orders"), metrics.WithOutput(&output))
	if err != nil {
		return nil, err
	}
	ctx, finish := m.StartScope(ctx)
	bound := m.WithContext(ctx)
	for _, item := range []struct {
		name  string
		value float64
	}{
		{"10", 10}, {"2", 2}, {"01", 1}, {"NonFinite", math.NaN()}, {"NonFinite", math.Inf(1)}, {"NonFinite", math.Inf(-1)}, {"NonFinite", math.Copysign(0, -1)},
	} {
		if err := bound.AddMetric(item.name, metrics.Count, item.value); err != nil {
			return nil, err
		}
	}
	if err := bound.AddDimension("__proto__", "ignored"); err != nil {
		return nil, err
	}
	prototype := bound.AddMetric("constructor", metrics.Count, 1)
	if prototype == nil {
		return nil, errors.New("expected inherited metric name conflict")
	}
	document, err := bound.Serialize()
	if err != nil {
		return nil, err
	}
	if err := bound.AddMetadata("_aws", map[string]bool{"custom": true}); err != nil {
		return nil, err
	}
	reserved, err := bound.Serialize()
	if err != nil {
		return nil, err
	}
	if err := finish(); err != nil {
		return nil, err
	}
	return map[string]any{
		"document":        jsonv1.RawMessage(document),
		"reserved":        jsonv1.RawMessage(reserved),
		"prototype_error": prototype.Error(),
		"closed":          !bound.HasStoredMetrics() && errors.Is(bound.AddMetric("Late", metrics.Count, math.NaN()), metrics.ErrInvocationClosed),
	}, nil
}

type metricsProbeConfig struct{ calls []string }

func (c *metricsProbeConfig) GetNamespace() (string, error) {
	c.calls = append(c.calls, "namespace")
	return "Configured", nil
}

func (c *metricsProbeConfig) GetServiceName() (string, error) {
	c.calls = append(c.calls, "service")
	return "custom-service", nil
}

func metricsConfigurationProbe(ctx context.Context) (map[string]any, error) {
	var output bytes.Buffer
	config := &metricsProbeConfig{}
	m, err := metrics.New(metrics.WithConfigService(config), metrics.WithOutput(&output), metrics.WithDisabled(true), metrics.WithRequireMetrics(true), metrics.WithWarningHandler(func(string) {}))
	if err != nil {
		return nil, err
	}
	ctx, finish := m.StartScope(ctx)
	bound := m.WithContext(ctx)
	if err := bound.AddMetric("Parent", metrics.Count, 2); err != nil {
		return nil, err
	}
	parent, err := bound.Serialize()
	if err != nil {
		return nil, err
	}
	if err := bound.ClearDefaultDimensions(); err != nil {
		return nil, err
	}
	child, err := bound.SingleMetric()
	if err != nil {
		return nil, err
	}
	if _, err := child.Serialize(); err != nil {
		return nil, err
	}
	if err := child.AddMetric("Child", metrics.Count, 3); err != nil {
		return nil, err
	}
	if err := finish(); err != nil {
		return nil, err
	}
	return map[string]any{
		"calls":           config.calls,
		"parent":          jsonv1.RawMessage(parent),
		"child":           jsonv1.RawMessage(bytes.TrimSpace(output.Bytes())),
		"parent_disabled": bound.Disabled(),
		"child_disabled":  child.Disabled(),
		"closed":          errors.Is(child.AddMetric("Late", metrics.Count, 1), metrics.ErrInvocationClosed),
	}, nil
}

func metricsColdStartProbe(ctx context.Context) (map[string]any, error) {
	var output bytes.Buffer
	m, err := metrics.New(metrics.WithNamespace("ColdProbe"), metrics.WithServiceName("orders"), metrics.WithFunctionName("configured"), metrics.WithOutput(&output), metrics.WithWarningHandler(func(string) {}))
	if err != nil {
		return nil, err
	}
	ctx, finish := m.StartScope(ctx)
	bound := m.WithContext(ctx)
	for _, operation := range []func() error{
		func() error { return bound.SetFunctionName(" scoped ") },
		bound.Clear,
		func() error { return bound.CaptureColdStartMetric("argument") },
		func() error { return bound.CaptureColdStartMetric("duplicate") },
		finish,
	} {
		if err := operation(); err != nil {
			return nil, err
		}
	}
	documents := []jsonv1.RawMessage{}
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'}) {
		if len(line) > 0 {
			documents = append(documents, jsonv1.RawMessage(line))
		}
	}
	return map[string]any{
		"documents": documents,
		"closed":    errors.Is(bound.CaptureColdStartMetric(), metrics.ErrInvocationClosed) && errors.Is(bound.SetFunctionName("late"), metrics.ErrInvocationClosed),
	}, nil
}

func metricsDiagnosticProbe(ctx context.Context) (map[string]any, error) {
	var output bytes.Buffer
	var warnings []string
	var bound *metrics.Metrics
	observedMetrics := false
	m, err := metrics.New(metrics.WithNamespace("Diagnostics"), metrics.WithServiceName("orders"), metrics.WithDisabled(false), metrics.WithOutput(&output), metrics.WithWarningHandler(func(message string) {
		warnings = append(warnings, message)
		observedMetrics = observedMetrics || bound.HasStoredMetrics()
	}))
	if err != nil {
		return nil, err
	}
	ctx, finish := m.StartScope(ctx)
	bound = m.WithContext(ctx)
	timestamp := time.Now().Add(-15 * 24 * time.Hour)
	for _, operation := range []func() error{
		func() error { return bound.AddDimension("invalid", "\ufeff") },
		func() error { return bound.AddDimension("service", "checkout") },
		func() error { return bound.SetTimestamp(timestamp) },
		func() error { return bound.AddMetadata("service", "overwritten") },
		func() error { return bound.AddMetric("Count", metrics.Count, 1) },
		finish,
		finish,
	} {
		if err := operation(); err != nil {
			return nil, err
		}
	}
	result := map[string]any{
		"warnings":             warnings,
		"serialized":           jsonv1.RawMessage(bytes.TrimSpace(output.Bytes())),
		"timestamp":            timestamp.UnixMilli(),
		"callback_saw_metrics": observedMetrics,
		"closed":               errors.Is(bound.AddDimension("late", ""), metrics.ErrInvocationClosed),
	}
	return result, nil
}
