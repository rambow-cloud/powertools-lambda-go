package metrics

import (
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"unicode/utf16"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

var (
	ErrInvocationClosed = errors.New("metrics invocation is closed")
	ErrEmptyMetrics     = errors.New("The number of metrics recorded must be higher than zero")
	ErrDimensionLimit   = errors.New("The number of metric dimensions must be lower than 29")
)

type metric struct {
	unit       Unit
	resolution Resolution
	values     []float64
}
type state struct {
	mu                   sync.Mutex
	defaults, dimensions Dimensions
	sets                 []Dimensions
	metadata             map[string]jsonv1.RawMessage
	metrics              map[string]*metric
	order                []string
	timestamp            *float64
	requireMetrics       *bool
	functionName         *string
	invocationCold       *bool
	warnings             []string
	closed               *atomic.Bool
}
type output struct {
	mu     sync.Mutex
	writer io.Writer
}

// Metrics may be shared across handlers. Use WithContext inside WrapHandler for
// isolated request state. Configuration callbacks and custom writers are caller-owned.
type Metrics struct {
	cfg           config
	base, current *state
	sink          *output
	cold          *commons.Utility
}

func New(options ...Option) (*Metrics, error) {
	cold := commons.NewUtility()
	c, err := defaults()
	if err != nil {
		return nil, err
	}
	for _, option := range options {
		option(&c)
	}
	if err := c.resolve(); err != nil {
		return nil, err
	}
	defaults := Dimensions{"service": c.service}
	for key, value := range c.dimensions {
		defaults[key] = value
	}
	s := newState(nil)
	if c.function != "" {
		s.functionName = &c.function
	}
	m := &Metrics{cfg: c, base: s, current: s, sink: &output{writer: c.writer}, cold: cold}
	if err := m.SetDefaultDimensions(defaults); err != nil {
		return nil, err
	}
	return m, nil
}

func newState(defaults Dimensions) *state {
	s := &state{defaults: copyDimensions(defaults), closed: &atomic.Bool{}}
	s.clear()
	return s
}

func (s *state) clear() {
	s.dimensions = Dimensions{}
	s.sets = nil
	s.metadata = map[string]jsonv1.RawMessage{}
	s.clearMetrics()
}

func copyDimensions(values Dimensions) Dimensions {
	copy := make(Dimensions, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}

func (m *Metrics) Disabled() bool { return m.cfg.disabled }

func (m *Metrics) AddMetric(name string, unit Unit, value float64, resolutions ...Resolution) error {
	resolution := Standard
	if len(resolutions) > 0 {
		resolution = resolutions[0]
	}
	if length := len(utf16.Encode([]rune(name))); length < 1 || length > 255 {
		return errors.New("The metric name should be between 1 and 255 characters")
	}
	if !unit.valid() {
		return fmt.Errorf("Invalid metric unit '%s', expected either option: %s", unit, metricUnitNames())
	}
	if resolution != Standard && resolution != High {
		return fmt.Errorf("Invalid metric resolution '%d', expected either option: %d,%d", resolution, Standard, High)
	}
	s := m.current
	s.mu.Lock()
	defer m.unlock(s)
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	// The reference flushes before the next addition after 100 distinct metrics.
	if len(s.metrics) == 100 {
		if err := m.flushLocked(); err != nil {
			return err
		}
	}
	stored := s.metrics[name]
	if inheritedMetricName(name) {
		return metricUnitConflict(name, "undefined", unit)
	}
	if stored == nil {
		stored = &metric{unit: unit, resolution: resolution}
		s.metrics[name] = stored
		s.order = append(s.order, name)
	} else if stored.unit != unit {
		return metricUnitConflict(name, stored.unit, unit)
	}
	stored.values = append(stored.values, value)
	if len(stored.values) == 100 || m.cfg.single {
		return m.flushLocked()
	}
	return nil
}

func (m *Metrics) AddDimension(name, value string) error {
	return m.updateDimensions(Dimensions{name: value}, false, false)
}

// AddDimensionSet creates a separate dimension combination, matching TS addDimensions.
func (m *Metrics) AddDimensionSet(dimensions Dimensions) error {
	return m.updateDimensions(dimensions, true, false)
}

// SetDefaultDimensions merges defaults; invocation-bound defaults remain request-local.
func (m *Metrics) SetDefaultDimensions(dimensions Dimensions) error {
	return m.updateDimensions(dimensions, false, true)
}

func (m *Metrics) updateDimensions(values Dimensions, set, defaults bool) error {
	s := m.current
	s.mu.Lock()
	defer m.unlock(s)
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	base, regular, sets := s.defaults, s.dimensions, s.sets
	updated := s.sanitizeDimensions(values, set || defaults)
	if defaults {
		base = copyDimensions(base)
		for key, value := range updated {
			base[key] = value
		}
	} else if set {
		sets = append(append([]Dimensions(nil), sets...), updated)
	} else {
		regular = copyDimensions(regular)
		for key, value := range updated {
			regular[key] = value
		}
	}
	if len(dimensionKeys(base, regular)) > 29 {
		return ErrDimensionLimit
	}
	for _, dimensions := range sets {
		if len(dimensionKeys(base, dimensions)) > 29 {
			return ErrDimensionLimit
		}
	}
	if !set && !defaults {
		for _, key := range sortedKeys(updated) {
			s.warnDuplicate(key)
		}
		// Assigning a string to Object.prototype.__proto__ creates no own property.
		delete(regular, "__proto__")
	}
	s.defaults, s.dimensions, s.sets = base, regular, sets
	return nil
}

// AddMetadata snapshots JSON immediately so subsequent caller mutation cannot alter output.
func (m *Metrics) AddMetadata(key string, value any) error {
	var data []byte
	if key != "__proto__" {
		var err error
		data, err = json.Marshal(value)
		if err != nil {
			return err
		}
	}
	s := m.current
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	if key != "__proto__" {
		s.metadata[key] = data
	}
	return nil
}

// Clear removes request metrics, dimensions, metadata, and timestamp, retaining defaults.
func (m *Metrics) Clear() error {
	s := m.current
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	s.clear()
	return nil
}

func (m *Metrics) ClearDefaultDimensions() error {
	s := m.current
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	s.defaults = Dimensions{}
	return nil
}

// SingleMetric constructs independent state from current defaults and fresh environment
// settings. It shares the output, clock, warning callback and invocation closure.
// Each addition flushes. Constructor failures are returned before any metric is added.
func (m *Metrics) SingleMetric() (*Metrics, error) {
	m.current.mu.Lock()
	if m.current.closed.Load() {
		m.current.mu.Unlock()
		return nil, ErrInvocationClosed
	}
	dimensions := copyDimensions(m.current.defaults)
	closed, cold := m.current.closed, m.current.invocationCold
	m.current.mu.Unlock()
	single, err := New(WithNamespace(m.cfg.namespace), WithDefaultDimensions(dimensions), WithSingleMetric(true), WithOutput(m.cfg.writer), WithClock(m.cfg.clock), WithWarningHandler(m.cfg.onWarning), WithErrorHandler(m.cfg.onError))
	if err != nil {
		return nil, err
	}
	single.sink = m.sink
	single.current.closed, single.current.invocationCold = closed, cold
	return single, nil
}

// Flush clears request state even if serialization or output fails, matching TS cleanup.
func (m *Metrics) Flush() error {
	s := m.current
	s.mu.Lock()
	defer m.unlock(s)
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	return m.flushLocked()
}

func (m *Metrics) flushLocked() error {
	defer m.current.clear()
	hasMetrics := len(m.current.metrics) != 0
	if !hasMetrics && !m.requireMetricsLocked() {
		m.current.warn(emptyWarning)
	}
	if m.cfg.disabled {
		return nil
	}
	data, err := m.serializeLocked()
	if err != nil || !hasMetrics {
		return err
	}
	data = append(data, '\n')
	m.sink.mu.Lock()
	defer m.sink.mu.Unlock()
	n, err := m.sink.writer.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}
