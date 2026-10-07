package metrics

import (
	json "encoding/json/v2"
	"fmt"
	"math"
	"sort"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type definition struct {
	Name              string
	Unit              Unit
	StorageResolution Resolution `json:",omitzero"`
}

func dimensionKeys(base, dimensions Dimensions) []string {
	keys := make(map[string]bool, len(base)+len(dimensions))
	for key := range base {
		keys[key] = true
	}
	for key := range dimensions {
		keys[key] = true
	}
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

// Serialize snapshots an EMF document without flushing or clearing buffered metrics.
func (m *Metrics) Serialize() ([]byte, error) {
	m.current.mu.Lock()
	defer m.unlock(m.current)
	if m.current.closed.Load() {
		return nil, ErrInvocationClosed
	}
	return m.serializeLocked()
}

func (m *Metrics) serializeLocked() ([]byte, error) {
	s := m.current
	if len(s.metrics) == 0 && m.requireMetricsLocked() {
		return nil, ErrEmptyMetrics
	}
	if m.cfg.namespace == "" {
		s.warn(namespaceWarning)
	}
	document := make(map[string]any)
	sources := map[string]string{}
	setValue := func(key string, value any, source string) {
		if existing, present := sources[key]; present {
			s.warn(fmt.Sprintf("EMF key \"%s\" is defined as both a %s and %s; the %s value will take precedence in the serialized output", key, existing, source, source))
		}
		sources[key], document[key] = source, value
	}
	for _, key := range sortedKeys(s.metadata) {
		setValue(key, s.metadata[key], "metadata")
	}
	for _, key := range sortedKeys(s.defaults) {
		setValue(key, s.defaults[key], "default dimension")
	}
	for _, key := range sortedKeys(s.dimensions) {
		setValue(key, s.dimensions[key], "dimension")
	}
	dimensions := make([][]string, 0, len(s.sets)+1)
	if len(s.dimensions) > 0 {
		dimensions = append(dimensions, dimensionKeys(s.defaults, s.dimensions))
	}
	for _, set := range s.sets {
		dimensions = append(dimensions, dimensionKeys(s.defaults, set))
		for _, key := range sortedKeys(set) {
			setValue(key, set[key], "dimension set")
		}
	}
	if len(dimensions) == 0 && len(s.defaults) > 0 {
		dimensions = append(dimensions, dimensionKeys(s.defaults, nil))
	}
	definitions := make([]definition, 0, len(s.order))
	order := append([]string(nil), s.order...)
	commons.SortObjectKeys(order)
	for _, name := range order {
		if source, exists := sources[name]; exists {
			return nil, fmt.Errorf("EMF key collision on \"%s\": registered as both a metric (number) and a %s (string)", name, source)
		}
		metric := s.metrics[name]
		def := definition{Name: name, Unit: metric.unit}
		if metric.resolution == High {
			def.StorageResolution = High
		}
		definitions = append(definitions, def)
		if len(metric.values) == 1 {
			document[name] = emfJSONNumber(metric.values[0])
		} else {
			values := make([]any, len(metric.values))
			for i, value := range metric.values {
				values[i] = emfJSONNumber(value)
			}
			document[name] = values
		}
	}
	namespace := m.cfg.namespace
	if namespace == "" {
		namespace = "default_namespace"
	}
	var timestamp float64
	if s.timestamp != nil {
		timestamp = *s.timestamp
	} else {
		timestamp = dateMilliseconds(m.cfg.clock())
	}
	if _, overwritten := document["_aws"]; !overwritten {
		document["_aws"] = map[string]any{"Timestamp": emfJSONNumber(timestamp), "CloudWatchMetrics": []any{map[string]any{"Namespace": namespace, "Dimensions": dimensions, "Metrics": definitions}}}
	}
	return json.Marshal(document)
}

// JSON.stringify emits null for non-finite numbers and normalizes negative zero.
func emfJSONNumber(value float64) any {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	if value == 0 {
		return 0
	}
	return value
}
