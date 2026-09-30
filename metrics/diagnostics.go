package metrics

import (
	"fmt"
	"slices"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

const (
	emptyWarning     = "No application metrics to publish. The cold-start metric may be published if enabled. If application metrics should never be empty, consider using `throwOnEmptyMetrics`"
	namespaceWarning = "Namespace should be defined, default used"
	timestampWarning = "This metric doesn't meet the requirements and will be skipped by Amazon CloudWatch. Ensure the timestamp is within 14 days in the past or up to 2 hours in the future and is also a valid number or Date object."
)

func (s *state) warn(message string) { s.warnings = append(s.warnings, message) }

// unlock delivers diagnostics only after releasing storage. A callback may query
// Metrics or write through another utility without recursively locking this state.
func (m *Metrics) unlock(s *state) {
	warnings := s.warnings
	s.warnings = nil
	s.mu.Unlock()
	for _, message := range warnings {
		m.cfg.onWarning(message)
	}
}

// Go maps have no insertion order; non-index batch keys use lexical order.
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	commons.SortObjectKeys(keys)
	return keys
}

func (s *state) sanitizeDimensions(values Dimensions, duplicates bool) Dimensions {
	result := Dimensions{}
	for _, key := range sortedKeys(values) {
		value := values[key]
		if commons.IsStringUndefinedNullEmpty(key) || commons.IsStringUndefinedNullEmpty(value) {
			s.warn(fmt.Sprintf("The dimension %s doesn't meet the requirements and won't be added. Ensure the dimension name and value are non empty strings", key))
			continue
		}
		if duplicates {
			s.warnDuplicate(key)
			if key == "__proto__" {
				continue
			}
		}
		result[key] = value
	}
	return result
}

func metricUnitConflict(name string, current, requested Unit) error {
	return fmt.Errorf("Metric \"%s\" has already been added with unit \"%s\", but we received unit \"%s\". Did you mean to use metric unit \"%s\"?", name, current, requested, current)
}

// The pinned store uses an ordinary object, so these inherited names resolve to
// existing values without a metric unit. Preserve its observable conflict error.
func inheritedMetricName(name string) bool {
	switch name {
	case "constructor", "__defineGetter__", "__defineSetter__", "hasOwnProperty", "__lookupGetter__", "__lookupSetter__", "isPrototypeOf", "propertyIsEnumerable", "toString", "valueOf", "__proto__", "toLocaleString":
		return true
	}
	return false
}

func (s *state) warnDuplicate(key string) {
	_, regular := s.dimensions[key]
	_, defaults := s.defaults[key]
	if regular || defaults {
		s.warn(fmt.Sprintf("Dimension \"%s\" has already been added. The previous value will be overwritten.", key))
	}
}
