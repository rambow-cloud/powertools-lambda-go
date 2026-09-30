package bedrock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type visit struct {
	kind    reflect.Kind
	pointer uintptr
	length  int
}
type jsonWriter struct {
	bytes.Buffer
	active map[visit]bool
}

func stringify(value any) ([]byte, error) {
	w := jsonWriter{active: map[visit]bool{}}
	if err := w.value(value); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (w *jsonWriter) value(value any) error {
	if value == nil {
		w.WriteString("null")
		return nil
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Map || rv.Kind() == reflect.Slice || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			w.WriteString("null")
			return nil
		}
		key := visit{kind: rv.Kind(), pointer: uintptr(rv.UnsafePointer())}
		if rv.Kind() == reflect.Slice {
			key.length = rv.Len()
		}
		if w.active[key] {
			return &NamedError{Name: "TypeError", Message: "Converting circular structure to JSON"}
		}
		w.active[key] = true
		defer delete(w.active, key)
	}
	switch v := value.(type) {
	case Undefined:
		w.WriteString("null")
	case string:
		w.string(v)
	case bool:
		w.WriteString(strconv.FormatBool(v))
	case float64:
		w.WriteString(numberString(v, true))
	case float32:
		w.WriteString(numberString(float64(v), true))
	case *Parameters:
		return w.object(v.Keys(), v.values)
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		commons.SortObjectKeys(keys)
		return w.object(keys, v)
	case []any:
		w.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				w.WriteByte(',')
			}
			if err := w.value(item); err != nil {
				return err
			}
		}
		w.WriteByte(']')
	default:
		// Native structs, custom marshalers and raw JSON retain their Go contracts.
		var encoded bytes.Buffer
		encoder := json.NewEncoder(&encoded)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil {
			return err
		}
		w.Write(bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'}))
	}
	return nil
}

func (w *jsonWriter) object(keys []string, values map[string]any) error {
	w.WriteByte('{')
	first := true
	for _, key := range keys {
		value := values[key]
		if _, absent := value.(Undefined); absent {
			continue
		}
		if !first {
			w.WriteByte(',')
		}
		first = false
		w.string(key)
		w.WriteByte(':')
		if err := w.value(value); err != nil {
			return err
		}
	}
	w.WriteByte('}')
	return nil
}

func (w *jsonWriter) string(value string) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	data := bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'})
	for i := 0; i < len(data); i++ {
		if data[i] == '\\' && i+1 < len(data) {
			if i+6 <= len(data) && (string(data[i:i+6]) == `\u2028` || string(data[i:i+6]) == `\u2029`) {
				if data[i+5] == '8' {
					w.WriteRune('\u2028')
				} else {
					w.WriteRune('\u2029')
				}
				i += 5
			} else {
				w.Write(data[i : i+2])
				i++
			}
		} else {
			w.WriteByte(data[i])
		}
	}
}

func numberString(number float64, jsonMode bool) string {
	if math.IsNaN(number) {
		if jsonMode {
			return "null"
		}
		return "NaN"
	}
	if math.IsInf(number, 0) {
		if jsonMode {
			return "null"
		}
		if number < 0 {
			return "-Infinity"
		}
		return "Infinity"
	}
	if number == 0 {
		return "0"
	}
	format := byte('f')
	if absolute := math.Abs(number); absolute < 1e-6 || absolute >= 1e21 {
		format = 'e'
	}
	result := strconv.FormatFloat(number, format, -1, 64)
	result = strings.Replace(result, "e-0", "e-", 1)
	return strings.Replace(result, "e+0", "e+", 1)
}

func thrownString(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case Undefined:
		return "undefined"
	case string:
		return v
	case float64:
		return numberString(v, false)
	case map[string]any:
		return "[object Object]"
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			if item != nil {
				if _, absent := item.(Undefined); !absent {
					parts[i] = thrownString(item)
				}
			}
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(value)
	}
}
