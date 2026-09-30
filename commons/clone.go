package commons

import "math/big"

// CloneValue isolates acyclic decoded configuration values, retaining opaque objects.
// It preserves keys and bytes; DeepMerge has different key-filtering/array semantics.
func CloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = CloneValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = CloneValue(item)
		}
		return out
	case []byte:
		return append([]byte{}, v...)
	case []string:
		return append([]string{}, v...)
	case []float64:
		return append([]float64{}, v...)
	case [][]byte:
		out := make([][]byte, len(v))
		for i, item := range v {
			out[i] = append([]byte{}, item...)
		}
		return out
	case *big.Int:
		if v == nil {
			return (*big.Int)(nil)
		}
		return new(big.Int).Set(v)
	default:
		return value
	}
}
