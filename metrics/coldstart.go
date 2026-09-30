package metrics

import "github.com/rambow-cloud/powertools-lambda-go/commons"

// CaptureColdStartMetric emits one isolated ColdStart metric per Metrics instance
// for on-demand initialization. Bound scopes also require a cold invocation.
// The optional function name is used only when no configured name exists.
// The cold-start decision is consumed before writing, including failed writes.
func (m *Metrics) CaptureColdStartMetric(functionName ...string) error {
	s := m.current
	s.mu.Lock()
	if s.closed.Load() {
		s.mu.Unlock()
		return ErrInvocationClosed
	}
	if s.invocationCold != nil && !*s.invocationCold || !m.cold.GetColdStart() {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	single, err := m.SingleMetric()
	if err != nil {
		return err
	}
	s.mu.Lock()
	function := ""
	if s.functionName != nil {
		function = commons.TrimSpace(*s.functionName)
	} else if len(functionName) > 0 {
		function = commons.TrimSpace(functionName[0])
	}
	s.mu.Unlock()
	if function != "" {
		if err := single.AddDimension("function_name", function); err != nil {
			return err
		}
	}
	return single.AddMetric("ColdStart", Count, 1)
}

// SetFunctionName changes the active scope's cold-start function name. Empty values
// suppress the capture argument; clearing or flushing does not reset the name.
// Deprecated: use WithFunctionName or the CaptureColdStartMetric argument instead.
func (m *Metrics) SetFunctionName(name string) error {
	s := m.current
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	s.functionName = &name
	return nil
}
