// Package metrics emits CloudWatch Embedded Metric Format documents for Go Lambda.
package metrics

import (
	"io"
	"slices"
	"strings"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type Unit string

const (
	Seconds            Unit = "Seconds"
	Microseconds       Unit = "Microseconds"
	Milliseconds       Unit = "Milliseconds"
	Bytes              Unit = "Bytes"
	Kilobytes          Unit = "Kilobytes"
	Megabytes          Unit = "Megabytes"
	Gigabytes          Unit = "Gigabytes"
	Terabytes          Unit = "Terabytes"
	Bits               Unit = "Bits"
	Kilobits           Unit = "Kilobits"
	Megabits           Unit = "Megabits"
	Gigabits           Unit = "Gigabits"
	Terabits           Unit = "Terabits"
	Percent            Unit = "Percent"
	Count              Unit = "Count"
	BytesPerSecond     Unit = "Bytes/Second"
	KilobytesPerSecond Unit = "Kilobytes/Second"
	MegabytesPerSecond Unit = "Megabytes/Second"
	GigabytesPerSecond Unit = "Gigabytes/Second"
	TerabytesPerSecond Unit = "Terabytes/Second"
	BitsPerSecond      Unit = "Bits/Second"
	KilobitsPerSecond  Unit = "Kilobits/Second"
	MegabitsPerSecond  Unit = "Megabits/Second"
	GigabitsPerSecond  Unit = "Gigabits/Second"
	TerabitsPerSecond  Unit = "Terabits/Second"
	CountPerSecond     Unit = "Count/Second"
	NoUnit             Unit = "None"
)

var metricUnits = []Unit{Seconds, Microseconds, Milliseconds, Bytes, Kilobytes, Megabytes, Gigabytes, Terabytes, Bits, Kilobits, Megabits, Gigabits, Terabits, Percent, Count, BytesPerSecond, KilobytesPerSecond, MegabytesPerSecond, GigabytesPerSecond, TerabytesPerSecond, BitsPerSecond, KilobitsPerSecond, MegabitsPerSecond, GigabitsPerSecond, TerabitsPerSecond, CountPerSecond, NoUnit}

func (u Unit) valid() bool { return slices.Contains(metricUnits, u) }

func metricUnitNames() string {
	names := make([]string, len(metricUnits))
	for i, unit := range metricUnits {
		names[i] = string(unit)
	}
	return strings.Join(names, ",")
}

type Resolution int

const (
	Standard Resolution = 60
	High     Resolution = 1
)

type Dimensions map[string]string
type config struct {
	namespace, service, function     string
	dimensions                       Dimensions
	writer                           io.Writer
	clock                            func() time.Time
	disabled, requireMetrics, single bool
	onError                          func(error)
	onWarning                        func(string)
	envNamespace, envService         string
	configService                    ConfigService
}
type Option func(*config)

func WithNamespace(namespace string) Option {
	return func(c *config) { c.namespace = namespace }
}
func WithServiceName(service string) Option {
	return func(c *config) { c.service = service }
}
func WithFunctionName(name string) Option {
	return func(c *config) { c.function = commons.TrimSpace(name) }
}
func WithDefaultDimensions(dimensions Dimensions) Option {
	return func(c *config) { c.dimensions = copyDimensions(dimensions) }
}
func WithOutput(writer io.Writer) Option {
	return func(c *config) {
		if writer != nil {
			c.writer = writer
		}
	}
}

// WithClock supplies a concurrency-safe clock for timestamps and validation.
func WithClock(clock func() time.Time) Option {
	return func(c *config) {
		if clock != nil {
			c.clock = clock
		}
	}
}
func WithDisabled(disabled bool) Option       { return func(c *config) { c.disabled = disabled } }
func WithRequireMetrics(required bool) Option { return func(c *config) { c.requireMetrics = required } }

// WithSingleMetric enables immediate publication after each metric addition.
func WithSingleMetric(single bool) Option { return func(c *config) { c.single = single } }

// WithErrorHandler observes wrapper flush failures without replacing business errors.
func WithErrorHandler(handler func(error)) Option {
	return func(c *config) {
		if handler != nil {
			c.onError = handler
		}
	}
}

// WithWarningHandler receives reference-compatible diagnostics. It may run during
// construction and concurrently across requests. Storage locks are released first.
// Pass a no-op function to suppress warnings; the default writes to stderr.
// Callbacks must handle their own synchronization and must not panic or recursively
// finish the scope that is delivering them. Callback panics propagate to the caller.
func WithWarningHandler(handler func(string)) Option {
	return func(c *config) {
		if handler != nil {
			c.onWarning = handler
		}
	}
}
