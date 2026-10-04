package logger

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"math"
	"reflect"
	"sort"
	"strings"
)

var standardOrder = []string{"level", "message", "timestamp", "service", "cold_start", "function_arn", "function_memory_size", "function_name", "function_request_id", "sampling_rate", "xray_trace_id", "tenant_id"}

func reserved(key string) bool {
	switch key {
	case "level", "message", "timestamp", "service", "sampling_rate":
		return true
	}
	return false
}

func cloneFields(fields Fields) Fields {
	out := make(Fields, len(fields))
	for key, value := range fields {
		out[key] = value
	}
	return out
}

// mergeFields filters reserved root fields and delegates nested/indexed merges to Commons.
func mergeFields(dst Fields, src Fields) {
	filtered := make(map[string]any, len(src))
	for k, v := range src {
		if reserved(k) {
			continue
		}
		filtered[k] = v
	}
	// Existing nested objects may belong to a parent logger; copy before merging.
	merged := commons.DeepMerge(map[string]any{}, map[string]any(dst), filtered)
	clear(dst)
	for key, value := range merged {
		dst[key] = value
	}
}

func asFields(value any) (Fields, bool) {
	switch value := value.(type) {
	case Fields:
		return value, true
	case map[string]any:
		return Fields(value), true
	default:
		return nil, false
	}
}

type visit struct {
	typ reflect.Type
	ptr uintptr
}

func normalize(key string, value any, replacer Replacer, active map[visit]bool, depth int) any {
	if replacer != nil {
		value = replacer(key, value)
	}
	return normalizeValue(key, value, replacer, active, depth)
}

// normalizeValue follows pointers without applying the same property's replacer twice.
// Descendant properties still pass through normalize and receive the replacer.
func normalizeValue(key string, value any, replacer Replacer, active map[visit]bool, depth int) any {
	if value == nil {
		return nil
	}
	if depth > 100 {
		return "[Truncated]"
	}
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Map || v.Kind() == reflect.Slice {
		if v.IsNil() {
			return nil
		}
		var pointer uintptr
		if v.Kind() == reflect.Map {
			pointer = uintptr(v.UnsafePointer())
		} else {
			pointer = v.Pointer()
		}
		ref := visit{v.Type(), pointer}
		if active[ref] {
			return "[Circular]"
		}
		active[ref] = true
		defer delete(active, ref)
	}
	if err, ok := value.(error); ok {
		result := Fields{"name": reflect.TypeOf(err).String(), "message": err.Error(), "location": ""}
		if wrapped, ok := err.(interface{ Unwrap() error }); ok && wrapped.Unwrap() != nil {
			result["cause"] = normalize("cause", wrapped.Unwrap(), replacer, active, depth+1)
		}
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			result["cause"] = normalize("cause", joined.Unwrap(), replacer, active, depth+1)
		}
		return result
	}
	if _, ok := value.(json.Marshaler); ok {
		return normalizeJSON(key, value, replacer, active, depth)
	}
	switch v.Kind() {
	case reflect.Struct:
		return normalizeJSON(key, value, replacer, active, depth)
	case reflect.Float32, reflect.Float64:
		if math.IsNaN(v.Float()) || math.IsInf(v.Float(), 0) {
			return nil
		}
	case reflect.Pointer, reflect.Interface:
		return normalizeValue(key, v.Elem().Interface(), replacer, active, depth+1)
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return value
		}
		result := Fields{}
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key().String()
			result[k] = normalize(k, iter.Value().Interface(), replacer, active, depth+1)
		}
		return result
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return value
		}
		result := make([]any, v.Len())
		for i := range result {
			result[i] = normalize(fmt.Sprint(i), v.Index(i).Interface(), replacer, active, depth+1)
		}
		return result
	}
	return value
}

// Apply descendant replacers to the JSON representation of structs/marshalers,
// preserving field tags and exact numeric tokens without repeating the root callback.
func normalizeJSON(key string, value any, replacer Replacer, active map[visit]bool, depth int) any {
	if replacer == nil {
		return value
	}
	data, err := marshal(value)
	if err != nil {
		return value
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var decoded any
	if err = decoder.Decode(&decoded); err != nil {
		return value
	}
	return normalizeValue(key, decoded, replacer, active, depth+1)
}

func marshal(value any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte{'\n'}), nil
}

// prepareForPrint makes a shallow copy of a map-shaped document, omitting empty
// strings and JSON null values before replacer traversal. Nested values and
// nonnil empty collections remain intact; the formatter's document is not mutated.
func prepareForPrint(value any) any {
	if _, ok := value.(json.Marshaler); ok {
		return value
	}
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Map || v.Type().Key().Kind() != reflect.String {
		return value
	}
	result := reflect.MakeMapWithSize(v.Type(), v.Len())
	iter := v.MapRange()
	for iter.Next() {
		item := iter.Value().Interface()
		if item == nil {
			continue
		}
		// A custom marshaler's JSON value or error cannot be inferred from its
		// underlying Go value. Preserve it for the normal encoding traversal.
		_, jsonCustom := item.(json.Marshaler)
		_, textCustom := item.(encoding.TextMarshaler)
		if !jsonCustom && !textCustom {
			field := reflect.ValueOf(item)
			switch field.Kind() {
			case reflect.String:
				if field.Len() == 0 {
					continue
				}
			case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface:
				if field.IsNil() {
					continue
				}
			}
		}
		result.SetMapIndex(iter.Key(), iter.Value())
	}
	return result.Interface()
}

func encode(fields Fields, c config) ([]byte, error) {
	var value any = fields
	var err error
	if c.formatter != nil {
		value, err = c.formatter(fields)
		if err != nil {
			return nil, err
		}
	}
	value = normalize("", prepareForPrint(value), c.replacer, map[visit]bool{}, 0)
	var data []byte
	if object, ok := asFields(value); ok {
		order := append(append([]string(nil), c.order...), standardOrder...)
		remaining := make([]string, 0, len(object))
		for k := range object {
			remaining = append(remaining, k)
		}
		sort.Strings(remaining)
		order = append(order, remaining...)
		seen := map[string]bool{}
		var out bytes.Buffer
		out.WriteByte('{')
		for _, key := range order {
			v, exists := object[key]
			if !exists || seen[key] {
				continue
			}
			if len(seen) > 0 {
				out.WriteByte(',')
			}
			seen[key] = true
			k, _ := marshal(key)
			b, e := marshal(v)
			if e != nil {
				return nil, e
			}
			out.Write(k)
			out.WriteByte(':')
			out.Write(b)
		}
		out.WriteByte('}')
		data = out.Bytes()
	} else {
		data, err = marshal(value)
		if err != nil {
			return nil, err
		}
	}
	if c.pretty {
		var out bytes.Buffer
		if err := json.Indent(&out, data, "", strings.Repeat(" ", 4)); err != nil {
			return nil, err
		}
		data = out.Bytes()
	}
	return data, nil
}
