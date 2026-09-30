package idempotency

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

type property struct {
	key   string
	value any
}
type object []property

// CanonicalJSON implements the pinned deepSort + JSON.stringify contract for
// JSON-shaped inputs. RawMessage preserves object insertion order for keys
// whose lowercase forms compare equally; Go maps have no insertion order.
// Numbers use JavaScript's float64 domain. See docs/IDEMPOTENCY.md for limits.
func CanonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoded, err := readValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON value")
	}
	var output bytes.Buffer
	writeValue(&output, decoded)
	return output.Bytes(), nil
}

func readValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); ok {
		switch delim {
		case '{':
			result := object{}
			positions := map[string]int{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				value, err := readValue(decoder)
				if err != nil {
					return nil, err
				}
				name := key.(string)
				if index, exists := positions[name]; exists {
					result[index].value = value
				} else {
					positions[name] = len(result)
					result = append(result, property{name, value})
				}
			}
			_, err := decoder.Token()
			return result, err
		case '[':
			result := []any{}
			for decoder.More() {
				value, err := readValue(decoder)
				if err != nil {
					return nil, err
				}
				result = append(result, value)
			}
			_, err := decoder.Token()
			return result, err
		}
	}
	return token, nil
}

func writeValue(out *bytes.Buffer, value any) {
	switch v := value.(type) {
	case object:
		// JavaScript enumerates array-index keys before other object properties.
		sort.SliceStable(v, func(i, j int) bool {
			a, ai := arrayIndex(v[i].key)
			b, bi := arrayIndex(v[j].key)
			if ai != bi {
				return ai
			}
			if ai {
				return a < b
			}
			return utf16Less(strings.ToLower(v[i].key), strings.ToLower(v[j].key))
		})
		out.WriteByte('{')
		count := 0
		for _, entry := range v {
			// The reference assigns properties into {}, invoking its prototype setter.
			if entry.key == "__proto__" {
				continue
			}
			if count > 0 {
				out.WriteByte(',')
			}
			count++
			writeString(out, entry.key)
			out.WriteByte(':')
			writeValue(out, entry.value)
		}
		out.WriteByte('}')
	case []any:
		out.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				out.WriteByte(',')
			}
			writeValue(out, item)
		}
		out.WriteByte(']')
	case string:
		writeString(out, v)
	case float64:
		if v == 0 {
			out.WriteByte('0')
			return
		}
		encoded, _ := json.Marshal(v)
		out.Write(encoded)
	default:
		encoded, _ := json.Marshal(v)
		out.Write(encoded)
	}
}

func writeString(out *bytes.Buffer, value string) {
	// encoding/json escapes HTML and two Unicode separators even though
	// JSON.stringify does not. Emit strings directly to avoid changing hashes.
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(out, `\u%04x`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
}

func arrayIndex(key string) (uint64, bool) {
	n, err := strconv.ParseUint(key, 10, 32)
	return n, err == nil && n < 4294967295 && strconv.FormatUint(n, 10) == key
}

func utf16Less(a, b string) bool {
	x, y := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return len(x) < len(y)
}
