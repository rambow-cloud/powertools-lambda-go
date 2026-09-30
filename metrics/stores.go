package metrics

import "encoding/json"

// ClearDimensions removes regular dimensions and independent dimension sets.
// Default dimensions, metrics, metadata and the timestamp are preserved.
func (m *Metrics) ClearDimensions() error {
	s := m.current
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	s.dimensions = Dimensions{}
	s.sets = nil
	return nil
}

// ClearMetadata removes metadata without changing metrics or dimensions.
func (m *Metrics) ClearMetadata() error {
	s := m.current
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	s.metadata = map[string]json.RawMessage{}
	return nil
}

// ClearMetrics removes buffered metrics and the explicit timestamp, retaining
// dimensions, metadata and the empty-metric policy, as in the reference store.
func (m *Metrics) ClearMetrics() error {
	s := m.current
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	s.clearMetrics()
	return nil
}

func (s *state) clearMetrics() {
	s.metrics = map[string]*metric{}
	s.order = nil
	s.timestamp = nil
}

// HasStoredMetrics reports whether the current scope has buffered metrics.
// Closing a scope flushes and clears it, so closed scopes report false.
func (m *Metrics) HasStoredMetrics() bool {
	m.current.mu.Lock()
	defer m.current.mu.Unlock()
	return len(m.current.metrics) != 0
}

// SetThrowOnEmptyMetrics changes the current scope's empty-buffer policy.
// New request scopes inherit a snapshot. Clearing or flushing does not reset it.
func (m *Metrics) SetThrowOnEmptyMetrics(enabled bool) error {
	s := m.current
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	s.requireMetrics = &enabled
	return nil
}

// ThrowOnEmptyMetrics enables the empty-buffer error policy.
// Deprecated: use SetThrowOnEmptyMetrics instead.
func (m *Metrics) ThrowOnEmptyMetrics() error { return m.SetThrowOnEmptyMetrics(true) }

func (m *Metrics) requireMetricsLocked() bool {
	if m.current.requireMetrics != nil {
		return *m.current.requireMetrics
	}
	return m.cfg.requireMetrics
}
