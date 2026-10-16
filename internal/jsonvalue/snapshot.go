package jsonvalue

import (
	json "encoding/json/v2"
	"math"
	"unicode/utf8"
)

// Snapshot returns the private, float64-number representation of a default
// JSON v2 marshal/unmarshal round trip. Ordinary JSON-shaped values can be
// copied without encoding; all other values retain the standard library path.
func Snapshot(value any) (any, error) {
	if result, ok := snapshot(value, 0); ok {
		return result, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func snapshot(input any, depth int) (any, bool) {
	// Bound speculative copying without maintaining a second graph/cycle detector.
	// Deep or cyclic inputs use JSON v2's own limits and diagnostics instead.
	if depth > 64 {
		return nil, false
	}
	switch value := input.(type) {
	case nil, bool:
		return value, true
	case string:
		return input, utf8.ValidString(value)
	case float64:
		return input, !math.IsNaN(value) && !math.IsInf(value, 0)
	case int:
		return float64(value), true
	case int8:
		return float64(value), true
	case int16:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint:
		return float64(value), true
	case uint8:
		return float64(value), true
	case uint16:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint64:
		return float64(value), true
	case uintptr:
		return float64(value), true
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, item := range value {
			if !utf8.ValidString(key) {
				return nil, false
			}
			cloned, ok := snapshot(item, depth+1)
			if !ok {
				return nil, false
			}
			result[key] = cloned
		}
		return result, true
	case []any:
		result := make([]any, len(value))
		for i, item := range value {
			cloned, ok := snapshot(item, depth+1)
			if !ok {
				return nil, false
			}
			result[i] = cloned
		}
		return result, true
	default:
		// Named types may implement marshalers. float32 also needs its JSON decimal
		// representation before float64 conversion, rather than a direct Go cast.
		return nil, false
	}
}
